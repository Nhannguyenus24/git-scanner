// Package observability wires logging, metrics and health endpoints.
//
// The HTTP server here is the one Kubernetes talks to: /healthz for liveness,
// /readyz for readiness and /metrics for Prometheus. Keeping it separate from a
// service's business listener means a saturated API port cannot make a pod look
// dead to the kubelet.
package observability

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync/atomic"
	"time"
)

// NewLogger returns a JSON logger at the given level, tagged with the service
// name so logs from six workloads remain separable in one stream.
func NewLogger(serviceName, level string) *slog.Logger {
	var lvl slog.Level
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h).With("service", serviceName)
}

// Health tracks readiness. It is separate from liveness on purpose: a worker
// that has not yet loaded the provider registry is alive but must not be ready,
// or it would report zero findings while looking healthy (plan.md §13.9).
type Health struct {
	ready atomic.Bool
	// checks run on each /readyz call: dependency pings that must stay true.
	checks map[string]func(context.Context) error
}

func NewHealth() *Health {
	return &Health{checks: make(map[string]func(context.Context) error)}
}

// AddCheck registers a readiness dependency.
func (h *Health) AddCheck(name string, fn func(context.Context) error) {
	h.checks[name] = fn
}

// SetReady marks the service as having finished startup.
func (h *Health) SetReady(v bool) { h.ready.Store(v) }

// runCheck enforces the deadline even when a driver ignores ctx during dial
// (franz-go uses its own dial timeout). A probe that hangs past the kubelet's
// timeout reads as a failure anyway, but without the reason in the log.
func runCheck(ctx context.Context, check func(context.Context) error) error {
	done := make(chan error, 1)
	go func() { done <- check(ctx) }()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Server is the health and metrics listener.
type Server struct {
	srv    *http.Server
	log    *slog.Logger
	health *Health
}

// NewServer builds the probe listener. Extra routes let a service mount its own
// handlers on the same port when it has no separate business listener.
func NewServer(addr string, log *slog.Logger, health *Health, extra http.Handler) *Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !health.ready.Load() {
			http.Error(w, "starting up", http.StatusServiceUnavailable)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		for name, check := range health.checks {
			if err := runCheck(ctx, check); err != nil {
				log.Warn("readiness check failed", "check", name, "error", err)
				http.Error(w, name+": "+err.Error(), http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	if extra != nil {
		mux.Handle("/", extra)
	}

	return &Server{
		srv: &http.Server{
			Addr:              addr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		},
		log:    log,
		health: health,
	}
}

// Start runs the listener until Shutdown is called.
func (s *Server) Start() {
	go func() {
		s.log.Info("probe listener starting", "addr", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.log.Error("probe listener failed", "error", err)
		}
	}()
}

// Shutdown stops the listener.
func (s *Server) Shutdown(ctx context.Context) error {
	s.health.SetReady(false)
	return s.srv.Shutdown(ctx)
}
