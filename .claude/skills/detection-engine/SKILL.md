---
name: detection-engine
description: How GitScanner decides what is a leaked credential today — the order of every gate a blob and a token pass through, which filter drops what and why, how a finding is attributed, stored and re-scored, and how to measure a detection change against real repositories before shipping it. Use this when asked why something was or was not reported, when findings look like junk ("rác", false positive, noise), when tuning the generic high-entropy pass, or before changing anything in internal/scan/detect.go. For adding one provider's rule, use detection-rule instead.
---

# How detection works

Everything below lives in `internal/scan/` — `detect.go` (the gates),
`bootstrap_rules.go` (the named rules), `engine.go` (walk, attribution,
finding construction), `git.go` (clone and history). Read this before editing
any of them: most gates exist because of a specific false positive or a
specific miss, and the reason is easy to undo by accident.

## The pipeline, in order

### 1. Per repository (`engine.go`, `git.go`)

1. `git clone --bare` — branches and tags, never `refs/pull/*` (see CLAUDE.md).
2. `rev-list --objects --all` lists every blob in history with *one* path.
   Blobs are deduplicated by OID: identical content is scanned once.
3. `ls-tree -r HEAD` gives the blobs still live, for `reachable_from_head`.
4. Each blob ≤ 10 MB goes through `Detector.Detect(path, blob)`.
5. After the walk, findings are **held**, then attributed (step 4 below), then
   emitted to the sink. Nothing is emitted mid-walk.

### 2. Per blob (`Detect`)

| Gate | Drops | Why |
|---|---|---|
| `isBinary` | NUL in first 512 bytes | same heuristic as git |
| `deniedPath` | `vendor/`, `node_modules/`, `testdata/`, `.git/`, `.idea/`; `package-lock.json`, `npm-shrinkwrap.json`, `pnpm-lock.yaml`, `go.sum`, `*.lock`; `*.min.js`, `*.map`, `*.svg`, `*.drawio` | vendored code, integrity hashes, IDE metadata, diagrams |
| named rules | — | see 3 |
| `exampleFile` | **generic pass only** for `.env.example`, `*.sample.*`, `*.template`, `*.tmpl`, `*.dist`, `example(s)/`, `sample(s)/` | templates hold stand-ins; a *named* rule still runs there because a real key pasted into a template is still a leak |
| generic pass | — | see 4 |

### 3. Named rules (`bootstrap_rules.go`)

For each rule: **literal prefilter** (`bytes.Contains`, case-sensitive — list
every casing) → regex → the `(?P<secret>…)` span → `keywords` on the line →
`isPlaceholder`. Only the **first** match per rule per blob is reported.

The maths and reasoning behind each gate — entropy ceilings, identifier
segmentation, key classification, worked traces and measured effect — are in the
`detection-algorithm` skill.

Catalogue today, by how it is anchored:

| Anchored on the value's own prefix | Anchored on the setting name (no prefix of their own) |
|---|---|
| aws `AKIA/ASIA`, github `ghp_/gho_/ghu_/ghs_/github_pat_`, stripe `sk_live_/sk_test_`, google `AIza` (also Gemini), `GOCSPX-`, slack `xox?-` and webhook URL, sendgrid `SG.`, cloudinary URL, smtp URL, PEM private key block (whole block), openai (`T3BlbkFJ` marker), anthropic `sk-ant-`, openrouter `sk-or-v1-`, groq `gsk_`, huggingface `hf_`, google refresh token `1//0`, resend `re_` (fixed layout), replicate `r8_`, perplexity `pplx-`, xai `xai-`, langsmith `lsv2_`, pinecone `pcsk_`, giphy request URL | cloudinary api key/secret, deepseek, cohere, mistral, azure-openai, giphy (`GIPHY…=` including `… \|\| 'fallback'`) |

### 4. The generic high-entropy pass

Catch-all for opaque secrets no rule names. Its findings have an **empty
provider**; the enricher clusters them into provider candidates. A token is
`[A-Za-z0-9+/_-]{20,}={0,2}` — it stops at dots, so a JWT arrives as three
tokens. Gates, in order:

| # | Gate | Drops | Real case it came from |
|---|---|---|---|
| 1 | dedupe / `overlapsNamed` | a value a named rule already claimed | — |
| 2 | Shannon entropy < 4.0, `isPlaceholder` | prose, `your-…-key`, `xxxx`, `changeme`, repeats | config templates |
| 3 | preceded by `${` | variable references | `${GF_SECURITY_ADMIN_PASSWORD}` |
| 4 | `base64,` before it, or PNG/JPEG/GIF magic | embedded images | notebook output cells |
| 5 | `inURLPath` | host/path segments of a URL (userinfo and `?key=` still checked) | CDN asset hashes in a SQL dump |
| 6 | `looksLikeIdentifier` unless directly assigned to a secret setting | camelCase, SCREAMING_SNAKE, package paths | `UserAuthenticationFilter`, test method names |
| 7 | `isTestPath`, `publicIdentifier` | test sources and fixtures; Stripe `pk_live_/pk_test_`, Google OAuth client ids | `detect_test.go` fixtures, `GOOGLE_OAUTH_CLIENT_ID=` |
| 8 | **assignment-key class** (`assignmentKey` → `classifyKey`): public key → drop, secret key → keep, unknown → needs a whole secret word (`secretKeyword`) within **±160 bytes** | values assigned to `client_id`, `site-key`, `…_id`, `username`; tokens with no secret word nearby | client ids reported because OAUTH contains "auth" |
| 9 | `exampleLine` | `example`, `examples`, `e.g.`, `for instance` near it | `@Schema(example = "eyJ…")`, `**Example:**` |
| 10 | `inJWT`, `isJSONSegment` | any segment of a JWT; base64url JSON quoted alone | sample tokens in API docs and tests |
| 11 | `jwtSecretContext` | JWT signing secrets: `jwt` starting a word in the window, a file named `Jwt*`, or a `jwt:` YAML parent ≤ 4 lines up | `jwt.secret=mySecretKey…`, `@Value("${jwt.secret:…}")` |

