---
name: run
description: Build, start, stop and observe the six GitScanner services locally without Kubernetes. Use this whenever the user wants to run the app, restart it, check whether it is up, read service logs, or submit a test scan — including phrasings like "chạy thử", "restart đi", "check log", "is it running", or when any other task needs a live stack to verify against.
---

# Running GitScanner locally

Six Go services plus a React frontend, driven by `make`. k3d and Tilt are the
Kubernetes path and are not needed here.

## Infrastructure is already on the machine

Postgres and RabbitMQ run as pre-existing containers that this
repository does not own. **There is deliberately no docker-compose file here —
do not add one.** An earlier attempt was removed precisely because it duplicated
infrastructure the developer already runs.

| Backend  | Port | Credentials (the Makefile defaults) |
|----------|------|-------------------------------------|
| postgres | 5432 | `postgres` / `postgres123`, db `app_db` |
| rabbitmq | 5672 | `admin` / `admin123` (UI on 15672) |

Confirm they are up before blaming the services:

```bash
pg_isready -h localhost -p 5432
curl -sf http://localhost:15672 >/dev/null && echo "rabbit ok"
docker ps --format '{{.Names}}'
```

## Start

```bash
make run-local
```

Builds every binary into `./bin/`, starts each service in the background, and
writes stdout and stderr to `/tmp/gitscanner-logs/<service>.log`. The api starts
first and is given two seconds to apply migrations, then the workers.

| Service | Port | Service | Port |
|---|---|---|---|
| api | 8080 | enricher | 8083 |
| discovery-worker | 8081 | notifier | 8084 |
| scan-worker | 8082 | analytics | 8085 |

Each serves `/healthz` and `/readyz`.

## Restarting: free the ports first

`make stop-local` only signals the PIDs it recorded. A service that was killed
or crashed outside that bookkeeping keeps its port, and the next `run-local`
reports it as DEAD with `bind: address already in use` buried in its log. This
looks like a startup bug and is not one.

Always restart with the ports cleared:

```bash
make stop-local
for port in 8080 8081 8082 8083 8084 8085; do
  pid=$(lsof -ti tcp:$port -sTCP:LISTEN 2>/dev/null); [ -n "$pid" ] && kill $pid
done
make run-local
```

Rotating the logs first makes the next run readable, which matters when you are
about to read them:

```bash
mkdir -p /tmp/gitscanner-logs/prev && mv /tmp/gitscanner-logs/*.log /tmp/gitscanner-logs/prev/
```

The health block printed by `run-local` is sampled moments after launch, so an
occasional FAIL there is a slow start rather than a failure. Re-check before
acting on it. A service that cannot reach Postgres or RabbitMQ keeps retrying
for `CONNECT_TIMEOUT` (default `1m`), logging "backend not reachable yet,
retrying", before it exits — so a FAIL that persists while the PID is still
running means a backend is down, not a slow start:

```bash
for p in 8080 8081 8082 8083 8084 8085; do
  printf ":$p "; curl -sf http://localhost:$p/readyz || echo FAIL; echo
done
```

## Read the logs

```bash
bin/logs                      # all six, colour-coded per service and level
bin/logs scan-worker          # one service
bin/logs enricher analytics   # several
```

The services log structured JSON; `bin/logs` renders it as a timestamp, level,
message and fields. For raw lines use `tail -f /tmp/gitscanner-logs/api.log`.
`bin/` is gitignored, so `bin/logs` is a local helper that a fresh clone does
not have; `tail -f /tmp/gitscanner-logs/*.log` always works.

## Submit work

```bash
curl -s -X POST http://localhost:8080/v1/scans/user \
  -H 'Content-Type: application/json' -d '{"name":"octocat","include_forks":false}'

curl -s -X POST http://localhost:8080/v1/scans/repo \
  -H 'Content-Type: application/json' -d '{"url":"https://github.com/owner/repo.git"}'

curl -s http://localhost:8080/v1/scans/<job-id>
curl -s http://localhost:8080/v1/scans/<job-id>/findings
```

The dashboard reads everything from the server, so these are also the quickest
way to look at results from a shell:

```bash
curl -s http://localhost:8080/v1/overview          # headline counts and breakdowns
curl -s http://localhost:8080/v1/scans             # jobs, newest first, with their target (paged: ?limit=&offset=, ?state=active|done)
curl -s 'http://localhost:8080/v1/findings?severity=critical&status=open&q=env&limit=50&offset=0'
curl -s http://localhost:8080/v1/repositories      # per-repository open counts by severity (paged; ?q= ?only_open=1 ?sort=risk|open|recent|name)
curl -s http://localhost:8080/v1/candidates        # unmapped-provider backlog (paged)
curl -s -X PATCH http://localhost:8080/v1/findings/<id> -d '{"status":"false_positive"}'
```

`provider=unmapped` selects findings no named rule attributed; `status` is one
of `open`, `rotated`, `false_positive`, `ignored`.

A scan is asynchronous: the POST returns a queued job, and the pipeline runs
api → discovery-worker → scan-worker → enricher (a `/v1/scans/repo` scan goes
straight from api to scan-worker), with findings published to the
`findings.events` queue for analytics. Expect to wait, and watch the logs rather
than the response.

Live credential validation is **off by default**. Set `PROBE_SECRETS=true` on
the scan-worker to have it make one identity/liveness call per named finding
(is the key live, whose account) and surface a "LIVE — rotate" badge. It
authenticates to providers with the credentials it finds, so enable it **only
for targets you own**:

```bash
make run-local PROBE_SECRETS=true
```

Without `GITHUB_TOKEN` the GitHub API allows 60 requests an hour, which bulk
discovery exhausts quickly. Export one for anything beyond a few repositories.

## Machine-local secrets: `.env.local`

`make run-local` sources `.env.local` (gitignored) before starting the
services; `.env.example` lists the keys. That is where `GITHUB_TOKEN` and the
SMTP settings live, so they never reach the command line or the transcript.

## Disclosure mail

Mail is off until switched on — the "Disclosure mail" switch on the
Repositories page, or `PUT /v1/settings {"notifications_enabled":true}`. While
off, the api refuses `disclose` with 409 and the notifier drops anything already
queued. Without `SMTP_ADDR` the notifier logs the mail instead of sending it,
which is the safe way to read a rendered disclosure. But `.env.example` fills
it in and `run-local` always sources `.env.local`, so comment `SMTP_ADDR` out
there before relying on log-only mode — otherwise real mail goes out.

## Frontend

```bash
make frontend-dev    # Vite on :3000, or the next free port if taken
```

It calls the api at `http://localhost:8080` directly rather than through the
Vite proxy, and CORS is open on the services, so a port other than 3000 is fine.

## Stop

```bash
make stop-local
```

## Overrides

```bash
make run-local \
  INFRA_POSTGRES='postgresql://user:pass@host:5432/db' \
  INFRA_RABBIT='amqp://user:pass@host:5672/' \
  RUN_LOG_DIR=/your/log/path
```

Pass the same `RUN_LOG_DIR` to `make stop-local`, and export it for `bin/logs`;
both otherwise look in `/tmp/gitscanner-logs` and find nothing.

## When it looks stuck

A job whose counters never move, or a service that stops logging, has a small
set of likely causes and a diagnostic order worth following. Use the
`debug-pipeline` skill rather than guessing.
