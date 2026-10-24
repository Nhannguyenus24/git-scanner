---
name: architecture
description: How GitScanner fits together at runtime — what each of the six services does, how work flows between them over RabbitMQ and Kafka, which store owns what state, and the contracts a structural change has to respect. Use this when adding or changing a service, moving logic between packages, touching queue or event plumbing, changing the schema, or when asked how the system works, before reading code at random.
---

# How GitScanner fits together

[README.md](../../../README.md) says where files live and lists the dependency
rules; [plan.md](../../../plan.md) carries the design rationale in numbered
sections worth citing. This skill is the part neither gives directly: the
runtime picture, and the contracts to respect when changing it.

## The flow of one scan

```
POST /v1/scans/user ──► api
                         │  creates target + scan_job (Postgres)
                         ▼
                   discovery.tasks ──► discovery-worker
                                        │  one GitHub page per delivery,
                                        │  republishes itself with the next cursor
                                        ▼
                                   scan.tasks ──► scan-worker
                                                   │  git clone --bare, walk unique blobs,
                                                   │  detect, store finding
                                                   ├──► findings.v1 (Kafka) ──► analytics
                                                   │                             rollup tables
                                                   └──► enrich.tasks ──► enricher
                                                                          provider catalog,
                                                                          or candidate backlog

POST /v1/repositories/{id}/disclose ──► notify.tasks ──► notifier ──► one mail per repository
```

Two deliberate shapes in that picture:

**Discovery paginates by republishing itself.** One page per delivery, with the
cursor advanced, rather than one long-running walk. That keeps each delivery
short so prefetch backpressure means something, and makes pause simply a matter
of not republishing (plan.md §8).

**RabbitMQ carries work, Kafka carries facts.** A task is addressed to one
consumer and may be retried or dead-lettered; a finding event is a durable log
that analytics replays. They are not interchangeable — a Kafka consumer that
cannot process a record must stop and be fixed, because the rollups depend on
seeing every record once and in order, which is why that package has no retry or
DLQ machinery (plan.md §1).

## Every service is the same shape

`cmd/<name>/main.go` is a declaration of about fifteen lines; `internal/app`
does the rest in a fixed order: load config → connect backends → `Build` →
serve probes → ready → run → unready → drain → close.

```go
app.Main(app.Spec{
    Name:  "scan-worker",
    Needs: app.Needs{Postgres: true, Rabbit: true, Redis: true, Kafka: true},
    Build: build,
})

func build(_ context.Context, d *app.Deps) (app.Parts, error) {
    consumer, err := d.Rabbit.NewConsumer(rabbit.QueueScan, prefetch, maxRetry)
    ...
    return app.Parts{Worker: app.NewConsumerWorker(consumer, worker.Handle)}, nil
}
```

`Needs` decides which backends are dialled and which readiness checks are
registered — a service does not connect to a broker it never uses, so an outage
there cannot take it down. `Build` runs after the backends are up and before the
service reports ready, which is where anything that must be loaded before taking
work belongs.

`Parts` takes an HTTP handler, a `Worker`, or both. Two adapters cover the
existing services: `app.NewConsumerWorker` for a RabbitMQ queue and
`app.NewKafkaConsumerWorker` for a consumer group. Both implement `Drainer`, so
shutdown stops intake before cancelling work — the drain order is the part that
must not drift, and it lives in one place for that reason.

## Where state lives

| Store | Holds | Why there |
|---|---|---|
| Postgres | targets, jobs, repositories, findings, provider catalog, rollups | the system of record; anything that must survive and be queried |
| RabbitMQ | per-task dispatch, with retry and dead-letter | addressed work, bounded by prefetch so queue depth is a real scaling signal |
| Kafka | the findings event log | durable, replayable, partitioned by repository so a repo's events stay ordered |
| Redis | pause flags, the shared mail rate cap, blob index | small, hot, shared across replicas |

The Redis cap matters architecturally: a per-process limiter would multiply the
send rate by the replica count, which is precisely the failure the cap exists to
prevent (plan.md §13.5).

## Package boundaries

The dependency rules are in the README and enforced by `depguard`. The intent
behind them:

- **`libs/entity` is stdlib-only.** Domain types and their validation must not
  drag in a driver or a wire format. `libs/proto` is separate on purpose so the
  schema and the wire format can evolve independently (plan.md §3.3).
- **`internal/store` owns Postgres.** It lives in `internal/` rather than
  `libs/` because the schema is private to this system. Nothing outside it
  should know a table name, and it stays free of proto imports — mapping an
  event to a domain value is the worker's job.
- **`internal/dispatch` owns queue messages.** It is the single place that knows
  a task's wire shape and which queue it belongs on, so handlers and the API
  never build raw messages.
- **`internal/scan` is a pure library.** The engine clones, walks and detects; it
  has no queue, database or Kafka dependency. All that wiring is
  `internal/scanworker`. Keeping the split is what makes the engine testable
  without a broker.
- **Domain code depends on the narrowest interface it needs, declared where it
  is used.** `internal/discovery` defines its own `Store` and `Dispatcher`
  interfaces rather than importing the concrete types.

## Idempotency is the backbone

Everything here is at-least-once, so every write has to tolerate a replay. Each
mechanism is different and they are not interchangeable:

| Where | Mechanism |
|---|---|
| findings | unique `(fingerprint, repository_id)`; a redelivered scan re-inserts and reports unchanged |
| analytics | a batch claims finding ids in a dedupe table and counts only the ids it won |
| notifications | a partial unique index on `(repository_id)` for pending and sent |
| scan state | `SetScanState` is a no-op when the state is unchanged, and rejects illegal transitions |

The findings rule has one deliberate exception — a *named* rule may re-attribute
a finding the generic pass stored unattributed — and the guard is narrow for a
reason. See CLAUDE.md's invariants section before touching it.

## Adding a service

The mechanical steps are in the README. What to get right beyond them: pick
`Needs` honestly, decide which store owns the new state, and put the logic in
`internal/<domain>` with the wiring in a worker package, so the domain code
stays testable. If it shells out to `git`, it needs the runtime-git image —
`GIT_SERVICES` in the Makefile and `git_services` in the Tiltfile.

## Where to read further

| Topic | plan.md |
|---|---|
| Scope limits: bounded targets, no live auth, no payment link | §0 |
| Service responsibilities | §2 |
| Layout, dependency rules, entity vs proto | §3 |
| Scan pipeline, concurrency, classification | §4 |
| Provider registry, unmapped backlog, reclassify vs rescan | §5 |
| Secret storage: fingerprint and masking | §6 |
| Schema | §7 |
| API and pause/resume | §8 |
| Analytics rollups | §10 |
| scan-worker under Kubernetes, drain, autoscaling, mail cap | §13 |
