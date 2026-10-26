package app_test

import (
	"github.com/mservice/gitscanner/internal/app"

	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func testEnv(t *testing.T) {
	t.Helper()
	t.Setenv("HTTP_ADDR", "127.0.0.1:0")
	t.Setenv("LOG_LEVEL", "error")
	t.Setenv("SHUTDOWN_TIMEOUT", "2s")
	for _, k := range []string{"POSTGRES_URL", "RABBITMQ_URL"} {
		t.Setenv(k, "")
	}
}

func specWithWorker(w app.Worker) app.Spec {
	return app.Spec{
		Name: "test",
		Build: func(context.Context, *app.Deps) (app.Parts, error) {
			return app.Parts{Worker: w}, nil
		},
	}
}

func TestRunEndsWhenWorkerFinishes(t *testing.T) {
	testEnv(t)
	err := app.Run(specWithWorker(app.WorkerFunc(func(context.Context) error { return nil })))
	if err != nil {
		t.Fatalf("Run = %v, want nil when the worker finishes cleanly", err)
	}
}

func TestRunReturnsWorkerError(t *testing.T) {
	testEnv(t)
	boom := errors.New("boom")
	err := app.Run(specWithWorker(app.WorkerFunc(func(context.Context) error { return boom })))
	if !errors.Is(err, boom) {
		t.Fatalf("Run = %v, want the worker's error so the process exits non-zero", err)
	}
}

func TestRunReportsAllMissingConfig(t *testing.T) {
	testEnv(t)
	err := app.Run(app.Spec{Name: "test", Needs: app.Needs{Postgres: true, Rabbit: true}})
	if err == nil {
		t.Fatal("Run succeeded with no backend URLs configured")
	}
	for _, want := range []string{"POSTGRES_URL", "RABBITMQ_URL"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %s", err, want)
		}
	}
}

func TestRunFailsWhenBuildFails(t *testing.T) {
	testEnv(t)
	boom := errors.New("registry not loadable")
	err := app.Run(app.Spec{Name: "test", Build: func(context.Context, *app.Deps) (app.Parts, error) {
		return app.Parts{}, boom
	}})
	if !errors.Is(err, boom) {
		t.Fatalf("Run = %v, want the build error", err)
	}
}

func TestRunFailsWhenPortTaken(t *testing.T) {
	testEnv(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	t.Setenv("HTTP_ADDR", ln.Addr().String())

	if err := app.Run(app.Spec{Name: "test"}); err == nil {
		t.Fatal("Run must fail when its port is taken, not run unreachable")
	}
}

// drainWorker records whether Drain ran before its context was cancelled,
// which is the order the SIGTERM sequence in plan.md §13.3 depends on.
type drainWorker struct {
	started          chan struct{}
	drainedBeforeCtx atomic.Bool
	ctxCancelled     atomic.Bool
}

func (w *drainWorker) Run(ctx context.Context) error {
	close(w.started)
	<-ctx.Done()
	w.ctxCancelled.Store(true)
	return ctx.Err()
}

func (w *drainWorker) Drain(context.Context) error {
	w.drainedBeforeCtx.Store(!w.ctxCancelled.Load())
	return nil
}

func TestSIGTERMDrainsBeforeCancelling(t *testing.T) {
	testEnv(t)
	w := &drainWorker{started: make(chan struct{})}

	done := make(chan error, 1)
	go func() { done <- app.Run(specWithWorker(w)) }()

	select {
	case <-w.started:
	case <-time.After(5 * time.Second):
		t.Fatal("worker never started")
	}
	if err := syscall.Kill(syscall.Getpid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run = %v, want a clean shutdown", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after SIGTERM")
	}

	if !w.drainedBeforeCtx.Load() {
		t.Error("Drain must run before the worker's context is cancelled")
	}
	if !w.ctxCancelled.Load() {
		t.Error("worker context was never cancelled")
	}
}
