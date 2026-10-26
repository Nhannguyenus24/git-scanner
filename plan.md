# GitScanner — Implementation Plan

Multi-repo git secret scanner in Go. Source spec: [spec.md](spec.md).

---

## 0. Scope decisions

Three parts of the original spec are rescoped. The engineering is unchanged in each
case; only the target set or the trust boundary moves.

### 0.1 Crawl bounded target sets, not all of public GitHub

A resumable crawler that accumulates live credentials across all of GitHub is the
highest-value attack target we could host, and a breach of it would be worse than
any individual leak it found.

Keep the identical engine — paginated discovery, pause/resume, checkpointed cursor —
but seed it from an authorized set:

- our own org
- a customer's GitHub App installation (repos explicitly granted)
- an explicit allowlist

Only the seed source changes. `discovery-worker` does not care where the repo list
came from.

### 0.2 No live authentication with discovered credentials

Calling a provider's API with someone else's key to test whether it works is
unauthorized access, even for a read-only probe. Risk is derived without touching
the provider:

- key format and embedded scope metadata (JWT claims, AWS key-ID prefix and account,
  GitHub PAT prefix `ghp_` / `github_pat_`)
- commit recency
- whether the blob is still reachable from HEAD
- repo reach (stars, forks, visibility)
- detected app type (see §4.3)

Real verification is gated behind proof of ownership: the requester scanning their
own repo, or an installed GitHub App. It runs only on that path.

The provider registry (§5) **stores** probe definitions for every provider; the
ownership gate decides **when** one is allowed to execute. Storing the capability
and exercising it are separate concerns.

### 0.3 Disclosure mail carries no payment link

Unsolicited mail saying "your credentials leaked" with a donation link attached
reads as a shakedown and will get the sending domain blocklisted. The notice is a
plain responsible-disclosure message: what leaked, where, how to rotate it, and an
unsubscribe. One mail per repo, suppression list, hard rate cap.

The donation link lives on our own site, not in the notice.

---

## 1. Stack

Go has no LTS — the two most recent majors receive security fixes. Pin the current
stable release in `go.mod` with a `toolchain` directive.

| Concern | Choice |
|---|---|
| HTTP | `go-chi/chi/v5`, `log/slog`, OpenTelemetry |
| Postgres | `pgx/v5` + `sqlc`, `golang-migrate` |
| RabbitMQ | `rabbitmq/amqp091-go` |
| SMTP | `wneessen/go-mail` |
| Git | `git` binary over `cat-file --batch` pipes, behind an interface |

**Git library note.** `go-git` is pure Go but degrades badly on large histories;
`git2go` requires cgo. Shelling out to `git` is fastest and most robust. Keep it
behind an interface so it stays swappable.

**One broker, two kinds of message.** RabbitMQ carries both per-task dispatch and
the finding stream analytics consumes, on queues that differ only in what they
mean: a task asks for work, an event reports what happened. Both get the same
ack, retry and dead-letter treatment.

This reverses an earlier split that put the finding stream on Kafka for its
retention, so that a corrected aggregation could be replayed over the log. That
property is genuinely gone: once analytics acks an event it is not recoverable
from the queue. What replaces it is that `findings` is the system of record, so
a rollup is rebuilt by reading the table rather than replaying a topic. The
trade was deliberate — one broker instead of two for a system whose rollups are
derivable from a table it already keeps.

What did *not* change is the need for idempotency. RabbitMQ is at-least-once
just as Kafka was, so the dedupe tables behind the rollups (§10) remain load
bearing.

Infrastructure (Postgres, RabbitMQ) is already deployed in Docker — see
connection values in [spec.md](spec.md).

---

## 2. Services

| Binary | Role |
|---|---|
| `api` | REST, auth, job submission, results, pause/resume |
| `discovery-worker` | Expands a target into scan tasks (user → repo list, org → repo list) |
| `scan-worker` | Clone, walk history, detect, emit findings |
| `enricher` | Resolves a match against the provider registry → provider, type, severity |
| `registry` | Provider catalog, versioning, unmapped backlog (§5) — ships inside `api` initially |
| `notifier` | Disclosure mail, suppression, rate caps |
| `analytics` | Finding stream → rollup tables, serves dashboards |

---

## 3. Repository layout and shared libraries

### 3.1 One module, `libs/` for the common libraries

