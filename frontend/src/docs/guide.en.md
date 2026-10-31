# Using GitScanner

GitScanner clones a repository's full history, reads every unique file version
(blob) once, and reports the credentials it finds — in the current tree and
anywhere in the past. A key deleted three years ago is still readable by anyone
who clones the repository, so it is still a leak until it is revoked.

This guide walks through the dashboard page by page. The short version:

1. Check that the dot beside the API address in the top bar is green.
2. Press **New scan** and give it a repository, a user or an organisation.
3. Watch the scan's page until it finishes.
4. Work through **Findings**: rotate what is live, mark what is not a leak.
5. Tell the owners with a disclosure mail from **Repositories**.

---

## The screen

### Top bar

- **API address** — the server everything on screen comes from. The dot beside
  it is grey while checking, green when the API answers and red when it cannot
  be reached. Type your own host if you are not running the default
  `http://localhost:8080`; it is remembered in this browser.
- **Command box** — opens the command palette (see below).
- **Help** — this page.
- **Light / dark** — flips between light and dark paper.
- **Language** — English, Tiếng Việt, Italiano, 中文, Español, Deutsch and
  Français. Everything, including this guide, switches with it.

### Sidebar

Every page is one click away. Three entries carry a live count: **Findings**
(open findings), **Scans** (scans still running) and **Unmapped** (shapes
awaiting a provider rule). Collapse the sidebar to icons only with the arrow at
its foot or with `Ctrl+[` (`⌘[` on a Mac). On a phone the sidebar becomes a
bottom bar; the less used pages sit under **More**.

### Keyboard

- `Ctrl+K` / `⌘K` — open or close the command palette, from anywhere.
- `/` — open the command palette.
- `N` — start a new scan.
- `Ctrl+[` / `⌘[` — collapse or expand the sidebar.
- `Esc` — close a dialog, drawer or palette.

`N` and `/` are ignored while you are typing in a text box, so they never eat a
keystroke meant for a filter.

### Command palette

One box for everything the dashboard can do from the keyboard. Type to narrow
the list, move with `↑` `↓` (or `Ctrl+N` / `Ctrl+P`), run with `Enter`. It
offers:

- **Actions** — start a new scan; export the current view.
- **Go to** — every page.
- **Filters** — the finding presets and the filters you saved, applied in one
  step.
- **Appearance** — light or dark, follow the system, each theme, density,
  motion, sidebar, and reset.
- **Language** — switch the interface language.

Anything else you type becomes **Search findings for “…”**, which opens Findings
with that text as the search filter. The palette keeps no history on purpose: a
search box that remembers is a place a pasted credential could come to rest.

### Introduction

The first time the dashboard opens it offers a seven-step introduction. Skip it
whenever you like; replay it later from **Settings → Introduction**.

---

## Starting a scan

Press **New scan** (sidebar, Overview or Scans page, or `N`). Choose a target:

- **Repository** — one HTTPS clone URL, for example
  `https://github.com/owner/repo`. SSH URLs and URLs with a user name or token
  in them are refused.
- **User** — every repository a GitHub user owns. A login is letters, digits
  and hyphens, 1–39 characters.
- **Org** — every repository a GitHub organisation owns.
- **Random** — a random sample of public repositories, for testing detection
  only. Enter how many, or tick **Run forever** to keep sampling until you pause
  or cancel it.

For User, Org and Random, tick **Include forked repositories** to scan forks as
well; they are skipped by default because a fork mostly repeats its parent.

Press **Start Scan** or `Enter`. The scan is queued in the background and the
dashboard opens its detail page.

---

## Following a scan

### Scans list

**Scans** lists every job the API has accepted, newest first. Switch between
**All**, **Active** and **Finished**. Each row shows the state, the target,
progress, blobs read, new findings, when it started and how long it ran. Click
a row to open it; the icons at the end rescan or delete a finished job.

A job is in one of six states: **queued**, **running**, **paused**,
**completed**, **failed** or **cancelled**.

### Scan detail

The header names the target, the kind of scan, when it was submitted and how
long it has run. The buttons change with the state:

- **Pause** (running) — workers stop picking up new repositories; nothing
  already found is lost. **Resume** carries on from where it stopped.
- **Cancel** (any unfinished job) — stops the job for good.
- **Rescan** (finished) — queues the same target again as a new job.
- **Delete** (finished) — removes the job **and every finding it recorded**.
  This cannot be undone.

Below the header:

- **Counters** — repositories done of the total, blobs scanned, and new
  findings (first seen by this job).
- **Progress and time left** — an estimate appears once there is enough to go
  on; until then the counters are the truth.
- **Stages** — Discovering repositories → Scanning repositories (clone, walk
  history, detect) → Classifying providers → Finalising analytics.
