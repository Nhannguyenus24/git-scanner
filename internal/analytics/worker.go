// Package analytics is the analytics service: it consumes the durable finding
// stream and folds it into the rollup tables the dashboards read (plan.md §10).
//
// It reads Kafka rather than RabbitMQ because a rollup must be rebuildable: the
// topic is a retained log, so a corrected aggregation can be replayed over it,
// which a task queue could never offer once its messages are acked (plan.md §1).
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
	"github.com/mservice/gitscanner/libs/kafka"
	gpb "github.com/mservice/gitscanner/libs/proto/gen/gitscanner/v1"
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

// Handle processes one poll's worth of records (the kafka.Handler for
// findings.v1). The whole batch is folded in one transaction, so the offsets
// the consumer commits afterwards can never be ahead of the counters.
func (w *Worker) Handle(ctx context.Context, records []kafka.Record) error {
	batch := make([]store.RollupInput, 0, len(records))
	skipped := 0

	for _, r := range records {
		var evt gpb.FindingDetected
		if err := proto.Unmarshal(r.Value, &evt); err != nil {
			// Skipped, not returned as an error. Returning one stops the consumer
			// without committing, so this same unreadable record would be polled
			// again forever and the topic would never advance past it. The
			// RabbitMQ workers answer this with rabbit.ErrDrop and a dead-letter
			// queue; a Kafka topic has no such side exit, so the record is logged
			// and left behind instead.
			w.log.Error("skipping unparseable record",
				"topic", r.Topic, "key", r.Key, "bytes", len(r.Value), "error", err)
			skipped++
			continue
		}
		if evt.FindingId == "" {
			w.log.Error("skipping record with no finding id", "topic", r.Topic, "key", r.Key)
			skipped++
			continue
		}
		batch = append(batch, rollupInput(&evt))
	}

	counted := 0
	if len(batch) > 0 {
		var err error
		if counted, err = w.store.RecordFindingRollups(ctx, batch); err != nil {
			return fmt.Errorf("analytics: record rollups: %w", err)
		}
	}

	// counted below len(batch) is normal, not a warning: it is the replayed
	// portion of the batch that earlier runs already counted.
	w.log.Info("rollup batch done",
		"records", len(records), "counted", counted, "skipped", skipped)
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
