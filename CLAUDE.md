# Working on GitScanner

[README.md](README.md) has the layout, where-to-find-things table and dependency
rules; [plan.md](plan.md) has the design and the reasoning behind it. Read those
rather than rediscovering them. This file covers only what they do not: which
skill to reach for, and the handful of invariants that look like tidy-up targets
but are load-bearing.

## Skills

| Skill | Use it when |
|---|---|
| `architecture` | adding or changing a service, moving logic between packages, touching queue, event or schema plumbing |
| `run` | starting, restarting or observing the local stack; reading logs; submitting a test scan |
| `debug-pipeline` | a scan is stuck, silent, or finds nothing — before theorising about causes |
| `detection-rule` | a credential is missed, attributed to no provider, or a template is reported as a finding |
| `detection-engine` | asking why something was or was not reported, junk findings, or changing the gates in `detect.go` |
| `detection-algorithm` | studying how detection decides, tuning precision (junk findings) or recall, before editing a gate or rule |
| `wipe-data` | clearing dev data when stale results are masking the effect of a change |
| `merge-request` | branching, committing, or opening a PR — read it *before* committing, not after |

## Trunk-based

`main` is the only long-lived branch and must stay releasable and green. Work
goes on a short-lived `type/short-description` branch and reaches `main` through
a pull request titled `type(scope): description`. Nothing is committed straight
to `main`, and pushing or opening a PR needs the user's agreement first — the
`merge-request` skill has the details.
A PR that changes what ships in the frontend also bumps `frontend/package.json`
in the same PR (feat → minor, fix → patch); the skill says when and how.

## Verifying a change

```bash
go build ./... && go vet ./... && go test -count=1 ./test/unit/...
```

Tests live under `test/`, not beside the code, and are external `_test` packages
— see `test/README.md`. `make check` is what CI runs.

Beyond the unit suite, a change to the scan path deserves a real run: start the
stack and watch a scan through the logs. Several of the bugs found in this code
could not fail a unit test, because the failure was a hang rather than a wrong
answer.

## Invariants that look removable and are not

Each of these has a comment at the site saying why. They are listed here because
the comment only helps someone who is already looking at that line.

**`scan/git.go` — cancel `cat-file` before `Wait`.** Every early return from
`Blobs` must be able to kill the subprocess. Without that, nothing drains git's
stdout, git blocks writing and cannot exit, and the deferred `Wait` never
returns — swallowing the error that caused the return. A hang with no log line.

**`notify` — take the rate token before creating the notification row.** The
other order makes a rate-limited retry find its own earlier row, mistake it for
a duplicate under the one-mail-per-repository index, and drop the mail
permanently.

**`store/analytics.go` — the two dedupe tables.** Rollup columns are counters and
delivery is at-least-once, so a redelivered event would double-count. A batch
claims its finding ids and aggregates only the ids it won. The claim and the
counter bump share one transaction.

**`store/findings.go` — the narrow update arm.** A *named* rule may replace the
unattributed catch-all, never one provider another. Without it, improving a rule
changes nothing on any repository already scanned.

**Clone is `--bare`, not `--mirror`.** `--mirror` fetches `refs/*`, which on
GitHub includes one ref per pull request ever opened — gigabytes of other
people's fork commits, which are also not the target's history and were never
authorised as a scan target.

**`libs/rabbit` — dead-letter by publishing, never by nack.** Every queue's
dead-letter exchange is the *retry* exchange, so a nack can only ever retry.
Reaching `.dead`, or deferring without spending an attempt (`ErrRetryLater`),
takes an explicit confirmed publish followed by an ack. Replacing that with a
nack once sent every "dead-lettered" task round the retry loop forever — one
was found on its 421st attempt.

**Rule literals list every casing.** They gate a `bytes.Contains` prefilter,
which does not fold case, so a blob without the exact literal never reaches the
regex.

## Known gaps

- Analytics rollups counted under `provider=unmapped` are not corrected when a
  finding is later reclassified; the dedupe table rejects the replay.
- A repository skipped after its final scan attempt counts toward
  `repos_scanned`, so its job completes, but nothing on the job records that it
  was skipped; only the scan-worker log and the `scan.tasks.dead` queue say so.
- `envVarIn` in `detect.go` matches any word before a colon, so candidate
  `guessed_name` picks up prose from READMEs and YAML keys. Detection tuning is
  Phase 8.

## Conventions worth matching

Comments explain **why**, not what — the existing ones are the model. A comment
earns its place by recording a constraint, a past failure, or a decision a
reader would otherwise undo.

Workers take a `*slog.Logger` and log the start and end of each unit of work
with the ids needed to follow it. Structured fields, not formatted strings.

Never print, store or commit a real credential. `secrets.Mask` and the
fingerprint are the only forms that leave the engine, with one exception:
`secrets.Sealer` ciphertext in `sealed_secrets`, kept only for owners on the
unmask list and shown only while masking is off (plan.md §6). Do not widen that
gate. When investigating a real leak, mask values before they reach the
transcript.
