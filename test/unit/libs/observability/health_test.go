package observability_test

import (
	"github.com/mservice/gitscanner/libs/observability"

	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCheckNotReadyBeforeStartup(t *testing.T) {
	h := observability.NewHealth()
	if err := h.Check(context.Background()); !errors.Is(err, observability.ErrNotReady) {
		t.Fatalf("Check before SetReady = %v, want ErrNotReady", err)
	}
}

func TestCheckReportsFirstFailureInOrder(t *testing.T) {
	h := observability.NewHealth()
	h.SetReady(true)
	h.AddCheck("postgres", func(context.Context) error { return nil })
	h.AddCheck("redis", func(context.Context) error { return errors.New("down") })
	h.AddCheck("redis", func(context.Context) error { return errors.New("also down") })

	err := h.Check(context.Background())
	if err == nil || !strings.HasPrefix(err.Error(), "redis:") {
		t.Fatalf("Check = %v, want the redis failure reported first", err)
	}
}

// A driver that ignores its context must not stretch the probe past its budget.
func TestCheckEnforcesDeadline(t *testing.T) {
	h := observability.NewHealth()
	h.SetReady(true)
	h.AddCheck("stuck", func(context.Context) error {
		time.Sleep(time.Second)
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	if err := h.Check(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Check = %v, want deadline exceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("Check took %v; the deadline was not enforced", elapsed)
	}
}

func TestProbeEndpoints(t *testing.T) {
	h := observability.NewHealth()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := observability.NewServer(":0", log, h, nil)

	get := func(path string) int {
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		return rec.Code
	}

	if code := get("/healthz"); code != http.StatusOK {
		t.Errorf("/healthz = %d, want 200 even before ready", code)
	}
	if code := get("/readyz"); code != http.StatusServiceUnavailable {
		t.Errorf("/readyz before ready = %d, want 503", code)
	}

	h.SetReady(true)
	if code := get("/readyz"); code != http.StatusOK {
		t.Errorf("/readyz when ready = %d, want 200", code)
	}

	h.AddCheck("broker", func(context.Context) error { return errors.New("gone") })
	if code := get("/readyz"); code != http.StatusServiceUnavailable {
		t.Errorf("/readyz with failing check = %d, want 503", code)
	}
}
