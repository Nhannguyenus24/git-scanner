---
name: debug-pipeline
description: Diagnose a GitScanner scan that appears stuck, silent, or is producing no findings — counters frozen at zero, a service that stopped logging, a queue that will not drain, or work that vanishes without an error. Use this whenever the user reports the app "đứng yên", hangs, looks frozen, "nothing happens", a job stays at 0%, or a scan never finishes, instead of guessing at causes.
---

# Diagnosing a stuck pipeline

The hard part of this system is that a stall rarely announces itself. Work is
spread across six services, RabbitMQ and `git` subprocesses, and several of its
real failure modes present identically: nothing moves and nothing logs.

So resist the pull to theorise. Every cause below was found by walking the ladder
in order, and the ones that look most obvious from the outside were wrong more
often than right.

## The ladder

Work from cheapest to most invasive and stop when something is anomalous.

### 1. Are the processes alive?

```bash
for f in /tmp/gitscanner-logs/*.pid; do
  s=$(basename $f .pid); p=$(cat $f)
  kill -0 $p 2>/dev/null && echo "$s RUNNING" || echo "$s DEAD"
done
```

A DEAD service exited — read the tail of its log; the `fatal:` line names the
cause. Usually one of: a port it could not bind; a backend still unreachable
after `CONNECT_TIMEOUT` (default 1m — "backend not reachable yet, retrying"
followed by "gave up after N attempts"); or "worker stopped with error" because
its RabbitMQ connection dropped. Nothing restarts it under run-local. See the
`run` skill — restart with the ports explicitly cleared. A service that is
RUNNING but fails `/readyz` is usually still retrying a backend.

### 2. Who went quiet, and how long ago?

This single check localises most problems.

```bash
for svc in api discovery-worker scan-worker enricher notifier analytics; do
  last=$(tail -1 /tmp/gitscanner-logs/$svc.log 2>/dev/null | python3 -c "
import sys,json
try: print(json.loads(sys.stdin.read()).get('time','?')[11:19])
except: print('?')")
  printf "  %-17s %s\n" "$svc" "$last"
done
echo "  now:              $(date +%H:%M:%S)"
```

A service silent for minutes while others are active is where to look next.

### 3. Where is the work sitting?

```bash
curl -s -u admin:admin123 http://localhost:15672/api/queues | python3 -c "
import json,sys
for q in json.load(sys.stdin):
    r,u = q.get('messages_ready',0), q.get('messages_unacknowledged',0)
    if r or u: print(f\"  {q['name']:22s} ready={r:4d} unacked={u}\")"
```

Read it as follows. `unacked` that never changes means a consumer took the work
and is blocked inside the handler — go to step 5. A growing `.dead` queue means
tasks were given up on: each carries the reason in its `x-gitscanner-dead-reason`
header, and the service logged "task dead-lettered". A `.retry` queue that
never drains, with "task deferred, attempt not counted" in the log, is a rate
limit (discovery: "discovery rate limited"; notifier: the mail rate) — those
tasks cycle every 30s without spending attempts, so that is waiting, not a
hang. All
queues empty while a job still reports work outstanding points at the database,
not the broker.

### 4. What does the database think?

```bash
PGPASSWORD=postgres123 psql -h localhost -U postgres -d app_db -c \
  "SELECT id, state, repos_total, repos_scanned, findings_count, error FROM scan_jobs ORDER BY created_at DESC LIMIT 5;"
```

`repos_total` climbing with `repos_scanned` stuck at 0 means discovery works and
scanning does not.

### 5. What is the blocked service actually doing?

Two signals together tell you a great deal.

```bash
PID=$(cat /tmp/gitscanner-logs/scan-worker.pid)
ps -o pid,etime,%cpu,stat -p $PID

# scan-worker shells out to git; find its children
pgrep -P $PID | while read c; do ps -o pid,etime,%cpu,command -p $c | tail -1; done
```

Beware the pattern that misled this investigation once: the command is
`git -C <dir> cat-file`, so a grep for `git cat-file` finds nothing and invites
the false conclusion that no git process is running. Match on `git` alone, or
list children by PID as above.

Interpretation:

- **Go process burning CPU** — it is working. A large repository genuinely takes
  time; check progress before calling it stuck.
- **Go process at 0% CPU** — blocked on I/O. Go to step 6.
- **Both the Go process and a git child at 0% CPU** — a pipe deadlock. This is
  the signature of the class of bug described below.
- **A git clone that has run for minutes** — check whether it is stalled or
  simply enormous, by watching the directory grow:

```bash
# clones live under SCRATCH_DIR, else the OS temp dir; two may run at once
D=$(ls -dt "${SCRATCH_DIR:-$TMPDIR}"/scan-* 2>/dev/null | head -1)
du -sh "$D"; sleep 5; du -sh "$D"
```

