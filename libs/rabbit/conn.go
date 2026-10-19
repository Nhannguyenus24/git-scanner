// Package rabbit wraps amqp091 with this system's queue conventions.
//
// What it encodes (plan.md §3.4): topology with a dead-letter exchange and TTL
// backoff, publisher confirms, bounded prefetch, and a consumer whose cancel path
// matches the Kubernetes drain in §13.3. It does not try to hide amqp091.
//
// Files:
//
//	conn.go       connection and queue topology
//	publisher.go  confirmed publishing
//	consumer.go   bounded consumption, retry routing and drain
package rabbit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
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

	// retryDelay is how long a failed task waits in its retry queue before it
	// is redelivered.
	retryDelay = 30 * time.Second
)

// Queues is every task queue the topology declares. Adding a queue here is the
// only step needed for it to get retry and dead-letter queues too.
var Queues = []string{QueueDiscovery, QueueScan, QueueEnrich, QueueNotify}

// Conn is a managed AMQP connection.
type Conn struct {
	log  *slog.Logger
	conn *amqp.Connection

	mu sync.Mutex
}

// Connect dials RabbitMQ and declares the topology. ctx bounds the dial, so a
// broker that accepts TCP but never answers cannot hang startup.
func Connect(ctx context.Context, url string, log *slog.Logger) (*Conn, error) {
	var d net.Dialer
	conn, err := amqp.DialConfig(url, amqp.Config{
		Dial: func(network, addr string) (net.Conn, error) {
			return d.DialContext(ctx, network, addr)
		},
	})
	if err != nil {
		return nil, fmt.Errorf("rabbit: dial: %w", err)
	}

	c := &Conn{log: log, conn: conn}
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

	for _, q := range Queues {
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
			"x-message-ttl":             int32(retryDelay.Milliseconds()),
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
