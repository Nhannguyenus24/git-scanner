package app

import (
	"context"

	"github.com/mservice/gitscanner/libs/kafka"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// ConsumerWorker adapts a RabbitMQ consumer to the Worker and Drainer
// interfaces, so a service that consumes a queue becomes a one-line Build:
//
//	cons, _ := d.Rabbit.NewConsumer(rabbit.QueueScan, prefetch, maxRetry)
//	return app.Parts{Worker: app.NewConsumerWorker(cons, handler)}, nil
//
// Run consumes until the context is cancelled; Drain cancels the subscription
// and waits for in-flight deliveries, which is exactly the SIGTERM step 1 the
// lifecycle expects (plan.md §13.3).
type ConsumerWorker struct {
	consumer *rabbit.Consumer
	handler  rabbit.Handler
}

// NewConsumerWorker wraps consumer so the app lifecycle drives it.
func NewConsumerWorker(consumer *rabbit.Consumer, handler rabbit.Handler) *ConsumerWorker {
	return &ConsumerWorker{consumer: consumer, handler: handler}
}

// Run consumes until ctx is cancelled or the consumer drains.
func (c *ConsumerWorker) Run(ctx context.Context) error {
	return c.consumer.Run(ctx, c.handler)
}

// Drain stops new deliveries and waits for in-flight work to finish.
func (c *ConsumerWorker) Drain(ctx context.Context) error {
	return c.consumer.Drain(ctx)
}

// KafkaConsumerWorker adapts a Kafka consumer group to the Worker and Drainer
// interfaces, the event-stream counterpart of ConsumerWorker.
//
// Drain commits the offsets the handler has already earned and leaves the group
// cleanly. Skipping it would not lose data — offsets are committed only after a
// batch succeeds — but it would make the next rebalance replay the last batch,
// and a clean leave avoids that cost on every rollout (plan.md §13.3).
type KafkaConsumerWorker struct {
	consumer *kafka.Consumer
	handler  kafka.Handler
}

// NewKafkaConsumerWorker wraps consumer so the app lifecycle drives it.
func NewKafkaConsumerWorker(consumer *kafka.Consumer, handler kafka.Handler) *KafkaConsumerWorker {
	return &KafkaConsumerWorker{consumer: consumer, handler: handler}
}

// Run polls until ctx is cancelled.
func (k *KafkaConsumerWorker) Run(ctx context.Context) error {
	return k.consumer.Run(ctx, k.handler)
}

// Drain commits final offsets and leaves the consumer group.
func (k *KafkaConsumerWorker) Drain(ctx context.Context) error {
	return k.consumer.Close(ctx)
}