```
gitscanner/                      module github.com/mservice/gitscanner
  cmd/
    api/  discovery-worker/  scan-worker/  enricher/  notifier/  analytics/
  libs/                          # shared across services
    entity/        domain types + validation, zero I/O
    proto/         buf workspace: proto/ definitions, gen/ generated Go
    rabbit/        reconnect, confirms, prefetch, DLX backoff, cancellable consumer
    observability/ slog + OTel wiring, health probes
    config/        env loading and validation
    lifecycle/     signal handling, ordered shutdown
    buildinfo/     version and commit stamped at link time
    id/            UUIDv7
  internal/                      # service-private, not importable outside
    app/           shared bootstrap: config, connect, build, serve, drain
    api/           http handlers, middleware, openapi
    scan/          engine: clone, blob walk, detect, entropy; blob index, pause/cursor
    rules/         embedded gitleaks-compatible TOML (bootstrap ruleset)
    registry/      provider catalog, versioned definitions, candidates, probes
    classify/      severity scoring, app-type detection
    notify/        smtp, templates, suppression
    analytics/     rollups
    store/         postgres (pgx + sqlc)
    github/        REST + GraphQL client, token pool, rate limiter
    secrets/       fingerprint, mask, envelope crypto
  migrations/
  deploy/
  build/           Dockerfile
  Makefile  Tiltfile  buf.yaml  sqlc.yaml  .golangci.yaml
```

**Why one module rather than a module per lib.** A single `go.mod` means no
`replace` directives, no version skew between services deployed together, and one
shared `go mod download` layer across every service image. Module boundaries do not
actually prevent bad dependencies — they only make them inconvenient. An import
linter does prevent them, and it works inside one module.

**The directory is named `libs/` on purpose.** With module path
`github.com/mservice/gitscanner`, the import path is
`github.com/mservice/gitscanner/libs/rabbit`. If another repository later needs these
libraries, giving `libs/rabbit` its own `go.mod` with that exact module path and
adding a `go.work` makes the split mechanical — **no import statement changes**.
Pay that cost when a second repo actually exists, not before.

### 3.2 Dependency rules

- `libs/entity` imports standard library only.
- `libs/*` must never import `internal/*` or `cmd/*`.
- `internal/*` may import `libs/*`; the reverse is a build failure.
- Enforced by `depguard` in `.golangci.yaml`, checked in CI.

### 3.3 `entity` and `proto` are deliberately separate

`libs/entity` holds domain types used by the store and business logic.
`libs/proto` holds the wire format for the tasks and events on RabbitMQ, generated
by `buf`. Explicit mapping functions sit between them.

Do not let generated proto structs reach the database layer. The moment they do,
the wire format and the schema evolve together, and a backward-compatible proto
change starts requiring a migration. Keeping them apart costs some mapping code and
buys independent evolution of the two.

### 3.4 Keep the transport libraries thin

`libs/rabbit` encodes **our conventions** — queue and
queue naming, proto serialization, trace headers, consumer-group setup, retry and
DLQ topology, graceful shutdown (§13.3), OTel spans. It does not attempt to hide
amqp091.

A wrapper that re-exports every driver option is strictly worse than the driver: it
carries the same complexity plus a layer to debug through. When a service needs
something the convention does not cover, it should reach for the driver directly.

`libs/entity` carries the opposite risk — becoming a god-module every service
depends on, where one field triggers a full rebuild. Keep it pure data plus
validation, with no dependencies.

---

## 4. Scan engine

The central performance property: **scan unique blobs, not commits.** A repo with
50k commits may hold only 8k distinct blobs. Per-commit diffing rescans identical
content hundreds of times.

### 4.1 Pipeline

1. `git clone --mirror` into a scratch dir with a per-job disk quota and guaranteed
   cleanup on every exit path.
2. `git rev-list --objects --all` → dedupe blob OIDs → drop OIDs already scanned in
   a prior run. Incremental rescans become nearly free. (Not implemented: the
   Redis-backed index was never wired in and went with Redis; a revival belongs
   in Postgres.)
3. Stream contents through a single long-lived `git cat-file --batch` process.
   Skip binaries (sniff first 512 bytes) and blobs over ~10 MB.
4. **Literal prefilter before regex.** Running ~400 regexes over every blob is the
   obvious trap. Run one Aho-Corasick pass over rule literal prefixes (`AKIA`,
   `sk_live_`, `ghp_`, `xoxb-`, …) and evaluate only the rules whose anchor hit.
   Order-of-magnitude difference on large repos. The automaton is built from the
   active registry version (§5.6).