- **Live activity** — streamed from the workers while the scan runs.
- **Summary** — when the job completes: how many findings in how many
  repositories, how many are high or critical, how many are still reachable
  from the default branch, and which repositories are affected.
- **Findings recorded by this scan** — the list itself, with a link to open it
  in Findings.

### Live activity

The live panel shows what each worker is doing right now. Its status reads
**connecting**, **live**, **reconnecting**, **ended** or **unavailable**; if the
server does not stream, the counters above still refresh every few seconds.

- **Discovery** — for User and Org scans, the listing of the account's
  repositories, page by page. *Waiting on the GitHub rate limit* is a pause,
  not a failure.
- **In progress** — each repository and the step it is on: **clone** (bare:
  branches and tags only), **read history** (every unique file version, with
  the running count of candidate matches), **attribute** (tracing each match to
  the commit that introduced it), **probe** (live validation, if enabled) and
  **store**.
- **Just finished** — repositories completed a moment ago, with their new
  findings, blobs read and time taken.
- **Activity log** — the full stream of events; **Show more** expands it.

A repository whose attempt fails is retried automatically and shows *attempt
2*, *attempt 3*… A repository that fails its last attempt is **skipped**; the
rest of the job carries on.

### Why a scan can find nothing new

A credential is recorded once, by the first scan that found it. Scanning the
same repository again finds the same keys but records no *new* findings — they
stay attributed to the earlier scan. Open **Findings** filtered to that
repository to see them all.

---

## Overview

The headline page, refreshed every ten seconds (the **Live** marker shows the
interval).

- **Security posture** — a score out of 100. Press **How it is calculated** to
  see every penalty (below).
- **Headline counts** — open findings, critical, live at HEAD, exposed
  repositories, active scans, unmapped shapes.
- **Open by severity**, **Open by provider**, **Triage** — breakdowns. Every
  tile and bar is a link into a filtered Findings list.
- **Recent activity** — scans finishing or failing, new findings, disclosures
  sent.
- **Recent scans** — the latest jobs and their progress.

### Layouts

**Layout** switches between four arrangements: **Everything**, **Daily
triage** (counts and breakdowns, no score), **Executive summary** (score,
headline numbers, what changed) and **Security operations** (activity and
running scans first). **Customise** lets you move a panel up or down with the
arrows, hide it with the eye, and bring a hidden one back from the list below.
Layouts are saved in this browser.

### How the posture score is calculated

The score starts at 100 and loses points per factor. Each penalty saturates, so
the first few findings cost the most and no single factor can run away:

- **Critical findings** — up to 30 points.
- **Reachable from HEAD** — up to 25 points. The sharpest signal: anyone who
  clones the default branch today gets the credential.
- **High findings** — up to 15 points.
- **Exposed repositories** — up to 15 points, by the share of scanned
  repositories holding an open finding.
- **Unresolved share** — up to 10 points, by the share of findings still open.
  A closed finding costs nothing.
- **Owners not told** — up to 5 points, for exposed repositories with no
  disclosure sent.

85 and above reads **Holding**, 65–84 **Needs work**, 40–64 **Exposed**, below 40
**Critical**. With nothing scanned there is no score at all rather than a
misleading 100. The score is worked out in the browser from the server's
whole-database counters, never from one page of findings.

---

## Findings

The full list of every credential recorded, 50 rows to a page.

### Filtering

- **Search** — path, rule, provider or repository.
- **Severity**, **Provider**, **Status** (Open, Rotated, False positive,
  Ignored).
- **Exposure** — at HEAD, history only, or confirmed live.
- **Order** — default, newest detected, oldest commit, or severity.
- **One repository** / scan — appears as a chip when you arrive from a
  repository or a scan; remove it to see everything.

The filter lives in the URL, so a filtered view can be bookmarked, shared or
reopened after a reload. **Clear filters** resets it.

Exposure and Order are worked out in the browser. With either one on, the page
loads up to 500 findings from the server and refines those; if there were more,
it says so above the table rather than showing a misleading total.

### Presets and saved filters

The **Filters** menu offers ready-made views:

- **Critical, still open** — rotate these first.
- **Reachable from HEAD** — readable by anyone who clones today.
- **Confirmed live** — the probe got a valid response.
- **Unattributed** — no provider rule claimed them.
- **Recently discovered** — newest detections first.
- **Longest unresolved** — oldest commit first.

**Save the current filter** stores the filter you have set under a name; it can
be renamed or deleted from the same menu and is also offered in the command
palette. Saved filters live in this browser only and hold filter fields only —
never a search text that looks like a pasted secret.

### Reading a row

- **Severity** and the **provider** the credential belongs to (or
  *unmapped*).
- **Credential** — a masked preview. The real value never leaves the engine
  unless you have explicitly turned unmasking on (see Settings).
- **Repository** and **Location** — the file and line, linked to the file on
  GitHub.