Growth means it is downloading, not hung. A clone can no longer hang forever:
git aborts a transfer below 1 KB/s for 60s, and the whole clone is cut off at
30 minutes ("git clone: gave up after 30m0s"). Either surfaces as a scan
failure and a retry. If a small repository is pulling
hundreds of megabytes, suspect a ref explosion:

```bash
git ls-remote https://github.com/owner/repo.git | wc -l
```

### 6. What is it blocked on?

```bash
lsof -nP -p $PID | grep -E 'TCP|PIPE'
```

Established connections to 5432 (Postgres) and 5672 (RabbitMQ) are normal —
both are held open. What you are looking for is which one the goroutine is waiting on,
which the next step settles definitively.

### 7. Goroutine dump — stop guessing

`SIGQUIT` makes Go print every goroutine's stack and then exit. Because
`run-local` redirects stderr into the log file, the dump lands in the service's
own log and is readable afterwards.

This kills the process. That is acceptable here: unacked RabbitMQ deliveries are
redelivered, and findings are idempotent on their fingerprint, so nothing is
lost. Restart afterwards.

```bash
kill -QUIT $PID
sleep 3
# the goroutine holding the work
awk '/^goroutine /{buf=""} {buf=buf"\n"$0} /^$/{if (buf ~ /scanworker|Engine|gitRepo|Emit/) print buf}' \
  /tmp/gitscanner-logs/scan-worker.log | head -60
```

Read the stack from the bottom up to find application frames. A frame with a
file and line number in `internal/` tells you exactly which line is parked, and
that is normally the end of the investigation.

## Failure modes already found here

Knowing these shortens the ladder, but confirm rather than assume — the symptom
is the same for all of them.

**Pipe deadlock around `git cat-file`.** `scan.(*gitRepo).Blobs` streams blobs
through a long-lived `cat-file --batch`. If the function returns early while the
feeder goroutine is still writing OIDs, nothing drains git's stdout, git blocks
writing, and so cannot exit — and a `Wait()` on it never returns. The original
error that caused the early return is swallowed, which is why the symptom is a
silent hang rather than a failure. Fixed by giving `cat-file` a cancellable
context and cancelling before `Wait`. If you add another early return in that
function, preserve that ordering.

**A blocked network call inside the blob walk.** The deadlock above is usually
*triggered* by something else failing in the `Emit` callback — a RabbitMQ
confirm, a database write. When a scan hangs, suspect the callback before the
git plumbing.

**A clone that is huge rather than hung.** `git clone --mirror` fetches `refs/*`,
which on GitHub includes `refs/pull/*` — one ref per pull request ever opened.
`octocat/Spoon-Knife` advertises 63,775 refs of which 3 are branches. Cloning is
now `--bare`, which takes branches and tags only. Those PR refs are also other
people's fork commits, so scanning them would attribute another repository's
secrets to this one.

**A repository that keeps failing.** After its final attempt the scan-worker
logs "repository skipped after final attempt" and the task goes to
`scan.tasks.dead`. The repository still counts toward `repos_scanned`, so the
job completes without recording the skip (a known gap in CLAUDE.md). When
discovery gives up, the job goes to `failed` with the reason in
`scan_jobs.error`.

**Tasks referencing deleted rows.** After wiping the database, queued work still
names ids that no longer exist. Scan and discovery tasks for a deleted job are
skipped, enrich tasks dead-letter, and `findings.events` would be counted again
into the freshly emptied analytics rollups. Purge the queues too — and purge repeatedly, because messages sitting in a `.retry`
queue have a TTL and migrate back into the main queue mid-purge:

```bash
for round in 1 2 3; do
  for q in scan.tasks enrich.tasks discovery.tasks notify.tasks findings.events; do
    for suffix in "" ".retry" ".dead"; do
      curl -s -u admin:admin123 -X DELETE \
        "http://localhost:15672/api/queues/%2F/$q$suffix/contents" >/dev/null
    done
  done
  sleep 2
done
```

## Findings that look missing but are not

Not every "no findings" report is a stall.

- **A rescan reports `findings=0`.** `InsertFinding` is idempotent on
  `(fingerprint, repository_id)`, and the counter only counts rows actually
  inserted. Already-stored findings are correctly not re-counted. Check the
  table, not the log line.
- **A credential is detected but attributed to no provider.** It matched the
  generic entropy pass rather than a named rule. That is a detection-coverage
  gap — use the `detection-rule` skill.
- **An improved rule changes nothing on an already-scanned repository.** A
  rescan can re-attribute a finding, and the scan log reports it as
  `reclassified` rather than as a new finding. Analytics rollups already counted
  under the old classification are *not* corrected, which is a known gap.
