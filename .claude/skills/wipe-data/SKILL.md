---
name: wipe-data
description: Clear GitScanner's development data — findings, scan jobs, repositories, analytics rollups, provider candidates — and the queued tasks that reference them. Use this whenever the user wants to empty the database, start from a clean slate, reset dev data, says "xóa data", "clear DB", "wipe", or when stale results from an earlier scan are masking the effect of a change.
---

# Wiping development data

During development the database accumulates results from earlier runs, and those
results actively mislead: findings stored under an old rule keep their old
classification, rollups keep counting them, and a rescan reports `findings=0`
because the rows already exist. Clearing the data is often the fastest way to
see what a change really does.

It is also destructive and not undoable, so the scope is the user's call.

## Decide the scope first

"Delete old data" can mean three quite different things. Ask rather than assume,
and show the counts so the choice is informed:

```bash
PGPASSWORD=postgres123 psql -h localhost -U postgres -d app_db -c "
SELECT 'findings' AS t, count(*) FROM findings
UNION ALL SELECT 'scan_jobs', count(*) FROM scan_jobs
UNION ALL SELECT 'repositories', count(*) FROM repositories
UNION ALL SELECT 'targets', count(*) FROM targets
UNION ALL SELECT 'provider_candidates', count(*) FROM provider_candidates
UNION ALL SELECT 'analytics_rollups', count(*) FROM analytics_rollups
ORDER BY 1;"
```

| Scope | What it gives | Cost |
|---|---|---|
| **Everything** | a true clean slate | re-discovery hits the GitHub API again |
| **Results only** | new rules seen from scratch, repositories kept | rollups and candidates still go |
| **Bad rows only** | drops placeholder findings and shapeless candidates | real findings and history survive |

## Stop the workers first

A worker mid-delivery will write rows back into tables being emptied, or fail on
a foreign key as its target disappears. Stop them, wipe, then restart — see the
`run` skill for the port-clearing restart.

```bash
make stop-local
for port in 8080 8081 8082 8083 8084 8085; do
  pid=$(lsof -ti tcp:$port 2>/dev/null); [ -n "$pid" ] && kill $pid
done
```

## Full wipe

`targets` is the root of the foreign-key tree: deleting it cascades to
`scan_jobs`, `repositories`, and through them `findings` and `notifications`.
The analytics tables and `provider_candidates` hang off nothing, so they have to
be named explicitly — forgetting them leaves rollups describing findings that no
longer exist.

One transaction, so a failure part-way cannot leave a half-emptied database:

```bash
PGPASSWORD=postgres123 psql -h localhost -U postgres -d app_db -v ON_ERROR_STOP=1 <<'SQL'
BEGIN;
DELETE FROM targets;                   -- cascades: scan_jobs, repositories, findings, notifications
DELETE FROM analytics_rollups;
DELETE FROM analytics_seen_findings;
DELETE FROM analytics_seen_repos;
DELETE FROM provider_candidates;
COMMIT;
SQL
```

This keeps the schema and the applied migrations — it empties tables, it does
not drop them.

### Results only, keeping the repository inventory

Skip `DELETE FROM targets` and instead clear the results and reset the job
counters, so a rescan runs against repositories already discovered:

```sql
BEGIN;
DELETE FROM findings;
DELETE FROM analytics_rollups;
DELETE FROM analytics_seen_findings;
DELETE FROM analytics_seen_repos;
DELETE FROM provider_candidates;
UPDATE scan_jobs SET repos_scanned = 0, blobs_scanned = 0, findings_count = 0;
COMMIT;
```

### Bad rows only

```sql
DELETE FROM findings WHERE masked_preview LIKE 'your%';   -- template values
DELETE FROM provider_candidates WHERE charset_class = ''; -- recorded before shape metadata existed
```

## Purge the queues too

This is the step that is easy to miss and that spoils the clean slate. Tasks
already queued name job and repository ids that no longer exist; on redelivery
they fail and dead-letter, filling a freshly cleaned environment with errors
that have nothing to do with the code.

Purge repeatedly. Messages sitting in a `.retry` queue carry a TTL whose
expiry routes them back into the main queue, so a single pass leaves work
behind — this was observed, not theorised:

```bash
for round in 1 2 3; do
  for q in scan.tasks enrich.tasks discovery.tasks notify.tasks; do
    for suffix in "" ".retry" ".dead"; do
      curl -s -u admin:admin123 -X DELETE \
        "http://localhost:15672/api/queues/%2F/$q$suffix/contents" >/dev/null
    done
  done
  sleep 2
done
```

## Verify, then restart

```bash
PGPASSWORD=postgres123 psql -h localhost -U postgres -d app_db -c "
SELECT 'findings' AS t, count(*) FROM findings
UNION ALL SELECT 'targets', count(*) FROM targets
UNION ALL SELECT 'provider_candidates', count(*) FROM provider_candidates
UNION ALL SELECT 'analytics_rollups', count(*) FROM analytics_rollups ORDER BY 1;"

# schema and migrations must still be there
PGPASSWORD=postgres123 psql -h localhost -U postgres -d app_db -tAc \
  "SELECT version FROM schema_migrations ORDER BY version;"

curl -s -u admin:admin123 http://localhost:15672/api/queues | python3 -c "
import json,sys; print('total messages:', sum(q.get('messages',0) for q in json.load(sys.stdin)))"
```

Then `make run-local`.

## What a clean slate changes

Worth saying out loud, because it is the reason to do this at all: after a wipe
the next scan exercises the current rules from scratch. Credentials are
attributed to their provider on first detection rather than through the
reclassification path, the placeholder filter drops template values before they
are ever stored, and the analytics rollups are consistent with the findings —
which they cannot be after a reclassification, since the dedupe table rejects
the replay.
