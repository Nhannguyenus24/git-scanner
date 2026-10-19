// Command analytics consumes the finding stream into rollup tables (plan.md §10).
package main

import "github.com/mservice/gitscanner/internal/app"

func main() {
	app.Main(app.Spec{
		Name:  "analytics",
		Needs: app.Needs{Postgres: true, Kafka: true},
		// Phase 6 adds the worker here. Until then the process runs with no
		// worker so the chart, the probes and the drain path are exercised end
		// to end.
	})
}
