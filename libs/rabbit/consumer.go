package rabbit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Delivery is one task handed to a Handler.
type Delivery struct {
	Body    []byte
	Headers map[string]any
	// Attempt counts prior deliveries, derived from the dead-letter header.
	Attempt int
	// Final is set on the last attempt the consumer will make: if the handler
	// fails now, the task goes to the dead-letter queue and nothing will run it
	// again. A handler uses it to leave its own state consistent — mark the job
	// failed, count the repository — instead of leaving a job that waits forever
	// for a task that is gone.
	Final bool
}

// Handler processes one task. Returning an error sends the task to the retry
// queue; returning ErrDrop sends it straight to the dead-letter queue, and
// ErrRetryLater sends it to the retry queue without spending an attempt.
type Handler func(ctx context.Context, d Delivery) error

// ErrDrop marks a task as permanently unprocessable, skipping retries.
var ErrDrop = errors.New("drop task")

// ErrRetryLater marks a failure that is not the task's fault and says nothing
// about whether it will ever succeed — an upstream rate limit, say. The task
// waits out the retry delay like any other, but its attempt count starts over,
// so an hour-long rate limit does not dead-letter work that is perfectly good.
var ErrRetryLater = errors.New("retry later")

// Headers stamped on a dead-lettered message so whoever inspects the .dead
// queue can see why it is there without digging through logs.
const (
	HeaderDeadReason   = "x-gitscanner-dead-reason"
	HeaderDeadAttempts = "x-gitscanner-dead-attempts"
)

// maxReasonBytes bounds the error text copied into a header. Errors can carry
// git's stderr, which has no length limit of its own.
const maxReasonBytes = 1024

// Consumer reads one queue with bounded prefetch.
type Consumer struct {
	conn     *Conn
	ch       *amqp.Channel
	queue    string
	log      *slog.Logger
	maxRetry int

	cancelOnce sync.Once
	draining   atomic.Bool
	drained    chan struct{}
}

