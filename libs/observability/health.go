package observability

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// CheckFunc reports whether one dependency is usable. It must respect ctx.
type CheckFunc func(context.Context) error

// DefaultCheckTimeout bounds one /readyz evaluation across all checks. It stays
// below the kubelet's probe period so a slow dependency reads as a clear
// failure with a reason, not as an anonymous probe timeout.
const DefaultCheckTimeout = 3 * time.Second

type namedCheck struct {
	name string
	fn   CheckFunc
}

// Health tracks readiness. It is separate from liveness on purpose: a worker
// that has not yet loaded the provider registry is alive but must not be ready,
// or it would report zero findings while looking healthy (plan.md §13.9).
type Health struct {
	ready atomic.Bool

	mu     sync.RWMutex
	checks []namedCheck // ordered, so failures are reported deterministically
}

// NewHealth returns a Health that is not ready and has no checks.
func NewHealth() *Health { return &Health{} }

// AddCheck registers a readiness dependency. Checks run in registration order.
func (h *Health) AddCheck(name string, fn CheckFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks = append(h.checks, namedCheck{name: name, fn: fn})
}

// SetReady marks whether the service has finished startup.
func (h *Health) SetReady(v bool) { h.ready.Store(v) }

// ErrNotReady is returned by Check before SetReady(true).
var ErrNotReady = errors.New("starting up")

// Check evaluates readiness: the startup flag first, then every dependency.
// It returns the first failure, prefixed with the check name.
func (h *Health) Check(ctx context.Context) error {
	if !h.ready.Load() {
		return ErrNotReady
	}

	h.mu.RLock()
	checks := h.checks
	h.mu.RUnlock()

	for _, c := range checks {
		if err := runCheck(ctx, c.fn); err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
	}
	return nil
}

// runCheck enforces the deadline even when a driver ignores ctx during dial
// (franz-go uses its own dial timeout). A probe that hangs past the kubelet's
// timeout reads as a failure anyway, but without the reason in the log.
func runCheck(ctx context.Context, check CheckFunc) error {
	done := make(chan error, 1)
	go func() { done <- check(ctx) }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
