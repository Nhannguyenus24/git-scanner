package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Record is one consumed event.
type Record struct {
	Topic     string
	Key       string
	Value     []byte
	Headers   map[string]string
	Timestamp time.Time
}

// Handler processes a batch. Returning an error stops the consumer without
// committing, so the batch is redelivered.
type Handler func(ctx context.Context, records []Record) error

// Consumer reads a topic as part of a consumer group.
type Consumer struct {
	cl    *kgo.Client
	log   *slog.Logger
	group string
}

// NewConsumer joins a consumer group.
//
// Autocommit is off. Offsets are committed only after the handler succeeds,
// which turns "at least once" into something the rollup upserts can make
// idempotent. With autocommit a crash mid-batch silently loses records from the
// analytics totals, and nothing ever reports an error.
func NewConsumer(brokers []string, group string, topics []string, log *slog.Logger) (*Consumer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka: no brokers configured")
	}

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
		kgo.OnPartitionsRevoked(func(ctx context.Context, c *kgo.Client, _ map[string][]int32) {
			// Commit what we have before handing partitions to another member,
			// otherwise a rebalance replays work the group already finished.
			if err := c.CommitUncommittedOffsets(ctx); err != nil {
				log.Error("commit on revoke failed", "error", err)
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka: new consumer: %w", err)
	}

	return &Consumer{cl: cl, log: log, group: group}, nil
}

// Run polls until ctx is cancelled.
func (c *Consumer) Run(ctx context.Context, h Handler) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		fetches := c.cl.PollFetches(ctx)
		if fetches.IsClientClosed() {
			return nil
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("kafka: poll: %v", errs[0].Err)
		}

		records := make([]Record, 0, fetches.NumRecords())
		fetches.EachRecord(func(r *kgo.Record) {
			headers := make(map[string]string, len(r.Headers))
			for _, hdr := range r.Headers {
				headers[hdr.Key] = string(hdr.Value)
			}
			records = append(records, Record{
				Topic:     r.Topic,
				Key:       string(r.Key),
				Value:     r.Value,
				Headers:   headers,
				Timestamp: r.Timestamp,
			})
		})

		if len(records) == 0 {
			continue
		}

		if err := h(ctx, records); err != nil {
			return fmt.Errorf("kafka: handler: %w", err)
		}

		if err := c.cl.CommitUncommittedOffsets(ctx); err != nil {
			return fmt.Errorf("kafka: commit: %w", err)
		}
	}
}

// Close leaves the group cleanly, committing final offsets.
func (c *Consumer) Close(ctx context.Context) error {
	if err := c.cl.CommitUncommittedOffsets(ctx); err != nil {
		c.log.Error("final commit failed", "error", err)
	}
	c.cl.Close()
	return nil
}

// Ping is the readiness check.
func (c *Consumer) Ping(ctx context.Context) error { return c.cl.Ping(ctx) }
