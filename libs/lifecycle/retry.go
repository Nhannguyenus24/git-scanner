package lifecycle

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// Backoff bounds how Retry spaces its attempts.
type Backoff struct {
	// Budget is the total time Retry may spend before giving up.
	Budget time.Duration
	// Initial is the first wait; each wait after it doubles up to Max.
	Initial time.Duration
	Max     time.Duration
	// PerAttempt bounds one attempt, so a backend that accepts the connection
	// and never answers cannot eat the whole budget in a single try.
	PerAttempt time.Duration
}

// StartupBackoff suits dialling a backend at process start: about a minute in
// all, which covers a broker or database that is itself still starting.
func StartupBackoff(budget time.Duration) Backoff {
	return Backoff{Budget: budget, Initial: time.Second, Max: 10 * time.Second, PerAttempt: 10 * time.Second}
}

// Retry calls fn until it succeeds, ctx is cancelled, or the budget runs out,
// and returns the last error.
//
// Services start in no particular order — a local stack brings Postgres and
// the broker up alongside the workers, a node restart brings everything up at
// once — so a backend that is not answering yet is the normal case at startup,
// not a fault. Failing on the first refusal turns that into a crash loop whose
// backoff is the orchestrator's, and outside an orchestrator into a service
// that is simply not running.
func Retry(ctx context.Context, log *slog.Logger, name string, b Backoff, fn func(context.Context) error) error {
	deadline := time.Now().Add(b.Budget)
	wait := b.Initial
	for attempt := 1; ; attempt++ {
		actx, cancel := context.WithTimeout(ctx, b.PerAttempt)
		err := fn(actx)
		cancel()
		if err == nil {
			if attempt > 1 {
				log.Info("backend reachable", "backend", name, "attempts", attempt)
			}
			return nil
		}
		if ctx.Err() != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if time.Now().Add(wait).After(deadline) {
			return fmt.Errorf("%s: gave up after %d attempts: %w", name, attempt, err)
		}
		log.Warn("backend not reachable yet, retrying", "backend", name, "attempt", attempt, "retry_in", wait.String(), "error", err)
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: %w", name, err)
		case <-time.After(wait):
		}
		wait = min(wait*2, b.Max)
	}
}
