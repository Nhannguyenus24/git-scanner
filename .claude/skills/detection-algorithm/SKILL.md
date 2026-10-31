---
name: detection-algorithm
description: The secret-detection algorithm in depth — how GitScanner decides that a string is a credential, the maths behind each decision (Shannon entropy and its ceilings, identifier segmentation, assignment-key classification), the exact order of every gate with a worked trace, what each gate cost or saved on the real corpus, and how to change one without regressing. Use this when studying or explaining how detection works, when asked why a string was or was not reported, when findings are junk ("rác", false positive, noise) or a real key is missed, and before editing internal/scan/detect.go or bootstrap_rules.go. For the runtime pipeline around it (clone, attribution, storage, probing) see detection-engine; for adding one provider's rule see detection-rule.
---

# The detection algorithm

Detection answers one question per string: *is this a credential?* Getting it
wrong in one direction buries real leaks in noise; in the other, it misses them.
This skill explains how the answer is computed today and why each step exists.
The code is `internal/scan/detect.go` (gates) and `bootstrap_rules.go` (rules).

## 1. Two detectors with opposite trade-offs

| | Named rules | Generic high-entropy pass |
|---|---|---|
| Finds | credentials whose **shape** is known (`AKIA…`, `ghp_…`, `sk-ant-…`) | opaque strings no rule describes |
| Precision | high — the shape *is* the evidence | low by nature — "random-looking" fits a lot of things |
| Recall | only what it has a rule for | anything long and random enough |
| Provider | set by the rule | empty; becomes a candidate for a new rule (except split DB variables → `db-split-env`) |
| Tuned by | writing rules (`detection-rule`) | **filters** — almost everything in this skill |

The generic pass is where the junk comes from, so it is where the algorithm is.
A named rule always runs first; a value it claims is never re-reported as
generic (`overlapsNamed`).

## 2. Named rules

For each blob:

1. **Literal prefilter.** Each rule lists literals its pattern always contains
   (`AKIA`, `T3BlbkFJ`, `cloudinary`). Only rules whose literal occurs in the
   blob run their regex. The check is `bytes.Contains` — **case-sensitive**, so
   a rule must list every casing the wild uses (`giphy`, `GIPHY`, `Giphy`).
2. **Regex** (Go RE2 — linear time, no catastrophic backtracking). The
   `(?P<secret>…)` group marks the credential; context around it stays outside.
3. **Keywords** (optional) must all appear, case-insensitively, within ±400
   bytes of the match (`keywordWindow`) — so `cloudinary.config({...})` and
   `recaptcha:` blocks spanning several lines still qualify.
4. **`isPlaceholder`** drops template values (`your-…-key`, `xxxx`, `changeme`,
   `example`, repeated characters).
5. **`namedSuppressed`** drops published example keys (`isFakeValue`), test
   fixtures with a sequential run of 8+, a PEM header with no body, placeholder
   URL passwords and Firebase client keys; host-scoped URL rules also drop URLs
   whose every host is dev-local (`localURL`).

Every match is reported, up to 64 per rule per blob (`secretIndexes`).

How a rule earns its precision — pick the strongest anchor available:

| Anchor | Example | Why it works |
|---|---|---|
| Fixed prefix | `ghp_`, `AKIA`, `sk-ant-`, `1//0` | the provider stamps it on every key |
| Internal marker | OpenAI `T3BlbkFJ` | base64 of "OpenAI", present in every format; far rarer than `sk-` |
| Fixed layout | Resend `re_` + 8 + `_` + 24 | a 3-char literal alone would match `re_compile` |
| Setting name | `CLOUDINARY_API_SECRET=`, `GIPHY_API_KEY=` | the value has no shape of its own; the name is the whole signal |
| Word boundary | `\bhf_` | stops a short prefix matching inside identifiers (`pathf_`) |

## 3. The generic pass, gate by gate

### Tokenising

