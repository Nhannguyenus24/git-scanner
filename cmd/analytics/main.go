// Command analytics consumes the finding stream into rollup tables (plan.md §10).
package main

import (
	"context"
	"fmt"

	"github.com/mservice/gitscanner/internal/analytics"
	"github.com/mservice/gitscanner/internal/app"
	"github.com/mservice/gitscanner/libs/kafka"
)

// consumerGroup is this service's identity to Kafka, and the group name is what
// decides which committed offsets it resumes from. It is a constant rather than
// configuration because a deploy that changed it would silently re-consume the
// whole topic and double every counter the dedupe tables did not already hold.
const consumerGroup = "analytics"

func main() {
	app.Main(app.Spec{
		Name: "analytics",
		// Needs.Kafka also opens a producer this service never publishes with.
		// It stays because it is what validates KAFKA_BOOTSTRAP_SERVERS at
		// startup and registers the broker readiness check.
		Needs: app.Needs{Postgres: true, Kafka: true},
		Build: build,
	})
}

func build(_ context.Context, d *app.Deps) (app.Parts, error) {
	consumer, err := kafka.NewConsumer(d.Cfg.KafkaBroker, consumerGroup,
		[]string{kafka.TopicFindings}, d.Log)
	if err != nil {
		return app.Parts{}, fmt.Errorf("analytics: consumer: %w", err)
	}
	d.Health.AddCheck("kafka-consumer", consumer.Ping)

	worker := analytics.New(d.DB, d.Log)
	return app.Parts{Worker: app.NewKafkaConsumerWorker(consumer, worker.Handle)}, nil
}
