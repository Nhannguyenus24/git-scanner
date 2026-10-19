package rabbit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

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
