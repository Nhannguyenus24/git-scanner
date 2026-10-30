//go:build integration

// Where a failed task ends up. This is broker behaviour — which exchange a nack
// routes through, what x-death says — so it is tested against a real broker:
// a fake would only repeat this package's own assumptions back to it, and
// those assumptions are exactly what went wrong once (every "dead-lettered"
// task was in fact retried every 30 seconds, forever).
//
// Run with: make test-integration, with RABBITMQ_TEST_URL pointing at a
// disposable vhost. The tests purge enrich.tasks and its retry and dead queues,
// so never point it at a vhost a running stack consumes from.
package rabbit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/mservice/gitscanner/libs/rabbit"
)

const queue = rabbit.QueueEnrich

func dial(t *testing.T) (*rabbit.Conn, *amqp.Channel) {
	t.Helper()
	url := os.Getenv("RABBITMQ_TEST_URL")
	if url == "" {
		t.Skip("RABBITMQ_TEST_URL not set")
	}
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	conn, err := rabbit.Connect(ctx, url, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	// A second, raw connection to look inside queues the package does not
	// expose a reader for.
	raw, err := amqp.Dial(url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = raw.Close() })
	ch, err := raw.Channel()
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{queue, queue + ".retry", queue + ".dead"} {
		if _, err := ch.QueuePurge(q, false); err != nil {
			t.Fatal(err)
		}
	}
	return conn, ch
}

// consume runs a consumer with h until the test ends.
func consume(t *testing.T, conn *rabbit.Conn, maxRetry int, h rabbit.Handler) {
	t.Helper()
	c, err := conn.NewConsumer(queue, 1, maxRetry)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { _ = c.Run(ctx, h); close(done) }()
	t.Cleanup(func() {
		dctx, dcancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dcancel()
		_ = c.Drain(dctx)
		cancel()
		<-done
	})
}

func publish(t *testing.T, conn *rabbit.Conn, body string) {
	t.Helper()
	p, err := conn.NewPublisher()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()
	if err := p.Publish(context.Background(), queue, []byte(body), nil); err != nil {
		t.Fatal(err)
	}
}

// await polls q until a message arrives or the deadline passes.
func await(t *testing.T, ch *amqp.Channel, q string) amqp.Delivery {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		msg, ok, err := ch.Get(q, true)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			return msg
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no message reached %s", q)
	return amqp.Delivery{}
}

func depth(t *testing.T, ch *amqp.Channel, q string) int {
	t.Helper()
	info, err := ch.QueueDeclarePassive(q, true, false, false, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	return info.Messages
}

func TestDropReachesTheDeadQueue(t *testing.T) {
	conn, ch := dial(t)
	consume(t, conn, 5, func(context.Context, rabbit.Delivery) error {
		return errors.Join(rabbit.ErrDrop, errors.New("malformed"))
	})
	publish(t, conn, "poison")

	msg := await(t, ch, queue+".dead")
	if string(msg.Body) != "poison" {
		t.Errorf("body = %q", msg.Body)
	}
	if reason, _ := msg.Headers[rabbit.HeaderDeadReason].(string); reason == "" {
		t.Errorf("dead message carries no reason; headers %v", msg.Headers)
	}
	if n := depth(t, ch, queue+".retry"); n != 0 {
		t.Errorf("a dropped task was also queued for retry (%d)", n)
	}
}

func TestFinalAttemptDeadLetters(t *testing.T) {
	conn, ch := dial(t)
	sawFinal := make(chan bool, 1)
	// maxRetry 0 makes the first delivery the last, so the test does not wait
	// out the broker's retry delay.
	consume(t, conn, 0, func(_ context.Context, d rabbit.Delivery) error {
		sawFinal <- d.Final
		return errors.New("still broken")
	})
	publish(t, conn, "exhausted")

	if !<-sawFinal {
		t.Error("handler was not told this was its final attempt")
	}
	await(t, ch, queue+".dead")
	if n := depth(t, ch, queue+".retry"); n != 0 {
		t.Errorf("an exhausted task was queued for retry (%d)", n)
	}
}

func TestOrdinaryFailureRetries(t *testing.T) {
	conn, ch := dial(t)
	consume(t, conn, 5, func(context.Context, rabbit.Delivery) error {
		return errors.New("transient")
	})
	publish(t, conn, "flaky")

	msg := await(t, ch, queue+".retry")
	if got := rabbit.AttemptCount(msg.Headers); got != 1 {
		t.Errorf("attempt after one failure = %d, want 1", got)
	}
}

func TestRetryLaterDoesNotSpendAnAttempt(t *testing.T) {
	conn, ch := dial(t)
	consume(t, conn, 5, func(context.Context, rabbit.Delivery) error {
		return errors.Join(rabbit.ErrRetryLater, errors.New("rate limited"))
	})
	publish(t, conn, "throttled")

	msg := await(t, ch, queue+".retry")
	if got := rabbit.AttemptCount(msg.Headers); got != 0 {
		t.Errorf("a deferred task was counted as attempt %d", got)
	}
	if n := depth(t, ch, queue+".dead"); n != 0 {
		t.Errorf("a deferred task was dead-lettered (%d)", n)
	}
}

func TestRunFailsWhenTheConnectionDrops(t *testing.T) {
	conn, _ := dial(t)
	c, err := conn.NewConsumer(queue, 1, 5)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- c.Run(context.Background(), func(context.Context, rabbit.Delivery) error { return nil }) }()

	time.Sleep(200 * time.Millisecond)
	_ = conn.Close()

	select {
	case err := <-done:
		if err == nil {
			t.Error("Run returned nil after losing its connection; the process would exit as if finished")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the connection closed")
	}
}
