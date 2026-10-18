// Package rabbit wraps amqp091 with this system's queue conventions.
//
// What it encodes (plan.md §3.4): topology with a dead-letter exchange and TTL
// backoff, publisher confirms, bounded prefetch, and a consumer whose cancel path
// matches the Kubernetes drain in §13.3. It does not try to hide amqp091.
package rabbit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Queue names. Declared here so publisher and consumer cannot disagree.
const (
	QueueDiscovery = "discovery.tasks"
	QueueScan      = "scan.tasks"
	QueueEnrich    = "enrich.tasks"
	QueueNotify    = "notify.tasks"

	exchangeMain  = "gitscanner"
	exchangeRetry = "gitscanner.retry"
	exchangeDead  = "gitscanner.dead"
)

// Conn is a managed AMQP connection.
type Conn struct {
	url  string
	log  *slog.Logger
	conn *amqp.Connection

	mu sync.Mutex
}

// Connect dials RabbitMQ and declares the topology.
func Connect(ctx context.Context, url string, log *slog.Logger) (*Conn, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("rabbit: dial: %w", err)
	}

	c := &Conn{url: url, log: log, conn: conn}
	if err := c.declareTopology(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

// declareTopology sets up main, retry and dead-letter exchanges.
//
// The retry path is a queue with a message TTL whose dead-letter target is the
// main exchange: a nacked message lands in retry, waits out the TTL, and is
// redelivered. That gives backoff without a scheduler and without the consumer
// blocking a prefetch slot while it sleeps.
func (c *Conn) declareTopology() error {
	ch, err := c.conn.Channel()
	if err != nil {
		return fmt.Errorf("rabbit: channel: %w", err)
	}
	defer func() { _ = ch.Close() }()

	for _, ex := range []string{exchangeMain, exchangeRetry, exchangeDead} {
		if err := ch.ExchangeDeclare(ex, "direct", true, false, false, false, nil); err != nil {
			return fmt.Errorf("rabbit: declare exchange %s: %w", ex, err)
		}
	}

	for _, q := range []string{QueueDiscovery, QueueScan, QueueEnrich, QueueNotify} {
		if _, err := ch.QueueDeclare(q, true, false, false, false, amqp.Table{
			"x-dead-letter-exchange":    exchangeRetry,
			"x-dead-letter-routing-key": q,
		}); err != nil {
			return fmt.Errorf("rabbit: declare queue %s: %w", q, err)
		}
		if err := ch.QueueBind(q, q, exchangeMain, false, nil); err != nil {
			return fmt.Errorf("rabbit: bind queue %s: %w", q, err)
		}

		retryQ := q + ".retry"
		if _, err := ch.QueueDeclare(retryQ, true, false, false, false, amqp.Table{
			"x-dead-letter-exchange":    exchangeMain,
			"x-dead-letter-routing-key": q,
			"x-message-ttl":             int32(30000),
		}); err != nil {
			return fmt.Errorf("rabbit: declare retry queue %s: %w", retryQ, err)
		}
		if err := ch.QueueBind(retryQ, q, exchangeRetry, false, nil); err != nil {
			return fmt.Errorf("rabbit: bind retry queue %s: %w", retryQ, err)
		}

		deadQ := q + ".dead"
		if _, err := ch.QueueDeclare(deadQ, true, false, false, false, nil); err != nil {
			return fmt.Errorf("rabbit: declare dead queue %s: %w", deadQ, err)
		}
		if err := ch.QueueBind(deadQ, q, exchangeDead, false, nil); err != nil {
			return fmt.Errorf("rabbit: bind dead queue %s: %w", deadQ, err)
		}
	}

	return nil
}

// Healthy reports whether the connection is still open.
func (c *Conn) Healthy(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil || c.conn.IsClosed() {
		return errors.New("rabbit: connection closed")
	}
	return nil
}

// Close shuts the connection down.
func (c *Conn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil || c.conn.IsClosed() {
		return nil
	}
	return c.conn.Close()
}

// Publisher sends tasks with publisher confirms.
type Publisher struct {
	ch  *amqp.Channel
	log *slog.Logger
	mu  sync.Mutex
}

// NewPublisher opens a confirming channel.
//
// Confirms are not optional here: without them a broker restart silently drops
// in-flight publishes, and a scan task that is never enqueued is a job that hangs
// in "running" forever with no error anywhere.
func (c *Conn) NewPublisher() (*Publisher, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbit: publisher channel: %w", err)
	}
	if err := ch.Confirm(false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("rabbit: enable confirms: %w", err)
	}
	return &Publisher{ch: ch, log: c.log}, nil
}

// Publish sends body to a queue and waits for the broker to confirm it.
func (p *Publisher) Publish(ctx context.Context, queue string, body []byte, headers map[string]any) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	conf, err := p.ch.PublishWithDeferredConfirmWithContext(ctx,
		exchangeMain, queue, true, false,
		amqp.Publishing{
			ContentType:  "application/x-protobuf",
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			Body:         body,
			Headers:      headers,
		})
	if err != nil {
		return fmt.Errorf("rabbit: publish to %s: %w", queue, err)
	}

	ok, err := conf.WaitContext(ctx)
	if err != nil {
		return fmt.Errorf("rabbit: await confirm for %s: %w", queue, err)
	}
	if !ok {
		return fmt.Errorf("rabbit: broker rejected publish to %s", queue)
	}
	return nil
}

// Close shuts the publisher channel.
func (p *Publisher) Close() error { return p.ch.Close() }

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
	attempt := deathCount(msg.Headers)

	err := h(ctx, Delivery{
		Body:    msg.Body,
		Headers: msg.Headers,
		Attempt: attempt,
	})

	switch {
	case err == nil:
		if ackErr := msg.Ack(false); ackErr != nil {
			c.log.Error("ack failed", "queue", c.queue, "error", ackErr)
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

// deathCount reads how many times the broker has dead-lettered this message.
func deathCount(h amqp.Table) int {
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