5. Detection layers:
   - provider detection rules from the registry (§5.2)
   - Shannon entropy plus keyword proximity for generic high-entropy strings
   - structural decoders: JWT parse, AWS key-ID checksum, base64 inflation
6. False-positive filters: path denylist (`vendor/`, `testdata/`, `*.min.js`,
   lockfiles), placeholder detection (`EXAMPLE`, `your_key_here`, repeated runs).
7. Attribution: resolve blob → introducing commit, author, date **lazily**, only for
   confirmed findings. `git log --find-object` is expensive.

### 4.2 Concurrency

One RabbitMQ consumer per worker with `prefetch = N`; each task fans out over a
bounded `errgroup` across blobs. Backpressure comes from prefetch, not from an
unbounded in-process queue.

### 4.3 Classification — two axes

The spec's "type of app" resolves to two separate classifications, and both are
built:

- **Credential provider taxonomy** — which service the key belongs to. Resolved
  entirely from the registry (§5), never hard-coded.
- **Repo stack / app type** — detected from manifests at HEAD (`go.mod`,
  `package.json`, `requirements.txt`, `Dockerfile`, `pom.xml`).

Stack feeds severity: a leaked DB URL in a deployed service outranks the same string
in a tutorial repo.

---

## 5. Provider registry (mapping service)

Everything the system knows about a provider lives here: how to recognise its key,
what can be read off the key offline, and which API endpoint turns that key into
account info. `scan-worker` and `enricher` hard-code no provider — both load from
the registry.

### 5.1 Deployment

A self-contained module (`internal/registry`) that ships inside the `api` binary
to begin with. It is low-traffic CRUD; a seventh process would add operational
weight for no throughput gain. The module boundary stays clean so it can be split
out later without touching callers.

### 5.2 Provider definition

Each provider is a **versioned, declarative record — data, not code**:

```yaml
slug: cloudinary
name: Cloudinary
category: cdn
credential:
  kind: composite
  parts: [cloud_name, api_key, api_secret]

detect:
  - id: cloudinary-url
    literal: "cloudinary://"          # feeds the Aho-Corasick prefilter
    regex: 'cloudinary://(?P<api_key>[0-9]{15}):(?P<api_secret>[A-Za-z0-9_-]{27})@(?P<cloud_name>[a-z0-9_-]+)'
  - id: cloudinary-secret-env
    keywords: [CLOUDINARY_API_SECRET]
    regex: '[A-Za-z0-9_-]{27}'

extract:                              # offline only, no network
  account: "{{ .cloud_name }}"
  key_id:  "{{ .api_key }}"

probe:                                # runs only on ownership-verified scans (§0.2)
  auth: basic                         # api_key:api_secret
  method: GET
  url: "https://api.cloudinary.com/v1_1/{{ .cloud_name }}/usage"
  allow_hosts: [api.cloudinary.com]
  map:
    plan:       "$.plan"
    quota_used: "$.credits.used_percent"

severity_base: high
rotate_doc: "https://cloudinary.com/documentation/..."
```

Adding Azure is the same shape:

```yaml
slug: azure_storage
name: Azure Storage Account
category: cloud
detect:
  - literal: "AccountKey="
    regex: 'AccountName=(?P<account>[a-z0-9]{3,24});AccountKey=(?P<key>[A-Za-z0-9+/=]{88})'
extract:
  account: "{{ .account }}"
probe:
  auth: azure_shared_key              # HMAC signing — scheme lives in code
  method: GET
  url: "https://{{ .account }}.blob.core.windows.net/?comp=list"
  allow_hosts: ["*.blob.core.windows.net"]
severity_base: critical
```

**Auth schemes are a closed enum implemented in Go** — `basic`, `bearer`, `header`,
`query`, `aws_sigv4`, `azure_shared_key`, `hmac_custom`. The definition only picks a
name. Declarative templates cannot express request signing, and we do not want
arbitrary executable code living in the database.

### 5.3 Two levels of information

- **Offline extraction** — always runs. Parses account, key id, region, scopes,
  expiry straight out of the credential's own structure. Covers most of what the
  notifier and severity scoring need.
- **Probe** — needs the network and the credential. Gated by §0.2. Yields plan,
  quota, active scopes, real account identity.

Severity must be computable without the probe, otherwise most findings have no
score.

### 5.4 Unmapped backlog

This is the "list các service chưa được map" half, and it is what keeps the
catalog growing.

When `enricher` receives a finding that no registry rule claims — or that only the
generic entropy detector produced — it computes a **shape signature**:

