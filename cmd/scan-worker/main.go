// Command scan-worker clones a repository and walks its history for secrets
// (plan.md §4).
package main

import (
	"context"
	"fmt"

	"github.com/mservice/gitscanner/internal/app"
	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/scan"
	"github.com/mservice/gitscanner/internal/scanworker"
	"github.com/mservice/gitscanner/internal/secrets"
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

	publisher, err := d.Rabbit.NewPublisher()
	if err != nil {
		return app.Parts{}, fmt.Errorf("scan-worker: publisher: %w", err)
	}

	worker := scanworker.New(engine, d.DB, dispatch.New(publisher), d.Log)

	consumer, err := d.Rabbit.NewConsumer(rabbit.QueueScan, scanPrefetch, scanMaxRetry)
	if err != nil {
		return app.Parts{}, fmt.Errorf("scan-worker: consumer: %w", err)
	}
	d.Log.Info("scan-worker ready", "ruleset_version", det.Ruleset().Version())

	return app.Parts{Worker: app.NewConsumerWorker(consumer, worker.Handle)}, nil
}
