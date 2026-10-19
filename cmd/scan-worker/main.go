// Command scan-worker clones a repository and walks its history for secrets (plan.md §4).
package main

import "github.com/mservice/gitscanner/internal/app"

func main() {
	app.Main(app.Spec{
		Name:  "scan-worker",
		Needs: app.Needs{Postgres: true, Rabbit: true, Redis: true, Kafka: true},
		// Phase 1 adds the worker here. Until then the process runs with no
		// worker so the chart, the probes and the drain path are exercised end
		// to end.
	})
}