- charset and length
- leading token, if any (`sk-`, `xapp-`, `AIza`)
- the surrounding environment variable or config key name
- any hostname appearing nearby in the same blob

Findings cluster by that signature and upsert into `provider_candidates` with an
occurrence count, first/last seen, and a masked sample.

The environment variable name is by far the strongest naming signal —
`MAILGUN_API_KEY`, `AZURE_STORAGE_KEY`, `CLOUDINARY_API_SECRET` effectively name
the missing provider for you. Rank the candidate list by occurrence count and the
backlog sorts itself by impact.

### 5.5 API

```
GET    /v1/providers                               # mapped catalog; filter by category, status
GET    /v1/providers/{slug}
POST   /v1/providers                               # create draft
POST   /v1/providers/{slug}/versions               # new draft version
POST   /v1/providers/{slug}/versions/{v}/test      # run against fixtures
POST   /v1/providers/{slug}/versions/{v}/publish
POST   /v1/providers/{slug}/deprecate

GET    /v1/providers/candidates                    # NOT yet mapped, ranked by occurrences
POST   /v1/providers/candidates/{id}/promote       # candidate → draft definition
POST   /v1/providers/candidates/{id}/ignore

GET    /v1/providers/coverage                      # % of findings resolved to a provider
```

`coverage` is the metric that says whether the catalog is keeping up with reality;
it feeds analytics (§10).

### 5.6 Versioning and rollout

A definition version is immutable: `draft → test → published → deprecated`.

- **test** runs the version against stored fixtures. Fixtures are **synthetic keys
  only** — never a real leaked credential from the findings table.
- **publish** bumps a global `registry_version` and announces it on the
  `registry.events` queue.
- Workers rebuild the Aho-Corasick automaton and compiled regex set **off to the
  side**, then swap it in with `atomic.Pointer`. No lock, no dropped in-flight task.
- Workers report their active `registry_version` in the heartbeat, so a rollout is
  observable and a stuck worker is obvious.

### 5.7 Reclassify vs rescan

Worth separating, because it decides how expensive a catalog change is:

- **New mapping for an already-detected key** (pure classification): replay
  enrichment over stored findings. No repo is cloned again. Cheap.
- **New detection rule** (a pattern nothing ever matched): blobs must be walked
  again. The blob-OID dedupe in §4.1 makes this far cheaper than a cold scan, but
  it is not free.

So version `detect` separately from `extract`/`probe` inside a definition. Then the
publish step can tell the operator which of the two a change triggers, instead of
conservatively rescanning everything.

### 5.8 Security of the registry itself

An API that accepts "a URL to call with a credential" **is an SSRF primitive by
construction**. Non-negotiable controls:

- `allow_hosts` is mandatory on every probe and matched before the request is built.
- Block private and link-local ranges — `10/8`, `172.16/12`, `192.168/16`, `127/8`,
  `169.254/16`, `::1`, `fc00::/7`. Enforce in the dialer's `Control` hook on the
  resolved IP, not by inspecting the hostname — otherwise DNS rebinding walks
  straight through.
- Redirects disabled. Request timeout and response size cap.
- RBAC: only an admin role may write definitions. Every version records its author.
- Go's `regexp` is RE2, so no catastrophic backtracking — but still cap pattern
  length, measure compile time, and require a literal anchor so the pattern can
  participate in the prefilter.

---

## 6. Secret storage

**Never store the plaintext secret.**

The `findings` row holds:

- a keyed **HMAC-SHA256 fingerprint** — for dedupe and idempotency
- a masked preview — `sk_live_4f2a…9c1d`

If an owner-verified flow genuinely requires the full value, envelope-encrypt it per
row under a separate KMS key with a short TTL.

That flow exists for detection testing: owners on `unmask_owners` (the operator's
own accounts) have each credential sealed with AES-256-GCM under `SECRET_SEAL_KEY`
into `sealed_secrets`, bound to its repository and fingerprint, expiring after 30
days. The api opens them only while the masking switch is off. `findings` stays
secret-free, mail is always masked, and no other repository is ever sealed.

Without this, our Postgres becomes more valuable than any single repository it
scanned.

The fingerprint also supplies free idempotency:
`ON CONFLICT (fingerprint, repo_id) DO NOTHING` makes RabbitMQ redelivery harmless.

---

## 7. Schema

- `targets`
- `scan_jobs` — state machine `queued | running | paused | completed | failed`,
  progress counters, resume cursor