- **Exposure** — `at HEAD` means it is still in the current tree; `history`
  means it was removed but is still reachable by cloning. *Commit unknown —
  rescan to attribute* means the introducing commit was not traced; a rescan
  fills it in.
- **LIVE — rotate** — live validation confirmed the key still works.
- **Status** and **Detected** time.

The menu at the end of a row marks a finding **Open**, **Rotated**, **False
positive** or **Ignored**.

### Finding details

Click a row to open the detail drawer. Its address goes into the URL, so
**Copy link** gives a colleague the same view; **Copy reference** copies a
short reference for a ticket. The drawer shows:

- **Credential** — masked preview, category and liveness. When the value was
  kept for an unmasked owner and masking is off, **Show** reveals it, marked
  **UNMASKED**.
- **Quota & limits** — what the provider reported during the liveness check:
  requests or tokens left, scopes, country, currency. A snapshot at scan time.
- **Why this was detected** — the rule, the provider it was attributed to, the
  length and character set, a nearby variable or host name, and the exposure.
  The explanation uses shape, length and context only, never the value.
- **Where** — file, commit, author and commit date, with a link to the line on
  GitHub.
- **Timeline** — committed, detected, probed and triage decisions.
- **What to do** — the remediation steps in order: revoke and reissue at the
  provider, check its audit log, remove the value from HEAD, decide whether to
  rewrite history, and add a pre-commit secret check.
- **Same repository** and **Same provider** — related findings, one click away.
- **Metadata** — anything else the engine recorded.

### Triage in bulk

Tick the boxes at the start of the rows, or the header box to select every row
on the page. A bar appears at the bottom with the count and the four statuses.
**Rotated** and **False positive** ask for confirmation first, because each is a
claim about the outside world — that the key was revoked, that the match was
wrong — and in bulk it is the claim that can quietly close a real leak. Open and
Ignored apply at once. **Export selected** exports only the ticked rows.

### Exporting

**Export** writes the findings currently loaded, with the filters that produced
them recorded in the file:

- **Findings as CSV** — one row per finding, for a spreadsheet.
- **Findings as JSON** — structured, for a script.
- **Incident report** — Markdown, grouped by repository.
- **Executive summary** — one page, no table.

Exports carry masked previews only; the plaintext credential is never written
to a file, in any format.

---

## Repositories

Every scanned repository, ranked by how bad its open exposure is.

- **Filter by owner/name**, and **Only with open findings**.
- **Order by** risk, open findings, last scanned or name.
- Columns: type, open counts per severity, open / total, **Risk**, **Next
  step**, **Notified** and last scanned.
- Click a repository to open its findings.

**Risk** runs from 0 (nothing open) to 100. Open findings are weighted by
severity — a critical counts ten times a low — and the total saturates, so a
repository with one critical already ranks high. Bands: Low, Medium (25+), High
(50+), Critical (75+). Hover for the counts behind it.

**Next step** names the one thing worth doing: *Rotate the critical keys*,
*Tell the owner*, *Disclosure queued*, *Disclosure failed — retry*, *Review the
remaining findings* or *Nothing open*.

### Telling the owner

The **Disclosure mail** chip at the top shows whether mail is on; it links to
Settings. While mail is on, **Notify** on a row queues a disclosure to the
repository owner, listing its findings with masked previews. The **Notified**
column then reads *sending*, *sent*, *failed* or *opted out*.

---

## Unmapped

The backlog of high-entropy secrets that no named provider rule claims,
clustered by shape. Each row shows a **guessed name** (taken from the nearest
variable or key — sometimes just a nearby word), a masked **sample**, the
**shape** (length and character set), the **context** it was found in, how
often it was **seen**, its status and when it was last seen. A shape that keeps
appearing is a provider worth writing a rule for. Findings of this kind show as
*unmapped* until a rule claims them; once one does, the next scan reattributes
them.

---

## Providers

Every provider GitScanner can detect and attribute, grouped by category (AI,
cloud, data, payments, messaging, DevOps, developer tools, other), with the
number of rules for each. Click a provider to see:

- **Why this severity** — critical is full control; high is broad API access;
  medium is narrow or host-scoped access; low identifies an account but cannot
  act on it.
- **What a leak lets someone do** — the impact for that category.
- **What the keys look like and how they are caught**, one card per rule:
  - *own prefix* — the value starts with the provider's prefix, which is
    evidence on its own;
  - *setting name* — a value with no shape of its own, reported only when
    assigned to a setting named for the provider;
  - *connection URL* — a URL with the credential inside; one pointing only at
    localhost or a compose service is treated as development config and
    dropped;
  - *PEM block* — a private key from BEGIN to END;
  - *layout only* — fixed lengths, separators or markers, kept deliberately
    narrow.
  Each card can show the layout of the key, the pattern, the text the file must
  contain, and a word that must appear nearby.
