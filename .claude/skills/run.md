# Run — start GitScanner locally

This skill covers how to build, start, and stop all six GitScanner services
without Kubernetes (k3d / Tilt are not required).

## Prerequisites

Four infrastructure containers must be running:

| Container | Port | Credentials |
|-----------|------|-------------|
| postgres  | 5432 | `postgres` / `postgres123` / db `app_db` |
| rabbitmq  | 5672 | `admin` / `admin123` |
| redis     | 6379 | password `redis123` |
| kafka     | 9092 | no auth |

Check with `docker ps`. If any are missing, start them first.

## Start everything

```bash
make run-local
```

This builds all binaries into `./bin/`, then starts each service in the
background, writing stdout/stderr to `/tmp/gitscanner-logs/<service>.log`.
The `api` service starts first (it applies DB migrations) with a 2-second
head start before the workers begin.

Services and their ports:

| Service          | Port |
|------------------|------|
| api              | 8080 |
| discovery-worker | 8081 |
| scan-worker      | 8082 |
| enricher         | 8083 |
| notifier         | 8084 |
| analytics        | 8085 |

Each service exposes `/healthz` (liveness) and `/readyz` (readiness) on its
port.

## Override infrastructure URLs

```bash
make run-local \
  INFRA_POSTGRES='postgresql://user:pass@host:5432/db' \
  INFRA_RABBIT='amqp://user:pass@host:5672/' \
  INFRA_REDIS='redis://:pass@host:6379/0' \
  INFRA_KAFKA='host:9092' \
  RUN_LOG_DIR=/your/log/path
```

## Stop everything

```bash
make stop-local
```

Sends SIGTERM to every PID saved during `run-local` and removes the `.pid`
files.

## Tail logs

```bash
tail -f /tmp/gitscanner-logs/api.log
tail -f /tmp/gitscanner-logs/scan-worker.log
```

## Notes

- The `api` binary applies database migrations on startup; subsequent starts
  are no-ops.
- In Kubernetes the same flow runs via `make cluster` + `make up` (Tilt).
- Integration credentials come from the Docker containers inspected during
  session setup — do not hardcode them elsewhere.