- `repositories`
- `findings` — fingerprint, rule_id, provider_slug, blob_oid, commit_sha, path, line,
  author, severity, status
- `providers` — slug, name, category, status, current_version
- `provider_versions` — immutable definition JSONB, author, state
- `provider_candidates` — shape signature, guessed name, occurrences, masked sample,
  first/last seen, status (`new | triaged | mapped | ignored`)
- `provider_fixtures` — synthetic test vectors and their expected parse
- `notifications`
- `suppressions`
- `analytics_rollups`

---

## 8. API

```
POST   /v1/scans/repo        {url, ruleset}
POST   /v1/scans/user        {username, include_forks, include_gists}
POST   /v1/scans/bulk        {org | allowlist}        # bounded, authorized sets
GET    /v1/scans/{id}                                 # status + progress
POST   /v1/scans/{id}/pause
POST   /v1/scans/{id}/resume
POST   /v1/scans/{id}/cancel
GET    /v1/findings?provider=&severity=&repo=
GET    /v1/analytics/summary
GET    /v1/analytics/trends
GET    /v1/analytics/providers
```

Registry endpoints: §5.5. Auth: API key or JWT, scoped per tenant.

**Pause/resume.** The worker checks `scan_jobs.pause_requested` each batch, flushes its cursor to
`scan_jobs`, and acks. Resume re-enqueues from the stored cursor.

---

## 9. GitHub client

- Token pool with per-token buckets (5000 req/hr each), on the shared Postgres
  bucket the notifier uses
- Secondary rate limit handling; honour `Retry-After`
- ETag conditional requests
- GraphQL for user and org repo enumeration — one query rather than N REST calls

---

## 10. Analytics

`scan-worker` and `enricher` publish to the `findings.events` queue. The
`analytics` consumer upserts rollups: findings by provider per day, repos scanned, top firing
rules, time-to-remediation, false-positive rate, and **registry coverage** (§5.5).
Served from `/v1/analytics/*`.

Consuming the stream rather than querying `findings` on every request keeps the
dashboards off the operational path, which is the reason the rollups exist.

Rebuilding them is a separate matter. A queue is not a retained log, so a
corrected aggregation cannot be replayed over it — the rebuild reads `findings`,
which is the system of record, and repopulates the rollup and dedupe tables
together. That is a batch job run deliberately, not something the stream gives
for free (§1).

---

## 11. Phases

| Phase | Deliverable |
|---|---|
| 0 | Scaffolding, migrations, wiring to existing PG / Rabbit / Redis; Helm chart skeleton and dev namespace (§13) |
| 1 | Repo scan end-to-end → findings in Postgres (vertical slice) |
| 2 | User scan, discovery worker, checkpoint / pause / resume |
| 3 | Provider registry: catalog, versioned definitions, hot reload; enricher reads from it |
| 4 | Candidates backlog, mapping CRUD API, coverage metric |
| 5 | Notifier with suppression and rate caps |
| 6 | Analytics consumer and endpoints |
| 7 | Ownership-gated probe execution with the §5.8 SSRF controls |
| 8 | Hardening: FP tuning against a labeled corpus, load test, observability, KEDA autoscaling and drain behaviour verified under load (§13) |

Phase 3 seeds the registry by importing the Gitleaks TOML corpus as initial
definitions, so the catalog does not start empty.

---

## 12. Testing

- `testcontainers-go` for integration tests against real Postgres / Rabbit.
- Build a **labeled fixture repo early** — known secrets at known commits, plus known
  decoys. False-positive rate is the metric that decides whether anyone trusts the
  tool; it needs to be measurable from Phase 1.
- Every provider definition ships with fixtures and cannot be published until they
  pass (§5.6).

---

## 13. Kubernetes deployment

### 13.1 Packaging

One Helm chart with a generic service template plus a values block per service —
the six workloads are structurally identical (Deployment, Service, PDB, scaler,
ConfigMap, ServiceAccount) and differ only in command, resources and scaling
trigger. Overlays per environment via separate values files.

```
deploy/
  chart/
    templates/{deployment,service,pdb,scaledobject,servicemonitor,networkpolicy}.yaml
    templates/job-migrate.yaml
  values/{dev,staging,prod}.yaml
```

### 13.2 Existing infrastructure is outside the cluster

The connection values in [spec.md](spec.md) are `localhost` against Docker on a
workstation. Inside a pod `localhost` is the pod itself, so **this is the first
thing that will break**.