- **After a match** — placeholders (`your-…-key`, `xxxx`, `changeme`),
  documentation and sandbox keys and test fixtures are dropped; what survives is
  stored only as a fingerprint and a masked preview.

A provider shows its most serious rule. On each finding, severity rises one
level if the leak is still reachable from HEAD, and one more for cloud and
payment providers, up to critical.

---

## Analytics

A read on everything scanned so far:

- **Findings** and **Repositories** — totals, open, exposed of scanned.
- **Leak introduction timeline** — findings by the month the credential was
  committed.
- **Open by severity**, **Open by provider** (named providers plus the unmapped
  catch-all), **By application type**, **Triage status**.
- **Detection rules** — which rules fire most.
- **Live credential validation** — live, revoked, no prober, check failed.
- **Disclosure mail** — repositories contacted, and exposed ones not yet
  notified.
- **Most exposed repositories** — by the same risk score as Repositories.
- **Longest unresolved** — open findings by commit date.

The figures are all-time; there is no date range, because the API does not
offer one. The filters at the top steer the Findings links the page opens.

---

## Disclosure mail (Notifications)

Every disclosure sent to repository owners and its outcome: repository,
recipient, status (*sending*, *sent*, *failed*, *opted out*), number of
findings, when it was requested and when it was sent. Search by repository or
recipient.

- **View mail** — the message as HTML and as plain text, re-rendered from the
  findings as they are now.
- **Resend** — queues it again.

---

## Settings

### Appearance

Saved in this browser only.

- **Theme** — Paper (aged stock, teal and oxblood), Midnight (near-black
  terminal, phosphor green), Ocean (navy, cyan and blue), Signal (graphite,
  only alerts carry colour), Minimal (plain enterprise).
- **Light or dark** — Light, Dark, or System to follow your operating system.
- **Density** — Compact fits more rows on a screen.
- **Motion** — Reduced turns animations off.
- **Start with the sidebar collapsed**.
- **Reset appearance** — back to the defaults.

### Introduction

**Replay the introduction** opens the onboarding tour again.

### Disclosure mail

The master switch. When on, Notify sends a disclosure to the repository owner.
When off, nothing is sent and anything already queued is dropped. Per-severity
or digest preferences do not exist yet.

### Safety masking and unmasked owners

**On** (the default): every credential is shown masked. **Off**: credentials
found in repositories of the owners listed under **Unmasked owners** are shown
in full, for testing; everyone else stays masked, and mail and exports are
always masked.

- Only add GitHub users or organisations you own.
- Values are kept, encrypted, from the next scan on — rescan an owner's
  repositories to see existing findings unmasked.
- Removing an owner deletes everything kept for them.
- If the server has no `SECRET_SEAL_KEY`, unmasking is unavailable and nothing
  is ever kept. Generate one with `openssl rand -hex 32`, add it to
  `.env.local` and restart.

While masking is off, a red warning stays on the page: full credentials of the
listed owners are visible to anyone who can open this dashboard.

---

## Live validation

When the operator turns it on (`PROBE_SECRETS`) for their own targets,
GitScanner makes one read-only identity call per finding to see whether the key
still works and whose account it is:

- **Live — rotate** — the provider accepted it. Rotate this one first.
- **Revoked** — the provider rejected it; already rotated or revoked.
- **No prober** — no check exists for this provider.
- **Check failed** — rate limit or network; try a rescan later.

It is off by default, never enumerates account contents, and its quota and
limits readings appear in the finding drawer.

---

## Severity

- **Critical** — full control: move money, take over the account, read
  everything it protects.
- **High** — broad API access as the account.
- **Medium** — narrow or host-scoped access: one channel, one webhook, a server
  that must also be reachable.
- **Low** — identifies an account but cannot act on it, such as a test-mode
  key. Still worth rotating.

---

## Troubleshooting

- **“Could not reach the API”** — check the address in the top bar and that the
  stack is running; the dot turns green when it answers.
- **A scan sits at 0%** — open it and look at Live activity. *Waiting on the
  GitHub rate limit* resolves on its own; no activity at all means the workers
  are not running.
- **A rescan found nothing new** — the findings belong to the earlier scan;
  filter Findings by the repository instead.
- **Commit unknown** — rescan the repository to attribute it.
- **Notify is greyed out** — turn disclosure mail on in Settings.
- **Show is missing in the drawer** — masking is on, the owner is not on the
  unmask list, the server has no `SECRET_SEAL_KEY`, or the repository was not
  rescanned after the owner was added.
- **The list says it refined only 500 findings** — narrow the server-side
  filters (severity, provider, status, repository) so the window covers
  everything.
- **Stale results** — a job deleted, or data wiped, removes its findings;
  rescan to bring them back.

GitScanner is for authorised security testing only. Scan what you own or have
permission to test, and tell owners about what you find.
