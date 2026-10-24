// Package analytics is the analytics service: it consumes the finding stream
// and folds it into the rollup tables the dashboards read (plan.md §10).
//
// The stream arrives over RabbitMQ, which is not a retained log, so a rollup
// cannot be rebuilt by replaying it — once a delivery is acked it is gone. The
// findings table is the system of record and a rebuild reads from there
// instead. What the queue must still survive is redelivery, and it does: the
// dedupe tables behind RecordFindingRollups count each finding id once however
// often it arrives.
package analytics

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/entity"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// unmappedProvider is the bucket a finding falls into when no registry entry
// claims it. Counting it is the point rather than a fallback: the size of this
// bucket over time is the unmapped backlog the registry works through
// (plan.md §5.4).
const unmappedProvider = "unmapped"

// Rollups is the store surface this worker needs.
type Rollups interface {
	RecordFindingRollups(ctx context.Context, batch []store.RollupInput) (int, error)
}

// Worker folds batches of finding events into the rollup tables.
type Worker struct {
	store Rollups
	log   *slog.Logger
}

// New builds an analytics worker.
func New(st Rollups, log *slog.Logger) *Worker {
	return &Worker{store: st, log: log}
}

// Handle folds one finding event into the rollups (the rabbit.Handler for
// findings.events).
//
// An event that cannot be read, or that names no finding, is dropped to the
// dead-letter queue rather than retried: neither will parse on a second
// attempt, and the message is kept there for inspection instead of being lost.
func (w *Worker) Handle(ctx context.Context, d rabbit.Delivery) error {
	var evt gpb.FindingDetected
	if err := proto.Unmarshal(d.Body, &evt); err != nil {
		return fmt.Errorf("%w: unmarshal finding event: %v", rabbit.ErrDrop, err)
	}
	if evt.FindingId == "" {
		return fmt.Errorf("%w: finding event carries no finding id", rabbit.ErrDrop)
	}

	counted, err := w.store.RecordFindingRollups(ctx, []store.RollupInput{rollupInput(&evt)})
	if err != nil {
		return fmt.Errorf("analytics: record rollups: %w", err)
	}

	// counted == 0 is normal rather than a warning: it is a redelivery of an
	// event an earlier attempt already counted.
	w.log.Info("finding rolled up",
		"finding", evt.FindingId, "provider", evt.ProviderSlug,
		"severity", severityName(evt.Severity), "counted", counted)
	return nil
}

// rollupInput maps one event to its rollup contribution. The proto stays on
// this side of the boundary: store takes domain values only (see the
// libs/entity package comment).
func rollupInput(evt *gpb.FindingDetected) store.RollupInput {
	provider := evt.ProviderSlug
	if provider == "" {
		provider = unmappedProvider
	}

	dims := map[string]string{
		store.DimensionProvider: provider,
		store.DimensionSeverity: severityName(evt.Severity),
		store.DimensionRule:     evt.RuleId,
	}
	// app_type is empty until the scan worker records a detected stack. An empty
	// key would aggregate every such finding under a bucket that means nothing,
	// so the dimension is simply absent until there is a value for it.
	if evt.AppType != "" {
		dims[store.DimensionAppType] = evt.AppType
	}

	return store.RollupInput{
		FindingID:    entity.ID(evt.FindingId),
		RepositoryID: entity.ID(evt.RepositoryId),
		BucketDate:   bucketDate(evt.DetectedAt),
		Dimensions:   dims,
	}
}

// severityName maps the wire enum back to the domain name. The producer writes
// gpb.Severity(f.Severity + 1), so the inverse is -1. A value outside the range
// — SEVERITY_UNSPECIFIED, or an enum member added by a newer producer — is
// counted as unknown rather than shifted into a real severity, which would
// quietly misreport the one number this table exists to get right.
func severityName(s gpb.Severity) string {
	if s < gpb.Severity_SEVERITY_INFO || s > gpb.Severity_SEVERITY_CRITICAL {
		return "unknown"
	}
	return entity.Severity(s - 1).String()
}

// bucketDate is the UTC day a finding counts under. A missing or zero timestamp
// falls back to now: year 1 is outside every dashboard's range, so such a
// finding would be counted and then never seen.
func bucketDate(ts *timestamppb.Timestamp) time.Time {
	if ts == nil {
		return time.Now().UTC()
	}
	if t := ts.AsTime(); !t.IsZero() {
		return t.UTC()
	}
	return time.Now().UTC()
}
