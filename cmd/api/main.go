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
	"github.com/mservice/gitscanner/internal/github"
	"github.com/mservice/gitscanner/internal/notify"
	"github.com/mservice/gitscanner/internal/progress"
	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/secrets"
	"github.com/mservice/gitscanner/libs/config"
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
	Needs: app.Needs{Postgres: true, Rabbit: true},
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
		sealer, err := secrets.OptionalSealer(config.GetOr("SECRET_SEAL_KEY", ""))
		if err != nil {
			return app.Parts{}, fmt.Errorf("api: %w", err)
		}
		// The providers list is derived from the same bootstrap ruleset the
		// scan-worker runs, so the dashboard shows exactly what the engine detects.
		srv := api.NewServer(d.DB, queue, d.DB, d.Log).
			WithCatalog(d.DB).
			WithSealer(sealer).
			WithOwnerLookup(github.New(config.GetOr("GITHUB_TOKEN", ""))).
			WithNoticeLog(d.DB, notify.DisclosureInfo{
				Project:   config.GetOr("DISCLOSURE_PROJECT", ""),
				Signature: config.GetOr("DISCLOSURE_SIGNATURE", ""),
				InfoURL:   config.GetOr("DISCLOSURE_INFO_URL", ""),
			}).
			WithProviders(scan.BootstrapRuleset().Providers())

		// The hub listens for the whole life of the process, on the build
		// context rather than as the service's Worker: that context ends on
		// SIGTERM, before the HTTP server drains, so open event streams close
		// and do not hold the drain until its deadline.
		hub := progress.NewHub(d.Log)
		go hub.Run(ctx, d.DB)
		srv.WithProgress(hub, progress.NewPublisher(d.DB, d.Log))
		srv.WithShares(d.DB)
		srv.WithInsights(d.DB)

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
