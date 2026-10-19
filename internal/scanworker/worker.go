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

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

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

// Worker handles one scan task at a time within a RabbitMQ consumer.
type Worker struct {
	engine *scan.Engine
	db     *store.DB
	pub    Publisher
}

// New builds a worker.
func New(engine *scan.Engine, db *store.DB, pub Publisher) *Worker {
	return &Worker{engine: engine, db: db, pub: pub}
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

	// A single-repo job starts running on its first (only) task. For a bulk job
	// the discovery worker has already moved it to running, and SetScanState is
	// a no-op when the state is unchanged.
	_ = w.db.SetScanState(ctx, jobID, entity.ScanRunning, "")

	sink := &storeSink{w: w, task: task}
	res, err := w.engine.Scan(ctx, task.JobId, task.RepositoryId, task.CloneUrl, sink)
	if err != nil {
		return fmt.Errorf("scan repository %s: %w", task.RepositoryId, err)
	}

	if err := w.db.RecordScanResult(ctx, jobID, res.BlobsScanned, sink.inserted); err != nil {
		return err
	}
	return nil
}

// storeSink persists each finding and mirrors it to Kafka. It counts only the
// rows it actually inserted, so a redelivered task does not inflate the total.
type storeSink struct {
	w        *Worker
	task     *gpb.ScanTask
	inserted int
}

func (s *storeSink) Emit(ctx context.Context, f entity.Finding) error {
	inserted, err := s.w.db.InsertFinding(ctx, f)
	if err != nil {
		return err
	}
	if !inserted {
		return nil // already recorded by an earlier delivery
	}
	s.inserted++
	return s.w.publish(ctx, s.task, f)
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
