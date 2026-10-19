// Command enricher resolves raw matches against the provider registry (plan.md §5).
package main

import "github.com/mservice/gitscanner/internal/app"

func main() {
	app.Main(app.Spec{
		Name:  "enricher",
		Needs: app.Needs{Postgres: true, Rabbit: true, Redis: true, Kafka: true},
		// Phase 3 adds the worker here. Until then the process runs with no
		// worker so the chart, the probes and the drain path are exercised end
		// to end.
	})
}
