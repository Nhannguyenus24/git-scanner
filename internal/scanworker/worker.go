// Package scanworker is the scan-worker service: it consumes scan tasks, runs
// the scan engine over each repository, persists findings idempotently and
// publishes them to the finding stream (plan.md §4, §10).
//
// It lives apart from internal/scan so the engine stays a pure library with no
// queue or database dependency, and this package holds all the wiring.
package scanworker

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// Enqueuer is the dispatch surface the worker needs, narrowed to an interface
// so the worker can be tested without a broker.
type Enqueuer interface {
	EnqueueEnrich(ctx context.Context, req dispatch.EnrichRequest) error
	PublishFinding(ctx context.Context, f entity.Finding, repoFullName, appType string) error
}

// Worker handles one scan task at a time within a RabbitMQ consumer.
type Worker struct {
	engine *scan.Engine
	db     *store.DB
	disp   Enqueuer
	log    *slog.Logger
}

// New builds a worker.
func New(engine *scan.Engine, db *store.DB, disp Enqueuer, log *slog.Logger) *Worker {
	return &Worker{engine: engine, db: db, disp: disp, log: log}
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
	repoID := entity.ID(task.RepositoryId)

	// Loaded once and used twice: to name the repository in the log lines that
	// bracket the scan, and to stamp the finding events. A log line carrying
	// only ids is unreadable while a scan is in flight, which is exactly when
	// it is read.
	repo, repoErr := w.db.GetRepository(ctx, repoID)
	if repoErr != nil {
		w.log.Debug("repository metadata unavailable", "repo_id", task.RepositoryId, "error", repoErr)
	}
	meta := repoLogFields(task, repo, repoErr)
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

	// The detected stack is worth keeping: it feeds severity scoring (plan.md
	// §4.3) and is a rollup dimension, and only a completed walk can establish
	// it. Stored before the events go out so both carry the same answer.
	if err := w.db.SetRepositoryAppType(ctx, repoID, res.AppType); err != nil {
		return err
	}
	if err := w.publishFindings(ctx, sink, repoName(repo, repoErr), res.AppType); err != nil {
		return err
	}

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

// storeSink persists each finding, queues it for enrichment, and holds the ones
// analytics needs told about until the walk ends.
//
// It counts only the rows it actually inserted, so a redelivered task does not
// inflate the total, and only those are held for publishing — a finding an
// earlier delivery already announced is not announced twice.
type storeSink struct {
	w            *Worker
	task         *gpb.ScanTask
	inserted     int
	reclassified int

	// announce holds the findings stored by this delivery. They cannot be
	// published as they are found: the event carries the repository's app type,
	// and that is only known once every blob has been seen.
	announce []entity.Finding
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
		// not a new finding and must not be counted or announced as one:
		// analytics has already counted this finding id and would reject the
		// replay anyway.
		s.reclassified++
		return s.w.enqueueEnrich(ctx, s.task, f)
	}

	s.inserted++
	s.announce = append(s.announce, f)
	return s.w.enqueueEnrich(ctx, s.task, f)
}

// publishFindings announces to analytics everything this scan stored.
//
// It runs after the walk because the app type the event carries is a verdict on
// the whole repository. The cost is a wider window in which a crash leaves
// findings in the database that analytics never counted; the same hole existed
// between the insert and the publish before, and closing it properly means
// re-announcing on redelivery, which belongs in its own change.
func (w *Worker) publishFindings(ctx context.Context, sink *storeSink, repoFullName, appType string) error {
	for _, f := range sink.announce {
		if err := w.disp.PublishFinding(ctx, f, repoFullName, appType); err != nil {
			return fmt.Errorf("scanworker: announce finding %s: %w", f.ID, err)
		}
	}
	return nil
}

// repoName is the owner/name the finding events carry, or empty when the
// repository could not be read. Empty is better than the "/" a zero-valued
// Repository would format to, which reads like a real name and is not one.
func repoName(r entity.Repository, lookupErr error) string {
	if lookupErr != nil {
		return ""
	}
	return r.FullName()
}

// repoLogFields describes the repository being scanned, for the log lines that
// bracket the scan.
//
// A lookup failure degrades to the ids and the URL the task already carries
// rather than failing: this is context for a human reading the log, and losing
// it is never a reason to abandon a scan that would otherwise succeed.
func repoLogFields(task *gpb.ScanTask, r entity.Repository, lookupErr error) []any {
	fields := []any{"repo_id", task.RepositoryId, "url", task.CloneUrl}
	if lookupErr != nil {
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