A candidate token is `[A-Za-z0-9+/_-]{20,}={0,2}`: a run of base64/base64url
characters, at least 20 long, with optional `=` padding. It stops at `.`, so a
JWT arrives as three tokens and a hostname splits at its dots.

### Gate order (as in `genericHighEntropy`)

Cheap, value-only checks first; context-dependent ones last.

```
for each token t at [lo, hi) in blob:
    if seen(t) or overlapsNamed(t):               skip   # already reported
    if H(t) < 4.0 or isPlaceholder(t)
       or isFakeValue(t):                         skip   # not random enough / template
    if text before t ends with "${":              skip   # ${VAR} reference
    if data-URI or image magic (iVBOR…, /9j/):    skip   # embedded image
    if t is a URL host/path segment:              skip   # CDN path, asset hash
    if looksLikeIdentifier(t) and not assigned
       to a secret-named setting:                 skip   # camelCase, paths, SNAKE
    if isTestPath(path):                          skip   # fixtures
    if publicIdentifier(t, text after t):         skip   # pk_test_…, OAuth client id
    key := assignmentKey(96 bytes before t)
    switch classifyKey(key):
        public:  skip                                    # client_id, site-key, …_id
        secret:  keep going                              # api_secret, DB_PWD
        unknown: require secretKeyword in ±160 bytes
    if hostScopedKey(key) and every host within
       ±4 lines is dev-local:                     skip   # DB_HOST=localhost nearby
    if exampleLine(window):                       skip   # example =, e.g.
    if part of a JWT, base64-JSON, or a JWT
       signing-secret context:                    skip   # policy, see §3.8
    report t as generic-high-entropy
```

Example/template files (`.env.example`, `*.sample.*`, `examples/`) skip the
whole generic pass before this loop.

### 3.1 Shannon entropy

`H(t) = −Σ p(c)·log₂ p(c)` over the characters of `t`, in bits per character.
It measures how evenly the characters are spread, **not** how secret a string
is. The threshold is 4.0. Real numbers:

| String | Length | H |
|---|---|---|
| random base64 | 40 | 4.72 |
| random base64 | 20 | 4.02 |
| camelCase identifier `getChangePasswordRequestSchema` | 30 | 4.00 |
| random UUID hex | 32 | 3.91 |
| random hex | 64 | 3.84 |
| English words run together | 29 | 3.78 |
| `aaaa…` | 24 | 0 |

Two consequences worth remembering:

- **H can never exceed log₂(length) or log₂(alphabet size).** A 20-character
  token tops out at log₂20 = 4.32, so it passes only with ~17 distinct
  characters. **Hex tops out at log₂16 = 4.0 and essentially never passes** (only a
  perfectly uniform hex string reaches exactly 4.0, which the strict `<` lets
  through) — a 32- or
  64-hex secret is invisible to the generic pass. That is a known recall gap
  (§6), not a bug in the arithmetic.
- **Entropy alone cannot tell identifiers from keys.** A long camelCase name
  scores the same as a short random key. That is why gates 3.2–3.4 exist.

### 3.2 `looksLikeIdentifier` — words versus noise

Split the token at separators (`_ - / . +`), case changes (`fooBar`,
`HTTPServer`) and letter/digit boundaries. Words produce long segments; random
strings fragment into runs of one or two characters.

- identifier if **average segment length ≥ 3.5** and **vowel ratio ≥ 0.25**;
- needs at least 2 segments (a single lowercase run is left to entropy);
- rescued when the token is assigned straight to a secret-named setting —
  `app.secret=mySuperSecretSigningKey…` is a hardcoded secret, wordy or not.

`getChangePasswordRequestSchema` (H = 4.00, so it gets past entropy) →
`get|Change|Password|Request|Schema`, average 6, vowels 0.33 → identifier. `Zx9Kq2Lp7Wm4Rt8Nv3Yb` → average ~1.4 → random.

