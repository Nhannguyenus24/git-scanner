# GitScanner

Multi-repository git secret scanner. Scans full git history for leaked
credentials, classifies them against a provider registry, and reports them.

Design and rationale: [plan.md](plan.md).

## Status

Phase 0 — scaffolding. All six services build, connect to their backends,
serve probes and drain cleanly. No scanning yet; that is Phase 1.

## Layout

```
cmd/        six service binaries
libs/       shared libraries (entity, proto, kafka, rabbit, redis, …)
internal/   service-private code
deploy/     Helm chart and per-environment values
build/      Dockerfile
```

`libs/` must not import `internal/`, and `libs/entity` is standard library only.
Both rules are enforced by `depguard` in CI — see [plan.md](plan.md) §3.2.

## Getting started

```bash
make bootstrap     # buf, sqlc, golangci-lint (Go tools)
brew install k3d helm tilt-dev/tap/tilt

make check         # vet, lint, test
make build         # binaries into ./bin
```

### Running against local infrastructure

Postgres, RabbitMQ, Redis and Kafka are expected to be running already (the
values are in [spec.md](spec.md)). To run a service directly:

```bash
export POSTGRES_URL='postgresql://postgres:postgres123@localhost:5432/app_db'
export RABBITMQ_URL='amqp://admin:admin123@localhost:5672/'
export REDIS_URL='redis://:redis123@localhost:6379/0'
export KAFKA_BOOTSTRAP_SERVERS='localhost:9092'

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
