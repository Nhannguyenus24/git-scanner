// Command notifier sends responsible-disclosure mail under a shared rate cap (plan.md §0.3).
package main

import "github.com/mservice/gitscanner/internal/app"

func main() {
	app.Main(app.Spec{
		Name:  "notifier",
		Needs: app.Needs{Postgres: true, Rabbit: true, Redis: true},
		// Phase 5 adds the worker here. Until then the process runs with no
		// worker so the chart, the probes and the drain path are exercised end
		// to end.
	})
}
