// Package kafka wraps franz-go with this system's event-log conventions.
//
// Kafka carries the durable finding stream; RabbitMQ carries per-task dispatch
// (plan.md §1). The split matters here: this package has no retry or DLQ
// machinery, because a Kafka consumer that cannot process a record must stop and
// be fixed rather than skip ahead — the analytics rollups depend on seeing every
// record exactly once in order.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Topics. Declared here so producer and consumer cannot disagree.
const (
	TopicFindings = "findings.v1"
	TopicRegistry = "registry.v1"
	TopicScans    = "scans.v1"
)

// Producer publishes events.
type Producer struct {
	cl  *kgo.Client
	log *slog.Logger
}

// NewProducer connects to the brokers.
func NewProducer(brokers []string, log *slog.Logger) (*Producer, error) {
	if len(brokers) == 0 {
		return nil, errors.New("kafka: no brokers configured")
	}

	cl, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ProducerBatchCompression(kgo.SnappyCompression()),
		// Idempotent production is the default in franz-go and is required for
		// the exactly-once-per-partition guarantee analytics relies on.
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerLinger(10*time.Millisecond),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka: new producer: %w", err)
	}

	return &Producer{cl: cl, log: log}, nil
}

// Publish writes one record synchronously.
//
// key decides the partition. Pass the repository id for finding events so every
// finding for a repo lands on one partition and stays ordered: analytics
// aggregates per repo, and out-of-order updates there produce wrong rollups.
func (p *Producer) Publish(ctx context.Context, topic, key string, value []byte, headers map[string]string) error {
	rec := &kgo.Record{
		Topic: topic,
		Key:   []byte(key),
		Value: value,
	}
	for k, v := range headers {
		rec.Headers = append(rec.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
	}

	if err := p.cl.ProduceSync(ctx, rec).FirstErr(); err != nil {
		return fmt.Errorf("kafka: publish to %s: %w", topic, err)
	}
	return nil
}

// Ping is the readiness check.
func (p *Producer) Ping(ctx context.Context) error { return p.cl.Ping(ctx) }

// Close flushes buffered records and disconnects.
func (p *Producer) Close(ctx context.Context) error {
	if err := p.cl.Flush(ctx); err != nil {
		return fmt.Errorf("kafka: flush: %w", err)
	}
	p.cl.Close()
	return nil
}

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