Expose each dependency as a Service in-cluster so application config never carries
an environment-specific host:

```yaml
# DNS-addressable host
apiVersion: v1
kind: Service
metadata: { name: postgres }
spec:
  type: ExternalName
  externalName: pg.infra.internal
```

`ExternalName` only resolves DNS names. For a bare IP, use a headless Service with
a hand-written `EndpointSlice` instead. Either way the app connects to
`postgres:5432` in every environment, and moving the infrastructure into the
cluster later changes nothing in the app.

Also: those credentials are development values committed to the repo. They must be
rotated and must not be reused by the cluster.

### 13.3 scan-worker is the workload that needs real design

The other five are ordinary stateless Deployments. `scan-worker` is not: it needs
disk, it runs long tasks, and it processes hostile input.

```yaml
spec:
  terminationGracePeriodSeconds: 300
  securityContext:
    runAsNonRoot: true
    seccompProfile: { type: RuntimeDefault }
  automountServiceAccountToken: false
  containers:
    - name: scan-worker
      securityContext:
        readOnlyRootFilesystem: true
        allowPrivilegeEscalation: false
        capabilities: { drop: ["ALL"] }
      resources:
        requests: { cpu: "2", memory: "2Gi" }
        limits:   { cpu: "2", memory: "2Gi" }     # Guaranteed QoS
      volumeMounts:
        - { name: scratch, mountPath: /scratch }
  volumes:
    - name: scratch
      emptyDir:
        sizeLimit: 20Gi
```

**Scratch disk.** `emptyDir` with an explicit `sizeLimit`, backed by disk — not
`medium: Memory`, which would put a mirror clone of a large repo into RAM and take
the node down. The limit means an oversized repo evicts its own pod instead of
filling the node. Pair it with the per-job quota from §4.1 so the application
fails cleanly before the kubelet intervenes. If scans need more than node ephemeral
storage can give, switch to a generic ephemeral volume with a
`volumeClaimTemplate`.

**Guaranteed QoS.** Requests equal to limits, plus a `PriorityClass`. A worker
evicted 8 minutes into a large repo scan wastes all of it.

**Graceful drain.** This is the correctness-critical part. On `SIGTERM` the worker
must:

1. cancel its RabbitMQ consumer so no new delivery arrives,
2. flush the current cursor to `scan_jobs` (§8),
3. finish the in-flight task if time allows, otherwise `nack(requeue=true)`,
4. exit.

`terminationGracePeriodSeconds` must exceed a typical task, not the longest
possible one — the requeue path covers the tail. Requeue is safe because findings
are idempotent on `(fingerprint, repo_id)` per §6, so a redelivered task cannot
double-write.

**Image.** §1 shells out to the `git` binary, so this image cannot be distroless or
`scratch`. Build on `debian-slim` or `alpine` with `git` installed, run as non-root.

**Hostile input.** `git` is parsing repositories from the internet and has a CVE
history. Harden the invocation: `GIT_CONFIG_NOSYSTEM=1`, `core.hooksPath=/dev/null`,
`protocol.allow=never` with https explicitly permitted, and
`--no-recurse-submodules` — a submodule URL is attacker-controlled and would
otherwise be fetched. Consider a dedicated node pool or gVisor runtime class for
these pods.

### 13.4 Autoscaling on queue depth

HPA on CPU is the wrong signal for queue consumers — a worker blocked on a slow
clone looks idle, and a backlog of 10k repos produces no CPU pressure until work is
already late. Use KEDA:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata: { name: scan-worker }
spec:
  scaleTargetRef: { name: scan-worker }
  minReplicaCount: 1
  maxReplicaCount: 40
  cooldownPeriod: 600          # longer than a typical scan
  triggers:
    - type: rabbitmq
      metadata:
        queueName: scan.tasks
        mode: QueueLength
        value: "5"
