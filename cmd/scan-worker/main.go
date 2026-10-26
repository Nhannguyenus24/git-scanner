// Command scan-worker clones a repository and walks its history for secrets
// (plan.md §4).
package main

import (
	"context"
	"fmt"

	"net/http"
	"time"

	"github.com/mservice/gitscanner/internal/app"
	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/github"
	"github.com/mservice/gitscanner/internal/probe"
	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/scanworker"
	"github.com/mservice/gitscanner/internal/secrets"
	"github.com/mservice/gitscanner/libs/config"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// scanPrefetch bounds how many repositories one worker holds at once. It is the
// backpressure that lets KEDA scale on queue depth (plan.md §13.4); a repo scan
// is heavy, so this is deliberately small.
const (
	scanPrefetch = 2
	scanMaxRetry = 3
)

func main() {
	app.Main(app.Spec{
		Name:  "scan-worker",
		Needs: app.Needs{Postgres: true, Rabbit: true, Redis: true},
		Build: build,
	})
}

func build(_ context.Context, d *app.Deps) (app.Parts, error) {
	hasher, err := secrets.NewHasher(d.Cfg.FingerprintKey)
	if err != nil {
		return app.Parts{}, fmt.Errorf("scan-worker: %w", err)
	}

	// Phase 1 scans with the built-in bootstrap ruleset. Phase 3 replaces this
	// with a registry-backed, hot-reloadable detector.
	det := scan.NewDetector(scan.BootstrapRuleset())
	engine := scan.NewEngine(scan.NewGitCLI(d.Cfg.ScratchDir), det, hasher)

	// Live credential validation is off unless explicitly enabled, and must only
	// be enabled for targets the operator owns: it authenticates to providers
	// with the credentials it finds (plan.md ownership gate).
	if d.Cfg.ProbeSecrets {
		engine = engine.WithProber(probe.New(&http.Client{Timeout: 10 * time.Second}))
		d.Log.Warn("live credential probing is ON — only valid for targets you own")
	}

	publisher, err := d.Rabbit.NewPublisher()
	if err != nil {
		return app.Parts{}, fmt.Errorf("scan-worker: publisher: %w", err)
	}

	// Host attributes (language, owner type, last push) for repositories that
	// arrived without them. GITHUB_TOKEN is optional, as for discovery; without
	// it the reads stop at GitHub's unauthenticated limit and resume later.
	worker := scanworker.New(engine, d.DB, dispatch.New(publisher), d.Log).
		WithRepoAttributes(github.New(config.GetOr("GITHUB_TOKEN", "")))

	consumer, err := d.Rabbit.NewConsumer(rabbit.QueueScan, scanPrefetch, scanMaxRetry)
	if err != nil {
		return app.Parts{}, fmt.Errorf("scan-worker: consumer: %w", err)
	}
	d.Log.Info("scan-worker ready", "ruleset_version", det.Ruleset().Version())

	return app.Parts{Worker: app.NewConsumerWorker(consumer, worker.Handle)}, nil
}