// NewConsumer opens a channel with the given prefetch.
//
// Prefetch is the backpressure mechanism: it bounds how much work a pod holds,
// which is what lets KEDA scale on queue depth meaningfully (plan.md §13.4). An
// unbounded consumer would drain the queue into one pod's memory and hide the
// backlog from the scaler.
//
// The channel is also in confirm mode, because the consumer publishes: it
// moves dead and retry-later messages itself, and acks the original only once
// the broker has confirmed the copy. Without the confirm, a broker restart
// between the two would lose the task.
func (c *Conn) NewConsumer(queue string, prefetch, maxRetry int) (*Consumer, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbit: consumer channel: %w", err)
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("rabbit: set qos: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("rabbit: enable confirms on consumer: %w", err)
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
//
// Any other end — the broker closing the channel or the connection dropping —
// is an error. Returning nil there would exit the process cleanly with nothing
// consuming the queue and nothing in the logs to say why.
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

	if !c.draining.Load() && ctx.Err() == nil {
		return fmt.Errorf("rabbit: delivery channel for %s closed by the broker or a lost connection", c.queue)
	}

	// The channel closes when the broker confirms the cancel, which happens only
	// after every outstanding delivery has been acked or nacked. Reaching here
	// means this consumer has genuinely drained.
	c.log.Info("consumer drained", "queue", c.queue)
	return nil
}

func (c *Consumer) handle(ctx context.Context, h Handler, msg amqp.Delivery) {
	attempt := AttemptCount(msg.Headers)
	final := attempt >= c.maxRetry
	c.log.Debug("delivery received", "queue", c.queue, "attempt", attempt, "size", len(msg.Body))

	err := h(ctx, Delivery{
		Body:    msg.Body,
		Headers: msg.Headers,
		Attempt: attempt,
		Final:   final,
	})

	switch {
	case err == nil:
		if ackErr := msg.Ack(false); ackErr != nil {
			c.log.Error("ack failed", "queue", c.queue, "error", ackErr)
		} else {
			c.log.Debug("delivery acked", "queue", c.queue)
		}

	// Checked before the attempt limit: a rate limit on the last attempt is
	// still not the task's fault.
	case errors.Is(err, ErrRetryLater) && !errors.Is(err, ErrDrop):
		c.log.Warn("task deferred, attempt not counted", "queue", c.queue,
			"attempt", attempt, "error", err)
		c.moveThenAck(ctx, msg, exchangeRetry, freshHeaders(msg.Headers))

	case errors.Is(err, ErrDrop), final:
		c.log.Error("task dead-lettered", "queue", c.queue,
			"attempt", attempt, "error", err)
		c.moveThenAck(ctx, msg, exchangeDead, deadHeaders(msg.Headers, err, attempt))

	default:
		c.log.Warn("task failed, will retry", "queue", c.queue,
			"attempt", attempt, "error", err)
		// The queue's dead-letter exchange is the retry exchange, so a nack
		// without requeue is the retry: the broker counts it in x-death.
		if nackErr := msg.Nack(false, false); nackErr != nil {
			c.log.Error("nack failed", "queue", c.queue, "error", nackErr)
		}
	}
}

// moveThenAck republishes msg to exchange under its queue's routing key and
// acks the original once the copy is confirmed.
//
// The queue's own dead-letter exchange is the retry exchange, so a plain nack
// can only ever retry. Reaching the dead queue — or retrying without the broker
// counting an attempt — takes an explicit publish. If the publish fails the
// message is nacked instead, which retries it: a task that runs once more is
// recoverable, a task that vanished is not.
func (c *Consumer) moveThenAck(ctx context.Context, msg amqp.Delivery, exchange string, headers amqp.Table) {
	// Bounded on its own clock: a drain cancels ctx, and the in-flight message
	// still has to land somewhere.
	pubCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()

	err := c.publishCopy(pubCtx, msg, exchange, headers)
	if err != nil {
		c.log.Error("could not move task, retrying it instead", "queue", c.queue, "exchange", exchange, "error", err)
		if nackErr := msg.Nack(false, false); nackErr != nil {
			c.log.Error("nack failed", "queue", c.queue, "error", nackErr)
		}
		return
	}
	if ackErr := msg.Ack(false); ackErr != nil {
		c.log.Error("ack failed", "queue", c.queue, "error", ackErr)
	}
}

func (c *Consumer) publishCopy(ctx context.Context, msg amqp.Delivery, exchange string, headers amqp.Table) error {
	conf, err := c.ch.PublishWithDeferredConfirmWithContext(ctx, exchange, c.queue, true, false, amqp.Publishing{
		ContentType:  msg.ContentType,
		DeliveryMode: amqp.Persistent,
		Timestamp:    msg.Timestamp,
		MessageId:    msg.MessageId,
		Body:         msg.Body,
		Headers:      headers,
	})
	if err != nil {
		return err
	}
	ok, err := conf.WaitContext(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("broker rejected the copy")
	}
	return nil
}

// deadHeaders keeps the original headers, so x-death still shows the retry
// history, and adds why the task stopped.
func deadHeaders(h amqp.Table, err error, attempt int) amqp.Table {
	out := amqp.Table{}
	for k, v := range h {
		out[k] = v
	}
	reason := err.Error()
	if len(reason) > maxReasonBytes {
		reason = reason[:maxReasonBytes]
	}
	out[HeaderDeadReason] = reason
	out[HeaderDeadAttempts] = int64(attempt)
	return out
}

// freshHeaders drops x-death so the broker starts counting again. The header is
// the broker's own bookkeeping; a copy published by a client is a new message.
func freshHeaders(h amqp.Table) amqp.Table {
	out := amqp.Table{}
	for k, v := range h {
		if k == "x-death" || k == "x-first-death-exchange" || k == "x-first-death-queue" || k == "x-first-death-reason" ||
			k == "x-last-death-exchange" || k == "x-last-death-queue" || k == "x-last-death-reason" {
			continue
		}
		out[k] = v
	}
	return out
}

// Drain stops new deliveries and waits for in-flight work to finish.
//
// This is step 1 of the SIGTERM sequence in plan.md §13.3. Cancelling the
// consumer is what makes requeue-on-shutdown unnecessary for work not yet
// started, so only genuinely in-flight tasks can be redelivered — and those are
// safe because findings are idempotent on their fingerprint (plan.md §6).
func (c *Consumer) Drain(ctx context.Context) error {
	c.draining.Store(true)
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
