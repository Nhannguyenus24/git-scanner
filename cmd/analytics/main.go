// Command analytics consumes the finding stream into rollup tables (plan.md §10).
package main

import (
	"context"
	"fmt"

	"github.com/mservice/gitscanner/internal/analytics"
	"github.com/mservice/gitscanner/internal/app"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// Folding one event is two small statements inside a transaction, so this
// worker can hold more in flight than a scan ever could. A failure is a
// database problem rather than a bad message, and those usually clear, so the
// retry budget is generous before a message is dead-lettered.
const (
	analyticsPrefetch = 16
	analyticsMaxRetry = 5
)

func main() {
	app.Main(app.Spec{
		Name:  "analytics",
		Needs: app.Needs{Postgres: true, Rabbit: true},
		Build: build,
	})
}

func build(_ context.Context, d *app.Deps) (app.Parts, error) {
	consumer, err := d.Rabbit.NewConsumer(rabbit.QueueFindings, analyticsPrefetch, analyticsMaxRetry)
	if err != nil {
		return app.Parts{}, fmt.Errorf("analytics: consumer: %w", err)
	}

	worker := analytics.New(d.DB, d.Log)
	return app.Parts{Worker: app.NewConsumerWorker(consumer, worker.Handle)}, nil
}
