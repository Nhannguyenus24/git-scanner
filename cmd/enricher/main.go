// Command enricher resolves raw matches against the provider registry (plan.md §5).
package main

import (
	"context"
	"fmt"

	"github.com/mservice/gitscanner/internal/app"
	"github.com/mservice/gitscanner/internal/enrich"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// Enrichment is a couple of indexed queries per delivery, with no clone and no
// network call, so a worker can hold far more in flight than the scan worker's
// two without the queue depth KEDA scales on losing its meaning (plan.md §13.4).
const (
	enrichPrefetch = 8
	enrichMaxRetry = 5
)

func main() {
	app.Main(app.Spec{
		Name:  "enricher",
		Needs: app.Needs{Postgres: true, Rabbit: true, Redis: true},
		Build: build,
	})
}

func build(_ context.Context, d *app.Deps) (app.Parts, error) {
	worker := enrich.New(d.DB, d.Log)

	consumer, err := d.Rabbit.NewConsumer(rabbit.QueueEnrich, enrichPrefetch, enrichMaxRetry)
	if err != nil {
		return app.Parts{}, fmt.Errorf("enricher: consumer: %w", err)
	}
	return app.Parts{Worker: app.NewConsumerWorker(consumer, worker.Handle)}, nil
}
