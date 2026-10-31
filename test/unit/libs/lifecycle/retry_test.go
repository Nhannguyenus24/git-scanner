package lifecycle_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mservice/gitscanner/libs/lifecycle"
)


func fast(budget time.Duration) lifecycle.Backoff {
	return lifecycle.Backoff{Budget: budget, Initial: time.Millisecond, Max: 4 * time.Millisecond, PerAttempt: time.Second}
}

// A backend that comes up while the service is waiting is the case this exists
// for: compose and a node restart start everything at once.
func TestRetrySucceedsOnceTheBackendAnswers(t *testing.T) {
	calls := 0
	err := lifecycle.Retry(context.Background(), quiet, "db", fast(time.Second), func(context.Context) error {
		calls++
		if calls < 3 {
			return errors.New("connection refused")
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Fatalf("err = %v after %d calls, want success on the third", err, calls)
	}
}

func TestRetryGivesUpWithTheLastError(t *testing.T) {
	refused := errors.New("connection refused")
	err := lifecycle.Retry(context.Background(), quiet, "db", fast(20*time.Millisecond), func(context.Context) error {
		return refused
	})
	if !errors.Is(err, refused) {
		t.Fatalf("err = %v, want it to wrap the backend's own error", err)
	}
}

// SIGTERM during startup must not wait out the whole budget.
func TestRetryStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	start := time.Now()
	err := lifecycle.Retry(ctx, quiet, "db", lifecycle.Backoff{Budget: time.Minute, Initial: 50 * time.Millisecond, Max: time.Second, PerAttempt: time.Second},
		func(context.Context) error {
			calls++
			cancel()
			return errors.New("refused")
		})
	if err == nil || calls != 1 || time.Since(start) > time.Second {
		t.Fatalf("err = %v, calls = %d, elapsed %s: want a prompt failure after one call", err, calls, time.Since(start))
	}
}

// One attempt is bounded on its own, so a backend that accepts and never
// answers cannot consume the whole budget in a single try.
func TestRetryBoundsEachAttempt(t *testing.T) {
	b := lifecycle.Backoff{Budget: 200 * time.Millisecond, Initial: time.Millisecond, Max: time.Millisecond, PerAttempt: 20 * time.Millisecond}
	calls := 0
	_ = lifecycle.Retry(context.Background(), quiet, "db", b, func(ctx context.Context) error {
		calls++
		<-ctx.Done()
		return ctx.Err()
	})
	if calls < 2 {
		t.Fatalf("a hung attempt was not cut short: %d calls", calls)
	}
}
