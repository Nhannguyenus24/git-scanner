package app

import (
	"context"

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
