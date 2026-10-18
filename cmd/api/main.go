// Command api serves the REST surface (plan.md §8) and the registry endpoints
// (§5.5).
package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/mservice/gitscanner/internal/service"
	"github.com/mservice/gitscanner/libs/runtime"
)

func main() {
	// The Helm pre-upgrade hook runs this same image with -migrate-only
	// (plan.md §13.6), which avoids shipping a second image whose only job is to
	// hold a copy of the migrations that could drift from the application's.
	migrateOnly := flag.Bool("migrate-only", false,
		"apply database migrations and exit")
	flag.Parse()

	if err := run(*migrateOnly); err != nil {
		// The logger may not exist yet when configuration fails, so this path
		// writes to stderr directly.
		println("fatal:", err.Error())
		os.Exit(1)
	}
}

func run(migrateOnly bool) error {
	ctx, stop := runtime.SignalContext()
	defer stop()

	deps := service.Deps{Postgres: true, Rabbit: true, Redis: true}
	if migrateOnly {
		deps = service.Deps{Postgres: true}
	}

	svc, err := service.New(ctx, "api", deps)
	if err != nil {
		return err
	}

	if err := svc.DB.Migrate(ctx); err != nil {
		return err
	}
	if migrateOnly {
		svc.Log.Info("migrations applied")
		runtime.Shutdown(svc.Log, svc.Cfg.ShutdownTimeout, svc.Close()...)
		return nil
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"service":"api","phase":"0"}`))
	})

	svc.StartProbes(mux)
	svc.Ready()
	svc.Log.Info("api started", "addr", svc.Cfg.HTTPAddr)

	<-ctx.Done()
	svc.Log.Info("shutdown signal received")

	runtime.Shutdown(svc.Log, svc.Cfg.ShutdownTimeout, svc.Close()...)
	return nil
}
