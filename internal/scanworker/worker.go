// Package scanworker is the scan-worker service: it consumes scan tasks, runs
// the scan engine over each repository, persists findings idempotently and
// publishes them to the finding stream (plan.md §4, §10).
//
// It lives apart from internal/scan so the engine stays a pure library with no
// queue, database or Kafka dependency, and this package holds all the wiring.
package scanworker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	"github.com/mservice/gitscanner/libs/kafka"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// Publisher is the subset of the Kafka producer the worker needs. Narrowed to
// an interface so the worker can be tested without a broker.
type Publisher interface {
	Publish(ctx context.Context, topic, key string, value []byte, headers map[string]string) error
}

// Enqueuer is the dispatch surface the worker needs, narrowed for the same
// reason as Publisher.
type Enqueuer interface {
	EnqueueEnrich(ctx context.Context, req dispatch.EnrichRequest) error
}

// Worker handles one scan task at a time within a RabbitMQ consumer.
type Worker struct {
	engine *scan.Engine
	db     *store.DB
	pub    Publisher
	disp   Enqueuer
	log    *slog.Logger
}

// New builds a worker.
func New(engine *scan.Engine, db *store.DB, pub Publisher, disp Enqueuer, log *slog.Logger) *Worker {
	return &Worker{engine: engine, db: db, pub: pub, disp: disp, log: log}
}

// Handle processes one ScanTask delivery. It is the rabbit.Handler for
// scan.tasks: returning an error retries the task, which is safe because
// findings are idempotent on their fingerprint (plan.md §6).
func (w *Worker) Handle(ctx context.Context, d rabbit.Delivery) error {
	var task gpb.ScanTask
	if err := proto.Unmarshal(d.Body, &task); err != nil {
		// A malformed task will never parse on retry, so drop it to the
		// dead-letter queue rather than spin.
		return fmt.Errorf("%w: unmarshal scan task: %v", rabbit.ErrDrop, err)
	}
	return w.scan(ctx, &task)
}

func (w *Worker) scan(ctx context.Context, task *gpb.ScanTask) error {
	jobID := entity.ID(task.JobId)

	// Identify the repository in human terms. A log line carrying only ids is
	// unreadable while a scan is in flight, which is exactly when it is read.
	meta := w.repoLogFields(ctx, task)
	w.log.Info("scan started", append([]any{"job", task.JobId}, meta...)...)

	// A single-repo job starts running on its first (only) task. For a bulk job
	// the discovery worker has already moved it to running, and SetScanState is
	// a no-op when the state is unchanged.
	_ = w.db.SetScanState(ctx, jobID, entity.ScanRunning, "")

	sink := &storeSink{w: w, task: task}
	res, err := w.engine.Scan(ctx, task.JobId, task.RepositoryId, task.CloneUrl, sink)
	if err != nil {
		w.log.Error("scan failed", append([]any{"job", task.JobId, "error", err}, meta...)...)
		return fmt.Errorf("scan repository %s: %w", task.RepositoryId, err)
	}

	w.log.Info("scan done", append([]any{
		"job", task.JobId, "blobs", res.BlobsScanned,
		"findings", sink.inserted, "reclassified", sink.reclassified,
		"app_type", res.AppType,
	}, meta...)...)

	if err := w.db.RecordScanResult(ctx, jobID, res.BlobsScanned, sink.inserted); err != nil {
		return err
	}

	// A single-repo job, or the last repo of a bulk job, completes here. The
	// check is transactional and no-ops when more repos are still pending
	// (plan.md §8), so concurrent workers finishing different repos race safely.
	if _, err := w.db.CompleteIfAllScanned(ctx, jobID); err != nil {
		return err
	}
	return nil
}

// storeSink persists each finding, mirrors it to Kafka and queues it for
// enrichment. It counts only the rows it actually inserted, so a redelivered
// task does not inflate the total — and, because the Kafka publish hangs off
// that same check, does not emit a duplicate event for a finding already seen.
type storeSink struct {
	w            *Worker
	task         *gpb.ScanTask
	inserted     int
	reclassified int
}