```

Per workload:

| Service | Trigger |
|---|---|
| `api` | HPA on CPU / RPS |
| `discovery-worker` | KEDA RabbitMQ, `discovery.tasks` |
| `scan-worker` | KEDA RabbitMQ, `scan.tasks` |
| `enricher` | KEDA RabbitMQ, `enrich.tasks` |
| `analytics` | KEDA RabbitMQ, `findings.events` |
| `notifier` | **fixed low replica count — see below** |

`cooldownPeriod` must exceed typical task duration, otherwise KEDA scales down into
a pod that is mid-scan and forces the requeue path constantly.

### 13.5 Scaling breaks the mail rate cap

§0.3 caps disclosure mail. If that cap is enforced per-process, scaling `notifier`
to 10 replicas multiplies the send rate by 10 and the domain gets blocklisted —
exactly the outcome the cap exists to prevent.

The limiter must be a **shared token bucket** — a locked row in Postgres
(`rate_buckets`), which every replica serialises on — with the suppression-list
check inside the same atomic operation as the send claim. Keep `notifier` at a low
fixed replica count anyway; it is not throughput-bound.

### 13.6 Migrations

A Helm `pre-install,pre-upgrade` hook Job, not an initContainer. With several
replicas starting at once, initContainers race; `golang-migrate` takes an advisory
lock but losers then block on it and trip their startup probes.

```yaml
metadata:
  annotations:
    "helm.sh/hook": pre-install,pre-upgrade
    "helm.sh/hook-delete-policy": before-hook-creation
```

### 13.7 Secrets

Never plain `Secret` manifests in git. Use External Secrets Operator against the
cluster's secret store, or Sealed Secrets if there is no external store.

Two keys need stricter handling than the infrastructure passwords: the HMAC
fingerprint key and the envelope-encryption KMS key from §6. Mount them only into
the workloads that need them — `scan-worker` and `enricher` — never into `api` or
`analytics`.

### 13.8 NetworkPolicy

Default-deny egress on every namespace, then open only what each workload needs.

For `scan-worker`, egress is GitHub plus the infrastructure Services. For probe
execution (Phase 7), the §5.8 dialer controls are the primary defence, and a
NetworkPolicy blocking RFC1918 and `169.254.169.254` is defence in depth — a single
registry definition pointing at the cloud metadata endpoint would otherwise hand out
node credentials. Route probe traffic through an egress proxy that enforces the
`allow_hosts` list independently of application code.

### 13.9 Registry rollout across pods

§5.6 distributes definition versions over RabbitMQ, which works
across replicas unchanged. Two K8s-specific requirements:

- A pod starting after a publish has missed the notification, so startup must load
  the full active registry, not just subscribe to deltas.
- The readiness probe must fail until the registry is loaded and the Aho-Corasick
  automaton is built. A worker that is ready but has no rules silently returns zero
  findings.

Export the active `registry_version` as a Prometheus gauge so a rollout stall is
visible per pod.

### 13.10 Observability

OTel collector as a DaemonSet, `ServiceMonitor` per workload. The metrics that
matter operationally: queue depth and consumer lag, scan duration percentiles,
scratch-disk high-water mark, mail send rate against the cap, and registry version
skew across pods.

---

## 14. Local pipeline — build and deploy into k3d on macOS

### 14.1 Cluster

`k3d` on this Mac. Lighter than minikube, and it solves the two local problems —
image distribution and reaching the existing Docker infrastructure — in one command:

```bash
k3d cluster create gitscanner \
  --agents 2 \
  --registry-create gitscanner-registry:0.0.0.0:5050 \
  --network <the docker network the infra containers are already on>
```

- `--registry-create` starts a registry reachable as `localhost:5050` from the Mac
  and `k3d-gitscanner-registry:5000` from inside the cluster. No `kind load`, no
  remote registry, and image pulls stay fast enough for an edit-run loop.
- `--network` joins the cluster to the network where Postgres and RabbitMQ
  are already running, so pods resolve them **by container name**. Confirm the
  network with `docker network ls` and the registry host with `k3d registry list`.

This is the local answer to §13.2: the `ExternalName` Services point at those
container names in dev and at real hostnames in staging and prod, while application
config says `postgres:5432` everywhere. If the infra containers are not on a shared
network, `host.k3d.internal` reaches the Mac host instead.

### 14.2 One Dockerfile for every service

```dockerfile
FROM golang:1.25 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download                      # cached across all services
COPY . .
ARG SERVICE
RUN CGO_ENABLED=0 go build -trimpath -o /out/app ./cmd/${SERVICE}

FROM gcr.io/distroless/static:nonroot AS runtime
COPY --from=build /out/app /app
USER nonroot
ENTRYPOINT ["/app"]