Gates 10–11 are a **policy decision**, not a claim that JWT material never
matters: every hit in the scanned repositories was a tutorial default or a doc
sample. A provider whose API key *is* a JWT (Supabase `service_role`) needs a
named rule; the exclusion only applies to the generic pass.

The ±160-byte window (`contextAround`) matters on its own: SQL dumps and
minified bundles put megabytes on one line, and a whole-line keyword check let
the word "JWT" in an unrelated article decide a token 4 MB away.

### 5. From match to finding (`engine.go`, `secrets`)

- **Fingerprint**: HMAC-SHA256 of the secret — the dedupe key per repository.
- **Masked preview**: first 4 + `…` + last 4. The raw secret goes no further.
- **Metadata**: `secret_length`, `charset_class`, `env_var`, `host` — shape,
  never value.
- **Attribution**: one `git log --all --reverse --raw` pass for the blobs that
  produced findings; the first commit whose diff adds the blob gives
  `commit_sha`, author, date and the path *at that commit*. A failed lookup is
  logged and the finding is kept unattributed.

### 5b. Live validation (off by default)

Two layers, both in the engine because it is the only place the plaintext secret
exists:

- **Offline extraction** (always on, no network, `extractOffline` in `probe.go`):
  decodes facts the credential carries by itself. Today: the AWS account id baked
  into an `AKIA…` key id, and a Stripe key's live/test mode. Written to metadata
  (`aws_account_id`, `stripe_mode`).
- **Liveness probe** (`PROBE_SECRETS=true`, default false): one identity call per
  *named* finding — GitHub `/user`, Stripe `/v1/account`, Slack `auth.test`,
  OpenAI `/v1/models`, SendGrid `/v3/scopes`, Giphy trending. Implemented in
  `internal/probe`, injected into the engine via `WithProber`. Result in metadata:
  `probe_status` (live/dead/unverified/error) and non-secret identity
  (`probe_owner`, `probe_account`, `probe_team`). The UI shows a "LIVE — rotate"
  badge.

**Rules this must keep.** The probe authenticates to third-party providers with a
found credential, so: it is **off unless explicitly enabled**, and only for
targets the operator owns (the ownership gate — see plan.md and `entity.go`'s
"probe results require the ownership gate"); it is **identity/liveness only** and
must never enumerate, read or modify account resources; and it runs **only inside
the engine**, so the secret itself still never travels onward (plan.md §6) — only
the boolean outcome and non-secret identity do. The generic (provider-less) pass
is never probed.

### 6. Storage and afterwards

- `InsertFinding` is idempotent on `(fingerprint, repository_id)`. Its update arm
  only lets a **named** rule replace the catch-all (see CLAUDE.md).
- `BackfillAttribution` fills commit fields on older rows (`commit_sha = ''`).
- The enricher raises severity by one for `reachable_from_head` and one for
  cloud/payment providers, and turns unmapped findings into candidates.

## Measuring a change — do this before claiming it works

Unit tests prove the cases you thought of. What matters is the effect on real
repositories, and the dev database's repositories are the corpus:

```bash
S=<scratchpad>; mkdir -p $S/repos
PGPASSWORD=postgres123 psql -h localhost -U postgres -d app_db -At -c \
  "SELECT owner||'__'||name, clone_url FROM repositories" |
while IFS='|' read n u; do [ -d $S/repos/$n ] || git clone --bare -q "$u" $S/repos/$n & done; wait
```

Then a throwaway `cmd/zprobe/main.go` that runs `scan.NewDetector(scan.BootstrapRuleset())`
over every blob (`rev-list --objects --all` + `cat-file --batch`) and prints
`repo  rule  secrets.Mask(secret)  path:line`. Run it on `main` and on the
branch, `sort -u` on the first three columns, and `comm` the two:

- every **dropped** line should be junk you meant to drop — read each one;
- every **added** line should be a rule you added;
- named-rule lines should not change unless that was the point.

To see *why* a token was reported, print its line with every run of 13+
characters masked (`abc…[42]`). Never print the value. Delete `cmd/zprobe`
before committing.

When checking that a new test fails on the old code, copy the files aside and
restore them from the copy. Do **not** `git stash` or `git checkout` a file with
uncommitted work: it takes all of the uncommitted change with it, not just the
part being tested.

## Known gaps

- **The local-config filter is not wired.** `LocalOnly` and `LocalURLCredential`
  in `localfilter.go` are implemented and unit-tested, but nothing in the
  pipeline calls them, so `postgres://u:p@localhost` is still reported.
- First match per rule per blob: two keys of one provider in a file → one finding.
- `envVarIn` takes any word before a colon, so `env_var` and candidate names pick
  up YAML keys and prose (`style`, `description`).
- Cloudinary's `api_secret: '…'` inside `cloudinary.config({...})` has no
  `cloudinary` on the same line and is caught only by the generic pass.
- Probe results are written at insert time; an already-stored finding is not
  re-probed unless the row is reclassified or re-inserted, so turning probing on
  and rescanning is how existing findings get a live/dead verdict.
- Filters run at detection time; stored rows only change on a rescan, and rows a
  new filter would drop stay until a wipe (`wipe-data`).
