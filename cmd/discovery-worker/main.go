// Command discovery-worker expands a target into per-repository scan tasks
// (plan.md §2).
package main

import (
	"context"
	"fmt"

	"github.com/mservice/gitscanner/internal/app"
	"github.com/mservice/gitscanner/internal/discovery"
	"github.com/mservice/gitscanner/internal/dispatch"
	"github.com/mservice/gitscanner/internal/github"
	"github.com/mservice/gitscanner/libs/config"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// discoveryPrefetch is modest: each delivery lists one page and enqueues scans,
// which is cheap, but holding many in flight would blur the queue-depth signal
// KEDA scales on (plan.md §13.4).
const (
	discoveryPrefetch = 4
	discoveryMaxRetry = 5
)

func main() {
	app.Main(app.Spec{
		Name:  "discovery-worker",
		Needs: app.Needs{Postgres: true, Rabbit: true},
		Build: build,
	})
}

func build(_ context.Context, d *app.Deps) (app.Parts, error) {
	publisher, err := d.Rabbit.NewPublisher()
	if err != nil {
		return app.Parts{}, fmt.Errorf("discovery-worker: publisher: %w", err)
	}

	// GITHUB_TOKEN is optional but strongly advised: the unauthenticated rate
	// limit exhausts after a few hundred requests (plan.md §9).
	gh := github.New(config.GetOr("GITHUB_TOKEN", ""))
	disp := dispatch.New(publisher)
	worker := discovery.New(gh, d.DB, disp, d.DB, d.Log)

	consumer, err := d.Rabbit.NewConsumer(rabbit.QueueDiscovery, discoveryPrefetch, discoveryMaxRetry)
	if err != nil {
		return app.Parts{}, fmt.Errorf("discovery-worker: consumer: %w", err)
	}
	return app.Parts{Worker: app.NewConsumerWorker(consumer, worker.Handle)}, nil
}
