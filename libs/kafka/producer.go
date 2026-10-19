// Package kafka wraps franz-go with this system's event-log conventions.
//
// Kafka carries the durable finding stream; RabbitMQ carries per-task dispatch
// (plan.md §1). The split matters here: this package has no retry or DLQ
// machinery, because a Kafka consumer that cannot process a record must stop and
// be fixed rather than skip ahead — the analytics rollups depend on seeing every
// record exactly once in order.
//
// Files:
//
//	producer.go  synchronous, idempotent publishing
//	consumer.go  consumer-group reads with commit-after-success
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