### 3.3 Assignment-key classification — the decisive gate

The strongest evidence about a value is **the name it is assigned to**.
`assignmentKey` reads it from the text just before the value, across formats:

| Form | Key |
|---|---|
| `DB_PASSWORD=…` · `db.password: …` · `"api_key": "…"` · `key := …` · `key => …` | the identifier before the operator |
| `${RESEND_API_KEY:re_…}` (Spring default) | `RESEND_API_KEY` |
| `cloudinary.config("…", "…")` (positional) | none |

`keyWords` splits the key into lower-case words across snake_case, kebab-case,
dotted.paths and camelCase. `classifyKey` then decides, **in this precedence**:

1. **public** if it says `public`, `publishable`, `pub`, or site + key
   (`site-key`, `sitekey`) — the public half of a pair is named after the
   secret half, so this must win over the word `key`;
2. **secret** if it says `secret`, `password`, `passwd`, `pwd`, `token`,
   `credential(s)`, `private`, `apikey`, or `api` + `key`;
3. **public** if it ends in `id` or says `user`, `username`, `email`, `url`,
   `uri`, `host`, `endpoint`, `name` — identifiers, not credentials;
4. **unknown** otherwise (`key`, `value`, `content`, no key at all).

A **public** key drops the value even if the line says "secret"
(`AWS_KEY_ID=… # secret`). A **secret** key keeps it even with no keyword
nearby. Only **unknown** falls back to the proximity test (3.4).

### 3.4 Proximity keyword (fallback only)

With no informative key, a token needs a secret-naming word within ±160 bytes:
`secret token password passwd api_key access_key private_key credential
authorization auth_token auth_key`.

Two lessons are baked into that list:

- **No bare `auth`.** It matched `OAUTH`, `author`, `AuthenticationFilter`;
  `GOOGLE_OAUTH_CLIENT_ID=<public id>` was reported as a secret because of it.
- **No short words as substrings.** `pwd` occurs by chance inside base64 and URL
  parameters (`?__cft__[0]=…` in a SQL dump). Short words are matched only as
  whole words, by `classifyKey`.

The window is ±160 bytes, not the line: SQL dumps and minified bundles put
megabytes on one line, and a whole-line test let the word "JWT" in an unrelated
article decide a token 4 MB away.

### 3.5 Public-by-design values

Some credentials are *meant* to be public, so finding them is not a leak:

- Stripe publishable keys `pk_live_…` / `pk_test_…` (shipped to browsers);
- Google OAuth client ids `<digits>-<hash>.apps.googleusercontent.com` (sent in
  every authorization URL).

### 3.6 Test files and examples

The generic pass skips test sources and fixture directories (`_test.go`,
`*Test.java`, `*.spec.*`, `*.test.*`, `test_*.py`, `test/`, `tests/`,
`__tests__/`, `spec/`, `specs/`, `fixtures/`, `__fixtures__/`, `__mocks__/`,
`*Tests.java`, `*Test.kt`) and example/template files.
**Named rules still run there** (they skip the generic gates but still pass
`isPlaceholder` and `namedSuppressed`): a real key with its own shape pasted into a
test is still a leak.

### 3.7 Encodings that look random

- `${VAR}` references, `data:…;base64,` URIs, PNG/JPEG/GIF magic, URL path
  segments — all random-looking, none credentials.

### 3.8 JWT context — a policy, not a fact

Any JWT segment, base64url-encoded JSON (`eyJ…` headers quoted in docs), and JWT
signing secrets (`jwt` starting a word nearby, a `Jwt*` file, a `jwt:` YAML
parent ≤ 4 lines up) are skipped. In this corpus every such hit was a tutorial
default (`mySecretKey…`) or a doc sample. A provider whose API key *is* a JWT
(Supabase `service_role`) needs a named rule — the generic gates never apply to named rules.

## 4. Worked traces (from the corpus, values masked)

