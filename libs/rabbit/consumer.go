package rabbit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Delivery is one task handed to a Handler.
type Delivery struct {
	Body    []byte
	Headers map[string]any
	// Attempt counts prior deliveries, derived from the dead-letter header.
	Attempt int
}

// Handler processes one task. Returning an error sends the task to the retry
// queue; returning ErrDrop sends it straight to the dead-letter queue.
type Handler func(ctx context.Context, d Delivery) error

// ErrDrop marks a task as permanently unprocessable, skipping retries.
var ErrDrop = errors.New("drop task")

// Consumer reads one queue with bounded prefetch.
type Consumer struct {
	conn     *Conn
	ch       *amqp.Channel
	queue    string
	log      *slog.Logger
	maxRetry int

	cancelOnce sync.Once
	drained    chan struct{}
}

// NewConsumer opens a channel with the given prefetch.
//
// Prefetch is the backpressure mechanism: it bounds how much work a pod holds,
// which is what lets KEDA scale on queue depth meaningfully (plan.md §13.4). An
// unbounded consumer would drain the queue into one pod's memory and hide the
// backlog from the scaler.
func (c *Conn) NewConsumer(queue string, prefetch, maxRetry int) (*Consumer, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbit: consumer channel: %w", err)
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("rabbit: set qos: %w", err)
	}
	return &Consumer{
		conn:     c,
		ch:       ch,
		queue:    queue,
		log:      c.log,
		maxRetry: maxRetry,
		drained:  make(chan struct{}),
	}, nil
}

// consumerTag identifies this consumer so Drain can cancel it by name.
func (c *Consumer) consumerTag() string { return c.queue + ".consumer" }

// Run consumes until ctx is cancelled or Drain is called.
func (c *Consumer) Run(ctx context.Context, h Handler) error {
	defer close(c.drained)

	msgs, err := c.ch.ConsumeWithContext(ctx, c.queue, c.consumerTag(),
		false /* autoAck off: we ack only after the handler succeeds */, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("rabbit: consume %s: %w", c.queue, err)
	}

	for msg := range msgs {
		c.handle(ctx, h, msg)
	}

	// The channel closes when the broker confirms the cancel, which happens only
	// after every outstanding delivery has been acked or nacked. Reaching here
	// means this consumer has genuinely drained.
	c.log.Info("consumer drained", "queue", c.queue)
	return nil
}

func (c *Consumer) handle(ctx context.Context, h Handler, msg amqp.Delivery) {
	attempt := AttemptCount(msg.Headers)
	c.log.Debug("delivery received", "queue", c.queue, "attempt", attempt, "size", len(msg.Body))

	err := h(ctx, Delivery{
		Body:    msg.Body,
		Headers: msg.Headers,
		Attempt: attempt,
	})

	switch {
	case err == nil:
		if ackErr := msg.Ack(false); ackErr != nil {
			c.log.Error("ack failed", "queue", c.queue, "error", ackErr)
		} else {
			c.log.Debug("delivery acked", "queue", c.queue)
		}

	case errors.Is(err, ErrDrop), attempt >= c.maxRetry:
		// Nack without requeue routes to the dead-letter exchange, where the
		// message is kept for inspection rather than discarded.
		c.log.Error("task dead-lettered", "queue", c.queue,
			"attempt", attempt, "error", err)
		if nackErr := msg.Nack(false, false); nackErr != nil {
			c.log.Error("nack failed", "queue", c.queue, "error", nackErr)
		}

	default:
		c.log.Warn("task failed, will retry", "queue", c.queue,
			"attempt", attempt, "error", err)
		if nackErr := msg.Nack(false, false); nackErr != nil {
			c.log.Error("nack failed", "queue", c.queue, "error", nackErr)
		}
	}
}

// Drain stops new deliveries and waits for in-flight work to finish.
//
// This is step 1 of the SIGTERM sequence in plan.md §13.3. Cancelling the
// consumer is what makes requeue-on-shutdown unnecessary for work not yet
// started, so only genuinely in-flight tasks can be redelivered — and those are
// safe because findings are idempotent on their fingerprint (plan.md §6).
func (c *Consumer) Drain(ctx context.Context) error {
	var cancelErr error
	c.cancelOnce.Do(func() {
		cancelErr = c.ch.Cancel(c.consumerTag(), false)
	})
	if cancelErr != nil {
		return fmt.Errorf("rabbit: cancel consumer %s: %w", c.queue, cancelErr)
	}

	select {
	case <-c.drained:
		return c.ch.Close()
	case <-ctx.Done():
		return fmt.Errorf("rabbit: drain %s timed out: %w", c.queue, ctx.Err())
	}
}

// AttemptCount reads how many times the broker has dead-lettered this message.
func AttemptCount(h amqp.Table) int {
	deaths, ok := h["x-death"].([]any)
	if !ok || len(deaths) == 0 {
		return 0
	}
	entry, ok := deaths[0].(amqp.Table)
	if !ok {
		return 0
	}
	count, ok := entry["count"].(int64)
	if !ok {
		return 0
	}
	return int(count)
}
