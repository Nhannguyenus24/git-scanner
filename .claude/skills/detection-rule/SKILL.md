---
name: detection-rule
description: Add or fix a credential detection rule in GitScanner's scan engine — when a provider's key is leaked but not caught, caught but attributed to no provider, or a placeholder is reported as a real finding. Use this whenever the user says a secret "chưa quét được", is missed, is misclassified, shows up as generic-high-entropy, or asks to support a new provider's credential format.
---

# Adding a detection rule

How the whole pipeline fits together — every gate, in order, and how to
measure a change against real repositories — is in the `detection-engine`
skill. This one covers a single provider's rule.

Rules live in `internal/scan/bootstrap_rules.go`. This is the Phase 1 built-in
set; Phase 3 moves the same shape into the provider registry, so a rule written
here should read like a registry definition — a pattern, the literals that gate
it, and optionally keywords.

## First, find out what is actually wrong

"Not detected" has three distinct causes with different fixes, and they are easy
to confuse. Check the database before changing anything:

```bash
PGPASSWORD=postgres123 psql -h localhost -U postgres -d app_db -c \
  "SELECT rule_id, provider_slug, severity, masked_preview, file_path
   FROM findings f JOIN repositories r ON r.id = f.repository_id
   WHERE r.name ILIKE '%<repo>%';"
```

- **Nothing at all for that file** — the blob was skipped, or no rule and not
  the entropy pass matched. A real gap.
- **Present as `generic-high-entropy` with an empty `provider_slug`** — the
  bytes *were* caught; what is missing is attribution. This is the common case
  and it is what makes a leak invisible as *that provider's* leak. Writing a
  named rule fixes it.
- **A `your-...` template value reported as a finding** — a placeholder-filter
  gap, in `isPlaceholder` in `detect.go`, not a rule problem. URL-password
  placeholders (`${DB_PASS}`, user == pass, `changeme`) and published example
  keys are dropped in `namedSuppressed` / `placeholderURLCredential` /
  `isFakeValue` — look there too.

## Learn the real shape without printing the secret

You will often need to inspect a real leaked credential to write the pattern.
Derive its shape; never echo the value into the transcript, a test, or a commit.

```bash
git clone --bare --quiet https://github.com/owner/repo.git /tmp/probe
cd /tmp/probe
git rev-list --objects --all | awk '{print $1}' | sort -u | while read oid; do
  [ "$(git cat-file -t "$oid" 2>/dev/null)" = blob ] || continue
  git cat-file blob "$oid" 2>/dev/null | grep -Ei "(api[_-]?key|secret)\s*[:=]" /dev/stdin | sed "s|^/dev/stdin|$oid|"
done | head | python3 -c "
import sys,re
for l in sys.stdin:
    print(re.sub(r'[A-Za-z0-9_-]{12,}', lambda m: m.group()[:3]+'…['+str(len(m.group()))+'ch]', l.rstrip())[:180])"
```

That prints setting names and value *lengths* while masking the values, which is
all a pattern needs.

## Write the rule

```go
{
    id: "cloudinary-api-secret", provider: "cloudinary", severity: "critical",
    pattern:  `(?i)cloudinary[._-]?api[._-]?secret\s*[:=]\s*["']?(?P<secret>[0-9A-Za-z_-]{27})`,
    literals: []string{"cloudinary", "CLOUDINARY", "Cloudinary"},
},
```

Four things govern whether it works.

**The `(?P<secret>...)` group is required.** `Rule.compile` rejects a pattern
without it, because the engine needs to know which span is the credential as
opposed to the context around it. Keep the context *outside* the group.

**Literals gate a case-sensitive prefilter.** `Ruleset.candidates` (via
`prefilter.matches`) runs `bytes.Contains` for each literal and only evaluates rules whose literal is
present, so a blob without it never reaches the regex. `bytes.Contains` does not
fold case, so `"cloudinary"` alone will never see `CLOUDINARY_API_SECRET` —
list every spelling the wild uses. A rule with no literals runs against every
blob, which is why the entropy pass is kept separate; do not leave literals off
to "be safe".

**Anchor on context when the value has no shape of its own.** A Cloudinary API
key is fifteen digits and nothing more. Matched alone it fires on every
timestamp and identifier in a codebase. Requiring the setting name is not an
optimisation there — it is the entire signal. By contrast `AKIA…` or `ghp_…`
carry their own prefix and need no context.

**Keywords are a second gate, on the surrounding text.** `keywordsPresent`
requires *every* keyword to appear (case-insensitively) within ±400 bytes of the
match (`keywordWindow`), which can span several lines. Use them to
rescue a pattern that is otherwise too generic to enable.

Every match is reported, up to `maxMatchesPerRule` (64) per rule per blob
(`secretIndexes` in `rule.go`); a value an earlier rule already claimed is not
reported again.

### Severity

Judge consequence, not confidence — the enricher raises it again for a leak
still reachable from HEAD and for cloud/payment blast radius.

| | |
|---|---|
| `critical` | full account control: private keys, Stripe live, Cloudinary secret |
| `high`     | broad API access: AWS, GitHub PAT, Google API key |
| `medium`   | narrow or host-scoped access |
| `low`      | identifies an account but cannot act: Stripe test keys |

## Verify before claiming it works

Run the real detector over the real file. A throwaway `main.go` under `cmd/` is
the quickest route; delete it afterwards.

```go
package main

import (
    "fmt"; "os"
    "github.com/mservice/gitscanner/internal/scan"
    "github.com/mservice/gitscanner/internal/secrets"
)

func main() {
    data, _ := os.ReadFile(os.Args[1])
    for _, m := range scan.NewDetector(scan.BootstrapRuleset()).Detect("config.properties", data) {
        fmt.Printf("rule=%-28s provider=%-12s sev=%-9s %s\n",
            m.RuleID, m.Provider, m.Severity, secrets.Mask(m.Secret))
    }
}
```

`secrets.Mask` keeps the output safe to paste. Confirm the named rule fires and
that `generic-high-entropy` no longer claims the same value — `overlapsNamed`
should suppress it, and if it does not, the capture group is probably wrong.

Then `go build ./... && go vet ./... && go test ./test/unit/internal/scan/...`.

## Rescanning an already-scanned repository

Findings are idempotent on `(fingerprint, repository_id)`, so a rescan does not
duplicate them — and for a long time it also meant an improved rule was
**invisible** on anything already scanned, because `ON CONFLICT DO NOTHING`
discarded the better classification.

`InsertFinding` now has an update arm for exactly this: a *named* rule may
replace the unattributed catch-all. The guard is deliberately narrow —
`findings.provider_slug = '' AND EXCLUDED.provider_slug <> ''` — so one provider
can never overwrite another. A rescan reports these as `reclassified` rather
than as new findings, and the counters stay honest.

The gap to know about: analytics rollups already counted under
`provider=unmapped` are **not** corrected, because the dedupe table keys on
finding id and rejects the replay. A clean rollup needs the data wiped and a
fresh scan.

## Placeholder false positives

`isPlaceholder` in `detect.go` drops template values. It missed
`your-cloudinary-api-secret` because the pattern required `your` immediately
before the noun; config templates spell the whole thing out. The arm now allows
words in between.

The filter runs at *detection* time, so tightening it does not retroactively
remove findings already stored — those only clear on a wipe and rescan.
