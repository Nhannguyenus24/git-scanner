// Package app is the one bootstrap every service binary goes through.
//
// A service declares what it is in a Spec; Run does the rest in a fixed order:
//
//	load config → connect backends → Build → serve probes → ready
//	→ run until SIGTERM or the worker exits
//	→ unready → drain the worker → flush and close backends
//
// Keeping this in one place is the point. Six main() functions would otherwise
// repeat the same connect, health and shutdown code and drift apart, and the
// drain order is the part that must not drift (plan.md §13.3).
//
// Files:
//
//	app.go   Spec, Worker, and the Run lifecycle
//	deps.go  backend connections, readiness checks and close order
package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/mservice/gitscanner/libs/buildinfo"
	"github.com/mservice/gitscanner/libs/config"
	"github.com/mservice/gitscanner/libs/lifecycle"
	"github.com/mservice/gitscanner/libs/observability"
)

// Worker is the long-running part of a service, typically a queue consumer.
//
// Run blocks until ctx is cancelled or the work is finished. Returning nil
// ends the process cleanly; returning an error ends it with a non-zero exit so
// Kubernetes restarts it.
type Worker interface {
	Run(ctx context.Context) error
}

// Drainer is implemented by workers that can stop intake before shutdown, such
// as a RabbitMQ consumer cancelling its subscription. Drain must return once
// in-flight work has finished or been handed back.
type Drainer interface {
	Drain(ctx context.Context) error
}

// WorkerFunc adapts a plain function to Worker.
type WorkerFunc func(ctx context.Context) error

// Run calls f(ctx).
func (f WorkerFunc) Run(ctx context.Context) error { return f(ctx) }

// Parts is what a service contributes on top of the shared bootstrap. Both
// fields are optional.
type Parts struct {
	// HTTP serves every path the probes do not, on the same port.
	HTTP http.Handler
	// Worker runs after the service reports ready.
	Worker Worker
}

// BuildFunc wires a service's own components from its dependencies. It runs
// after backends connect and before the service reports ready, so anything
// that must be loaded before taking work (the provider registry, plan.md
// §13.9) belongs here.
type BuildFunc func(ctx context.Context, d *Deps) (Parts, error)

// Spec declares a service.
type Spec struct {
	// Name is the service identity in logs, traces and metrics. It comes from
	// code, not the environment, so a misconfigured deployment cannot make two
	// workloads report the same identity.
	Name  string
	Needs Needs
	Build BuildFunc

	// OneShot runs the Worker once and exits without serving probes. Used for
	// jobs such as the migration hook.
	OneShot bool
}

// Main runs spec and exits the process with its result. Call it from main().
func Main(spec Spec) {
	if err := Run(spec); err != nil {
		// The logger may not exist yet when configuration fails, so this path
		// writes to stderr directly.
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

// Run executes the service lifecycle described in the package comment.
func Run(spec Spec) error {
	ctx, stop := lifecycle.SignalContext()
	defer stop()

	cfg, err := config.LoadCommon(spec.Name)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := observability.NewLogger(cfg.ServiceName, cfg.LogLevel)

	deps, err := connect(ctx, cfg, log, spec.Needs)
	if err != nil {
		return err
	}

	var parts Parts
	if spec.Build != nil {
		if parts, err = spec.Build(ctx, deps); err != nil {
			lifecycle.Shutdown(log, cfg.ShutdownTimeout, deps.closers()...)
			return fmt.Errorf("build %s: %w", spec.Name, err)
		}
	}

	var components []lifecycle.Component
	if !spec.OneShot {
		probes := observability.NewServer(cfg.HTTPAddr, log, deps.Health, parts.HTTP)
		if err := probes.Start(); err != nil {
			lifecycle.Shutdown(log, cfg.ShutdownTimeout, deps.closers()...)
			return err
		}
		components = append(components, lifecycle.Component{Name: "http", Stop: probes.Shutdown})
	}

	deps.Health.SetReady(true)
	log.Info("service started", "addr", cfg.HTTPAddr, "build", buildinfo.Get())

	// The worker gets its own context: on SIGTERM it is cancelled only after a
	// Drainer has had the chance to stop intake gracefully.
	workerCtx, cancelWorker := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelWorker()

	workerDone := make(chan error, 1)
	if parts.Worker != nil {
		go func() { workerDone <- parts.Worker.Run(workerCtx) }()
	}

	var runErr error
	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case runErr = <-workerDone:
		if runErr != nil {
			log.Error("worker stopped with error", "error", runErr)
		}
		workerDone = nil // already received; nothing left to wait for
	}

	if parts.Worker != nil && workerDone != nil {
		components = append(components, lifecycle.Component{
			Name: "worker",
			Stop: stopWorker(parts.Worker, cancelWorker, workerDone),
		})
	}
	components = append(components, deps.closers()...)

	if failed := lifecycle.Shutdown(log, cfg.ShutdownTimeout, components...); failed > 0 && runErr == nil {
		runErr = fmt.Errorf("%d component(s) did not shut down cleanly", failed)
	}
	return runErr
}

// stopWorker drains a worker if it supports it, then cancels it and waits for
// Run to return within the shutdown deadline.
func stopWorker(w Worker, cancel context.CancelFunc, done <-chan error) func(context.Context) error {
	return func(ctx context.Context) error {
		var drainErr error
		if d, ok := w.(Drainer); ok {
			drainErr = d.Drain(ctx)
		}
		cancel()

		select {
		case err := <-done:
			if errors.Is(err, context.Canceled) {
				err = nil
			}
			return errors.Join(drainErr, err)
		case <-ctx.Done():
			return errors.Join(drainErr, fmt.Errorf("worker did not stop: %w", ctx.Err()))
		}
	}
}
