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

	"github.com/twmb/franz-go/pkg/kerr"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
)

// Topics. Declared here so producer and consumer cannot disagree.
const (
	TopicFindings = "findings.v1"
	TopicRegistry = "registry.v1"
	TopicScans    = "scans.v1"
)

// Topics is every topic this system publishes. Adding one here is the only
// step needed for NewProducer to declare it.
var Topics = []string{TopicFindings, TopicRegistry, TopicScans}

// Producer publishes events.
type Producer struct {
	cl  *kgo.Client
	log *slog.Logger
}

// NewProducer connects to the brokers and declares the topics this system
// publishes, the way rabbit.Connect declares its queue topology.
//
// Declaring them here rather than relying on auto-creation is deliberate.
// auto.create.topics.enable is off in most production clusters, and when it is
// on the first produce can still race topic creation and fail with
// UNKNOWN_TOPIC_OR_PARTITION. Either way the failure surfaces as a publish
// error deep inside a scan, far from the real cause.
func NewProducer(ctx context.Context, brokers []string, log *slog.Logger) (*Producer, error) {
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

	p := &Producer{cl: cl, log: log}
	if err := p.EnsureTopics(ctx, Topics...); err != nil {
		cl.Close()
		return nil, err
	}
	return p, nil
}

// EnsureTopics creates any of topics that do not exist yet. It is idempotent:
// a topic another replica just created comes back as TOPIC_ALREADY_EXISTS,
// which is success here.
//
// Partition count and replication factor are left to the broker (-1 means "use
// the cluster default"), so dev and production each get what their own
// configuration says rather than a number hard-coded in application code.
func (p *Producer) EnsureTopics(ctx context.Context, topics ...string) error {
	if len(topics) == 0 {
		return nil
	}

	req := kmsg.NewPtrCreateTopicsRequest()
	for _, t := range topics {
		rt := kmsg.NewCreateTopicsRequestTopic()
		rt.Topic = t
		rt.NumPartitions = -1
		rt.ReplicationFactor = -1
		req.Topics = append(req.Topics, rt)
	}

	resp, err := req.RequestWith(ctx, p.cl)
	if err != nil {
		return fmt.Errorf("kafka: create topics: %w", err)
	}

	for _, t := range resp.Topics {
		switch err := kerr.ErrorForCode(t.ErrorCode); {
		case err == nil, errors.Is(err, kerr.TopicAlreadyExists):
		default:
			return fmt.Errorf("kafka: create topic %s: %w", t.Topic, err)
		}
	}
	return nil
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