func (s *storeSink) Emit(ctx context.Context, f entity.Finding) error {
	write, err := s.w.db.InsertFinding(ctx, f)
	if err != nil {
		return err
	}

	switch write {
	case store.FindingUnchanged:
		return nil

	case store.FindingReclassified:
		// The row existed but has just been attributed to a named rule, so its
		// severity needs recomputing against the provider it now names. It is
		// not a new finding and must not be counted or re-published as one:
		// analytics has already counted this finding id and would reject the
		// replay anyway.
		s.reclassified++
		return s.w.enqueueEnrich(ctx, s.task, f)
	}

	s.inserted++
	if err := s.w.publish(ctx, s.task, f); err != nil {
		return err
	}
	return s.w.enqueueEnrich(ctx, s.task, f)
}

// repoLogFields describes the repository being scanned, for the log lines that
// bracket the scan.
//
// A lookup failure degrades to the ids and the URL the task already carries
// rather than failing: this is context for a human reading the log, and losing
// it is never a reason to abandon a scan that would otherwise succeed.
func (w *Worker) repoLogFields(ctx context.Context, task *gpb.ScanTask) []any {
	fields := []any{"repo_id", task.RepositoryId, "url", task.CloneUrl}

	r, err := w.db.GetRepository(ctx, entity.ID(task.RepositoryId))
	if err != nil {
		w.log.Debug("repository metadata unavailable", "repo_id", task.RepositoryId, "error", err)
		return fields
	}

	fields = append(fields,
		"repo", r.FullName(),
		"branch", r.DefaultBranch,
		"stars", r.Stars,
		"fork", r.IsFork,
		"private", r.IsPrivate,
		"discovered_at", r.CreatedAt.UTC().Format(time.RFC3339),
	)
	if r.RemoteCreatedAt != nil {
		fields = append(fields, "repo_created_at", r.RemoteCreatedAt.UTC().Format(time.RFC3339))
	}
	return fields
}

// enqueueEnrich hands the finding to the enricher for registry resolution
// (plan.md §5). The detection context enrichment clusters on does not have
// columns of its own on the findings row; the engine records it in the
// finding's metadata, which is where these fields are read back from.
func (w *Worker) enqueueEnrich(ctx context.Context, task *gpb.ScanTask, f entity.Finding) error {
	// A missing or malformed length is simply unknown; it weakens the clustering
	// signature but is not a reason to fail the scan that produced the finding.
	length, _ := strconv.Atoi(f.Metadata["secret_length"])

	return w.disp.EnqueueEnrich(ctx, dispatch.EnrichRequest{
		FindingID:    f.ID,
		JobID:        entity.ID(task.JobId),
		RuleID:       f.RuleID,
		FilePath:     f.FilePath,
		NearbyEnvVar: f.Metadata["env_var"],
		NearbyHost:   f.Metadata["host"],
		CharsetClass: f.Metadata["charset_class"],
		SecretLength: length,
	})
}

// publish mirrors a finding onto the Kafka finding stream. The repository id is
// the partition key so every finding for a repo stays ordered, which the
// analytics rollups depend on (plan.md §10).
func (w *Worker) publish(ctx context.Context, task *gpb.ScanTask, f entity.Finding) error {
	evt := &gpb.FindingDetected{
		FindingId:         f.ID.String(),
		JobId:             task.JobId,
		RepositoryId:      task.RepositoryId,
		RuleId:            f.RuleID,
		ProviderSlug:      f.ProviderSlug,
		Fingerprint:       f.Fingerprint,
		MaskedPreview:     f.MaskedPreview,
		BlobOid:           f.BlobOID,
		FilePath:          f.FilePath,
		LineNumber:        int32(f.LineNumber),
		ReachableFromHead: f.ReachableFromHEAD,
		Severity:          gpb.Severity(f.Severity + 1), // entity 0..4 -> proto 1..5
		DetectedAt:        timestamppb.Now(),
	}
	body, err := proto.Marshal(evt)
	if err != nil {
		return fmt.Errorf("scanworker: marshal finding event: %w", err)
	}
	return w.pub.Publish(ctx, kafka.TopicFindings, task.RepositoryId, body, nil)
}