FROM debian:stable-slim AS runtime-git    # scan-worker only, per §13.3
RUN apt-get update \
 && apt-get install -y --no-install-recommends git ca-certificates \
 && rm -rf /var/lib/apt/lists/*
RUN useradd -u 10001 -m scanner
COPY --from=build /out/app /app
USER 10001
ENTRYPOINT ["/app"]
```

Five services build `--target runtime`; `scan-worker` builds `--target runtime-git`
because it shells out to the `git` binary. The shared `go mod download` layer is why
a single module (§3.1) matters for build times — six images, one dependency fetch.

### 14.3 Inner loop: Tilt

```python
registry = 'localhost:5050'
services  = ['api', 'discovery-worker', 'enricher', 'notifier', 'analytics']

for s in services:
    docker_build(registry + '/' + s, '.',
        dockerfile='build/Dockerfile',
        target='runtime',
        build_args={'SERVICE': s},
        only=['go.mod', 'go.sum', 'cmd/' + s, 'libs', 'internal'])

docker_build(registry + '/scan-worker', '.',
    dockerfile='build/Dockerfile', target='runtime-git',
    build_args={'SERVICE': 'scan-worker'},
    only=['go.mod', 'go.sum', 'cmd/scan-worker', 'libs', 'internal'])

k8s_yaml(helm('deploy/chart', values=['deploy/values/dev.yaml']))
```

The `only=` lists are what make a monorepo tolerable locally — editing
`internal/notify` must not rebuild six images. Note that every service lists `libs`,
so a change in `libs/entity` does rebuild everything. That is correct, and it is the
cost §3.4 warns about.

### 14.4 Apple Silicon

Local builds are arm64 and must stay single-arch — building multi-arch on the Mac
goes through QEMU and is slow enough to ruin the loop. CI builds `linux/amd64` and
`linux/arm64` with `docker buildx`.

Worth stating explicitly: **dev runs a different architecture than prod.** Anything
architecture-sensitive — cgo, assembly, `GOARCH`-dependent behaviour — will not be
caught locally. Current stack is pure Go with `CGO_ENABLED=0`, so this is low risk,
but it is a reason not to add a cgo dependency casually.

### 14.5 Dev resource overrides

§13.3 asks for 2 CPU / 2 Gi Guaranteed per `scan-worker` and up to 40 replicas.
That does not fit in a laptop VM. `deploy/values/dev.yaml` overrides:

```yaml
scan-worker:
  replicas: 1
  resources:
    requests: { cpu: 200m, memory: 512Mi }
    limits:   { cpu: 500m, memory: 1Gi }
  scratchSizeLimit: 5Gi
  keda: { minReplicaCount: 1, maxReplicaCount: 2 }
```

Keep the *shape* identical to prod — same chart, same probes, same drain path,
same KEDA scalers — and change only the numbers. A dev environment that stubs out
autoscaling stops testing the thing most likely to break in prod.

Two objects legitimately do not render in dev, and it is worth knowing why rather
than discovering it during an incident:

- **PodDisruptionBudgets** are skipped at `replicas: 1`. A PDB with
  `maxUnavailable: 1` over a single replica blocks node drains outright, so the
  chart emits them only above one replica. Dev therefore does not exercise them.
- **NetworkPolicies** are gated on `env: prod`. Default-deny egress against a
  cluster whose dependencies live on the Docker host makes local debugging
  considerably worse. The cost is that §13.8 is only ever verified in staging —
  so staging must run with `env: prod` values, not dev ones.

### 14.6 Pipeline stages

```
make proto      # buf lint, buf breaking --against main, buf generate
make sqlc       # sqlc generate
make lint       # golangci-lint, including the §3.2 depguard rules
make test       # unit + testcontainers integration
make images     # buildx all six
make deploy     # helm upgrade --install gitscanner deploy/chart -f values/dev.yaml
```

`make deploy` runs the migration Job as a Helm `pre-upgrade` hook (§13.6), locally
exactly as in prod.

A CI workflow runs the same targets so there is one definition of "builds and
passes". The step that earns its keep in a monorepo with shared proto is
`buf breaking --against '.git#branch=main'` — it blocks a wire-incompatible change
to `libs/proto` before it reaches a consumer that was not rebuilt.

### 14.7 Cluster add-ons

KEDA is required for §13.4 and installs via Helm into the local cluster. Install it
in dev too, for the reason in §14.5.

Argo CD is optional locally. Tilt covers the inner loop; add Argo only when there is
a real shared cluster where GitOps reconciliation matters.

### 14.8 Bootstrap order

1. `k3d cluster create` as in §14.1
2. `helm install keda kedacore/keda -n keda --create-namespace`
3. `kubectl create ns gitscanner`, seal or inject secrets (§13.7)
4. `make proto sqlc` — generated code must exist before the first build
5. `tilt up`
