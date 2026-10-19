// Command api serves the REST surface (plan.md §8) and the registry endpoints
// (§5.5).
package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/mservice/gitscanner/internal/api"
	"github.com/mservice/gitscanner/internal/app"
	"github.com/mservice/gitscanner/internal/dispatch"
)

func main() {
	// The Helm pre-upgrade hook runs this same image with -migrate-only
	// (plan.md §13.6), which avoids shipping a second image whose only job is to
	// hold a copy of the migrations that could drift from the application's.
	migrateOnly := flag.Bool("migrate-only", false, "apply database migrations and exit")
	flag.Parse()

	if *migrateOnly {
		app.Main(migrateSpec)
		return
	}
	app.Main(serveSpec)
}

var serveSpec = app.Spec{
	Name:  "api",
	Needs: app.Needs{Postgres: true, Rabbit: true, Redis: true},
	Build: func(ctx context.Context, d *app.Deps) (app.Parts, error) {
		// In Kubernetes the hook has already migrated and this is a no-op under
		// the advisory lock; running directly, it saves a separate step.
		if err := d.DB.Migrate(ctx); err != nil {
			return app.Parts{}, err
		}

		publisher, err := d.Rabbit.NewPublisher()
		if err != nil {
			return app.Parts{}, fmt.Errorf("api: publisher: %w", err)
		}
		queue := dispatch.New(publisher)
		srv := api.NewServer(d.DB, queue, d.Log)

		return app.Parts{HTTP: srv.Routes()}, nil
	},
}

var migrateSpec = app.Spec{
	Name:    "api",
	Needs:   app.Needs{Postgres: true},
	OneShot: true,
	Build: func(_ context.Context, d *app.Deps) (app.Parts, error) {
		return app.Parts{Worker: app.WorkerFunc(func(ctx context.Context) error {
			if err := d.DB.Migrate(ctx); err != nil {
				return err
			}
			d.Log.Info("migrations applied")
			return nil
		})}, nil
	},
}
