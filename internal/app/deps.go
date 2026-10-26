package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mservice/gitscanner/internal/store"
	"github.com/mservice/gitscanner/libs/config"
	"github.com/mservice/gitscanner/libs/lifecycle"
	"github.com/mservice/gitscanner/libs/observability"
	"github.com/mservice/gitscanner/libs/rabbit"
)

// Needs lists which backends a service uses. Connecting to everything
// regardless would make a broker outage take down services that never use it.
type Needs struct {
	Postgres bool
	Rabbit   bool
}

// validate reports every missing connection setting at once, before anything
// is dialled, so a misconfigured pod fails with the whole list in one log line.
func (n Needs) validate(cfg config.Common) error {
	var errs []error
	if n.Postgres && cfg.PostgresURL == "" {
		errs = append(errs, errors.New("POSTGRES_URL is required"))
	}
	if n.Rabbit && cfg.RabbitURL == "" {
		errs = append(errs, errors.New("RABBITMQ_URL is required"))
	}
	return errors.Join(errs...)
}

// Deps is everything a service's Build function may use. Fields for backends
// the service did not ask for in Needs are nil.
type Deps struct {
	Cfg    config.Common
	Log    *slog.Logger
	Health *observability.Health

	DB     *store.DB
	Rabbit *rabbit.Conn
}

// connect dials every backend in needs and registers its readiness check. On
// failure it closes whatever it already opened, so a half-started process does
// not leak connections while it exits.
func connect(ctx context.Context, cfg config.Common, log *slog.Logger, needs Needs) (*Deps, error) {
	if err := needs.validate(cfg); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	d := &Deps{Cfg: cfg, Log: log, Health: observability.NewHealth()}

	fail := func(err error) (*Deps, error) {
		lifecycle.Shutdown(log, cfg.ShutdownTimeout, d.closers()...)
		return nil, err
	}

	if needs.Postgres {
		db, err := store.Open(ctx, cfg.PostgresURL)
		if err != nil {
			return fail(err)
		}
		d.DB = db
		d.Health.AddCheck("postgres", db.Ping)
	}

	if needs.Rabbit {
		rc, err := rabbit.Connect(ctx, cfg.RabbitURL, log)
		if err != nil {
			return fail(err)
		}
		d.Rabbit = rc
		d.Health.AddCheck("rabbit", rc.Healthy)
	}

	return d, nil
}

// closers returns backend shutdown steps in reverse dependency order.
//
// Order is deliberate and is the reason this lives in one place: the broker
// closes first, then the stores that an in-flight handler writes to. Closing
// Postgres while a delivery is still being acked would lose the cursor the
// drain is trying to persist (plan.md §13.3).
func (d *Deps) closers() []lifecycle.Component {
	var cs []lifecycle.Component
	if d.Rabbit != nil {
		cs = append(cs, lifecycle.Component{Name: "rabbit", Stop: ignoreCtx(d.Rabbit.Close)})
	}
	if d.DB != nil {
		cs = append(cs, lifecycle.Component{Name: "postgres", Stop: ignoreCtx(d.DB.Close)})
	}
	return cs
}

func ignoreCtx(fn func() error) func(context.Context) error {
	return func(context.Context) error { return fn() }
}
