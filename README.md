# GitScanner

Multi-repository git secret scanner. Scans full git history for leaked
credentials, classifies them against a provider registry, and reports them.

Design and rationale: [plan.md](plan.md).

## Status

Scanning works end to end: a target is discovered, its repositories are cloned
and walked, credentials are detected, classified against the provider catalog,
rolled up for analytics, and can be disclosed by mail.

| Phase | | |
|---|---|---|
| 1 | repo scan to findings | done |
| 2 | bulk user/org discovery, pause and resume | done |
| 3 | enricher — registry resolution and the unmapped backlog | done |
| 5 | notifier — responsible disclosure under a shared rate cap | done |
| 6 | analytics — findings stream into rollups | done |
| 4, 7+ | provider registry API, ownership-verified probes, tuning | not started |

Detection currently runs on the built-in bootstrap ruleset
(`internal/scan/bootstrap_rules.go`). Phase 4 replaces it with the
registry-backed, hot-reloadable detector that the enricher already resolves
against.

## Layout

```
cmd/<service>/main.go   one per service — a declaration, ~15 lines, no logic
internal/
  app/                  the one bootstrap: config → connect → build → serve → drain
  api/                  REST handlers
  scan/                 scan engine and its Redis state (blob index, pause/cursor)
  store/                Postgres pool and migrations
libs/                   shared, importable conventions
  entity/               domain types + validation, stdlib only
  proto/                wire format for queue and stream messages (buf)
  rabbit/ redis/        transport conventions over the drivers
  config/               environment loading and validation
  observability/        logger and /healthz, /readyz
  lifecycle/            signal handling and ordered shutdown
  buildinfo/            version and commit stamped at link time
  id/                   UUIDv7
test/                   all tests, mirroring the source tree (test/README.md)
  unit/                 no backends — make test
  integration/          real backends — make test-integration
deploy/                 Helm chart and per-environment values
build/                  the single Dockerfile
```

Each package's `doc.go` or package comment lists its files and what each holds.

### Where to find things

| Looking for | Go to |
|---|---|
| What a service connects to and runs | `cmd/<service>/main.go` |
| Startup and shutdown order | `internal/app/app.go`, `internal/app/deps.go` |
| HTTP routes | `internal/api` |
| Queue names, retry and dead-letter topology | `libs/rabbit/conn.go` |
| Table definitions | `internal/store/migrations` |
| Message schemas | `libs/proto/proto/gitscanner/v1` |
| Per-service replicas, resources, scaling | `deploy/chart/values.yaml` |
| Tests for `libs/x` or `internal/x` | `test/unit/libs/x`, `test/unit/internal/x` |

### Dependency rules

Enforced by `depguard` in `make lint` — see [plan.md](plan.md) §3.2.

- `libs/` never imports `internal/` or `cmd/`.
- `libs/entity` imports the standard library only.
- `cmd/` never imports a database or broker driver; it declares a service and
  `internal/app` wires it.

Domain code depends on the narrowest interface it needs, defined where it is
used (for example `internal/scan` takes `goredis.Cmdable`, not a concrete
client), so it can be tested without the real backend.

### Adding a service

1. Create `cmd/<name>/main.go` calling `app.Main(app.Spec{...})` with the
   backends it needs and a `Build` function returning its `Worker` and/or
   HTTP handler. Put the logic itself in `internal/<domain>`.
2. Add a `services.<name>` block to `deploy/chart/values.yaml` (and
   `deploy/values/dev.yaml`).

The Makefile and Tiltfile discover services from `cmd/`, so nothing else
changes. If it shells out to `git`, add it to `GIT_SERVICES` in the Makefile
and `git_services` in the Tiltfile.

## Getting started

```bash
make bootstrap     # buf, sqlc, golangci-lint (Go tools)
brew install k3d helm tilt-dev/tap/tilt

make check         # vet, lint, test
make build         # binaries into ./bin
make help          # every target
```

### Running against local infrastructure

Postgres, RabbitMQ and Redis are expected to be running already (the values are
in [spec.md](spec.md)). To run a service directly:

```bash
export POSTGRES_URL='postgresql://postgres:postgres123@localhost:5432/app_db'
export RABBITMQ_URL='amqp://admin:admin123@localhost:5672/'
export REDIS_URL='redis://:redis123@localhost:6379/0'

go run ./cmd/api
curl localhost:8080/readyz
```

The `api` binary applies migrations on startup. In Kubernetes a Helm
`pre-upgrade` hook does it first and the startup call becomes a no-op.

### Local Kubernetes

```bash
make cluster       # k3d + local registry + KEDA + namespace
make up            # tilt
```

`make cluster` joins the cluster to a Docker network so pods reach the existing
infrastructure containers by name. Set `INFRA_NET` to the right network, or
leave the dev values pointing at `host.k3d.internal`. Details in
[plan.md](plan.md) §14.1.

## Security notes

Three things in this codebase are load-bearing and should not be "simplified":

- **Secrets are never stored in plaintext.** `findings` holds an HMAC
  fingerprint and a masked preview only (§6).
- **Discovered credentials are never used to authenticate.** Risk is derived
  offline; live probes run only on ownership-verified scans (§0.2).
- **Bulk scanning targets authorized sets only** — an org or an explicit
  allowlist, never all of public GitHub (§0.1).
