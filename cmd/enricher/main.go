// Command enricher resolves raw matches against the provider registry (plan.md §5).
package main

import (
	"os"

	"github.com/mservice/gitscanner/internal/service"
	"github.com/mservice/gitscanner/libs/runtime"
)

func main() {
	if err := run(); err != nil {
		println("fatal:", err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := runtime.SignalContext()
	defer stop()

	svc, err := service.New(ctx, "enricher", service.Deps{
		Postgres: true,
		Rabbit:   true,
		Redis:    true,
		Kafka:    true,
	})
	if err != nil {
		return err
	}

	svc.StartProbes(nil)
	svc.Ready()
	svc.Log.Info("enricher started", "addr", svc.Cfg.HTTPAddr)

	// Phase 3 replaces this with the consumer loop. Until then the process
	// exists so the chart, the probes and the drain path are exercised end to end.
	<-ctx.Done()
	svc.Log.Info("shutdown signal received")

	runtime.Shutdown(svc.Log, svc.Cfg.ShutdownTimeout, svc.Close()...)
	return nil
}
