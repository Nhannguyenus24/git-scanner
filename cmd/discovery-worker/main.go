// Command discovery-worker expands a target into per-repository scan tasks (plan.md §2).
package main

import "github.com/mservice/gitscanner/internal/app"

func main() {
	app.Main(app.Spec{
		Name:  "discovery-worker",
		Needs: app.Needs{Postgres: true, Rabbit: true, Redis: true, Kafka: true},
		// Phase 2 adds the worker here. Until then the process runs with no
		// worker so the chart, the probes and the drain path are exercised end
		// to end.
	})
}
