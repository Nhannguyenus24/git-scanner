// Package runtime holds the process lifecycle convention every service follows.
//
// The shape matters for Kubernetes: on SIGTERM a service must stop accepting new
// work, finish or hand back what it holds, and exit before
// terminationGracePeriodSeconds expires (plan.md §13.3). Getting this wrong in a
// queue consumer means lost or duplicated work on every rollout.
package runtime

import (
	"context"
	"log/slog"
	"os/signal"
	"syscall"
	"time"
)

// SignalContext returns a context cancelled on SIGINT or SIGTERM.
func SignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

// Component is anything with an ordered shutdown.
type Component struct {
	Name string
	Stop func(context.Context) error
}

// Shutdown stops components in the given order within a single deadline.
//
// Order is the caller's responsibility and it is not arbitrary: stop the work
// intake first (queue consumers, HTTP listeners), then flush state, then close
// the connections that flushing needs. Closing the database first would discard
// the cursor the drain is trying to save.
func Shutdown(log *slog.Logger, timeout time.Duration, components ...Component) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, c := range components {
		if c.Stop == nil {
			continue
		}

		start := time.Now()
		if err := c.Stop(ctx); err != nil {
			log.Error("shutdown step failed", "component", c.Name, "error", err,
				"elapsed", time.Since(start))
			continue
		}
		log.Info("shutdown step done", "component", c.Name, "elapsed", time.Since(start))

		if ctx.Err() != nil {
			log.Error("shutdown deadline exceeded; remaining components skipped",
				"after", c.Name)
			return
		}
	}
}