| Input | Path through the gates | Result |
|---|---|---|
| `GOOGLE_OAUTH_CLIENT_ID=5843…dg7j.apps.googleusercontent.com` | entropy ✓ → `publicIdentifier` (client-id shape + suffix) | **dropped** (was reported before: OAUTH ⊃ auth) |
| `STRIPE_API_KEY = 'pk_t…ylD1'` | `publicIdentifier` (`pk_test_`) | **dropped** |
| `"password": "5u89…HV3N"` in `config/config.json` | key `password` → secret | **reported** |
| `hash-secret: GV6V…F792 # VNPay Hash Secret` | key `hash-secret` → secret | **reported** |
| `resend.api-key=${RESEND_API_KEY:re_7…ZCeM}` | named rule `resend-api-key` | **reported, attributed** |
| `<a href="…?__cft__[0]=AZZh…4nAI">` in a SQL dump | no assignment key → unknown → no whole secret word nearby | **dropped** (let through for a while by `pwd` as a substring) |
| `ms := detect(".env", "APP_SECRET=Zx9K…")` in `detect_test.go` | `isTestPath` | **dropped** |

## 5. Measured effect

Unique findings (repo + rule + masked value) over every blob of the 51 dev
repositories, after each round of changes:

| Change | Total | Generic |
|---|---|---|
| Starting point | 303 | 273 |
| Identifier, path, PEM-block and lockfile filters | 81 | 51 |
| JWT / example context, ±160 window, Giphy rule | 61 | 29 |
| Assignment-key classification, public ids, test files, stricter keywords, Resend + Google refresh-token rules | 53 | 19 |

Named-rule results were unchanged in every round except where a new rule was
added on purpose. The 19 generic findings left are DB passwords, reCAPTCHA
secret keys, a VNPay hash secret and Cloudinary `api_secret` values — real
secrets that no named rule attributed at the time of measurement. reCAPTCHA,
Cloudinary-config and split-DB-variable rules have since been added — re-measure
before quoting these numbers.

## 6. Changing a gate safely

1. **Start from the junk, not the code.** List the generic findings with masked
   previews and paths over the corpus; group them by cause.
2. **Make the smallest change that removes a whole group**, and prefer evidence
   about the value's *role* (its key, its file) over its *look*.
3. **Diff the corpus before and after** (`detection-engine` → *Measuring a
   change*). Read every dropped line: it must be junk. Read every added line:
   there should be none you did not intend. Named results must not move.
4. **Write a test per real shape**, then check it **fails on the old code**
   (copy the files aside and restore them — never `git stash`/`checkout` a file
   with uncommitted work). A test that passes on both is only a regression
   guard; know which kind you wrote.
5. **Watch for vacuous tests.** A new filter can make an old test pass for a new
   reason — when test files were excluded, two cases named `AuthTest.java`
   stopped testing the identifier and JWT filters until renamed.

Pitfalls that have bitten this code:

- substring keywords (`auth`, `pwd`) — match inside unrelated words;
- whole-line context on minified or dumped files;
- a case-sensitive literal prefilter with only one casing listed;
- public halves of credential pairs named like the secret half (`site_key`).

## 7. Known limits and next steps

- **Hex secrets are never generic findings** (entropy ceiling 4.0). Fix:
  normalise entropy by the alphabet the token actually uses
  (`H / log₂|alphabet|`), with a matching threshold, and re-measure the junk.
- **Two notions of "secret setting"**: the identifier rescue still uses the
  `secretAssignment` regex while the decisive gate uses `classifyKey`. Unify on
  `classifyKey` and re-measure.
- **Gates are boolean.** A scored model (entropy margin + key class + file kind +
  keyword distance → one confidence) would rank findings instead of only
  keeping or dropping them.
- **Positional arguments carry no key** (`cloudinary.config("…", "…")`), so they
  fall back to proximity.
