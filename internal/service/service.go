// Package service assembles the dependencies every binary needs.
//
// Six main() functions would otherwise repeat the same forty lines of connect,
// health-check registration and ordered shutdown, and would drift apart. The
// drain order in Close is the part that must not drift (plan.md §13.3).
package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/config"
	"github.com/mservice/gitscanner/libs/kafka"
	"github.com/mservice/gitscanner/libs/observability"
	"github.com/mservice/gitscanner/libs/rabbit"
	"github.com/mservice/gitscanner/libs/redis"
	"github.com/mservice/gitscanner/libs/runtime"
)

// Deps lists which backends a binary needs. The api does not consume RabbitMQ;
// the analytics consumer does not publish to it. Connecting to everything
// regardless would make a broker outage take down services that never use it.
type Deps struct {
	Postgres bool
	Rabbit   bool
	Redis    bool
	Kafka    bool
}

// Service is a wired-up process.
type Service struct {
	Cfg    config.Common
	Log    *slog.Logger
	Health *observability.Health

	DB     *store.DB
	Rabbit *rabbit.Conn
	Redis  *redis.Client
	Kafka  *kafka.Producer

	probes *observability.Server
}

// New connects everything Deps asks for and registers readiness checks.
func New(ctx context.Context, name string, deps Deps) (*Service, error) {
	cfg, err := config.LoadCommon(name)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	log := observability.NewLogger(cfg.ServiceName, cfg.LogLevel)
	health := observability.NewHealth()

	s := &Service{Cfg: cfg, Log: log, Health: health}

	if deps.Postgres {
		db, err := store.Open(ctx, cfg.PostgresURL)
		if err != nil {
			return nil, err
		}
		s.DB = db
		health.AddCheck("postgres", db.Ping)
	}

	if deps.Redis {
		if cfg.RedisURL == "" {
			return nil, fmt.Errorf("REDIS_URL is required by %s", name)
		}
		rc, err := redis.New(ctx, cfg.RedisURL)
		if err != nil {
			return nil, err
		}
		s.Redis = rc
		health.AddCheck("redis", rc.Ping)
	}

	if deps.Rabbit {
		if cfg.RabbitURL == "" {
			return nil, fmt.Errorf("RABBITMQ_URL is required by %s", name)
		}
		rc, err := rabbit.Connect(ctx, cfg.RabbitURL, log)
		if err != nil {
			return nil, err
		}
		s.Rabbit = rc
		health.AddCheck("rabbit", rc.Healthy)
	}

	if deps.Kafka {
		if len(cfg.KafkaBroker) == 0 {
			return nil, fmt.Errorf("KAFKA_BOOTSTRAP_SERVERS is required by %s", name)
		}
		kp, err := kafka.NewProducer(cfg.KafkaBroker, log)
		if err != nil {
			return nil, err
		}
		s.Kafka = kp
		health.AddCheck("kafka", kp.Ping)
	}

	return s, nil
}

// StartProbes begins serving /healthz and /readyz, plus extra when a service
// mounts its own routes on the same port.
func (s *Service) StartProbes(extra http.Handler) {
	s.probes = observability.NewServer(s.Cfg.HTTPAddr, s.Log, s.Health, extra)
	s.probes.Start()
}

// Ready marks the service as having finished startup.
func (s *Service) Ready() { s.Health.SetReady(true) }

// Close shuts dependencies down in reverse dependency order.
//
// Order is deliberate and is the reason this lives in one place: message intake
// stops first, then the event producer flushes, then the stores that flushing
// writes to are closed. Closing Postgres before the Kafka flush would lose the
// cursor the drain is trying to persist.
func (s *Service) Close() []runtime.Component {
	var components []runtime.Component

	if s.probes != nil {
		components = append(components, runtime.Component{
			Name: "probes", Stop: s.probes.Shutdown,
		})
	}
	if s.Kafka != nil {
		components = append(components, runtime.Component{
			Name: "kafka", Stop: s.Kafka.Close,
		})
	}
	if s.Rabbit != nil {
		components = append(components, runtime.Component{
			Name: "rabbit",
			Stop: func(context.Context) error { return s.Rabbit.Close() },
		})
	}
	if s.Redis != nil {
		components = append(components, runtime.Component{
			Name: "redis",
			Stop: func(context.Context) error { return s.Redis.Close() },
		})
	}
	if s.DB != nil {
		components = append(components, runtime.Component{
			Name: "postgres",
			Stop: func(context.Context) error { return s.DB.Close() },
		})
	}

	return components
}
