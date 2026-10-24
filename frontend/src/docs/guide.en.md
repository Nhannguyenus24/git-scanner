# Using GitScanner

GitScanner clones a repository's full history, walks every unique blob, and
reports credentials it finds — in the current tree and anywhere in the past.
This page is the quick guide to the dashboard.

## Connecting

The address of the API is in the top bar; the dot beside it is green when the
dashboard can reach it. Point it at your own API host if you are not running the
default `http://localhost:8080`. The address, your theme and your language are
remembered in this browser.

Use the controls in the top right to switch between **light and dark paper** and
between **English and Tiếng Việt**.

## Starting a scan

Press **New scan** (top left, or the sidebar). Choose what to scan:

- **Repository** — one HTTPS clone URL, e.g. `https://github.com/owner/repo`.
- **User** or **Org** — every repository that account owns. Tick *include forks*
  to scan forked repositories too.

A scan runs in the background. The page opens on the scan's detail view, where
the progress bar and counters update as repositories are walked.

## Reading the results

**Overview** is the headline: open findings, how many are critical, how many are
still live at HEAD, and the breakdowns by severity, provider and triage state.
Every tile and bar is a link into a filtered Findings list.

**Analytics** goes deeper — a timeline of when credentials were introduced, and
breakdowns by application type and detection rule.

**Findings** is the full, filterable list. Filter by severity, provider, status
or free text; the filter lives in the URL, so a filtered view can be shared or
bookmarked. Each row shows:

- the severity and the provider the credential belongs to;
- a masked preview — the real secret never leaves the engine;
- the file and line, with a link to the file on GitHub;
- the commit that introduced it, its author and date;
- **Exposure** — `at HEAD` means it is still in the current tree; `history`
  means it was removed but is still reachable by cloning;
- a **LIVE — rotate** badge when live validation confirmed the key still works.

Use the menu at the end of a row to mark a finding **Rotated**, **False
positive** or **Ignored** as you work through them.

## Repositories and unmapped shapes

**Repositories** ranks every scanned repository by its open exposure, broken
down by severity. **Unmapped** is the backlog of high-entropy secrets no named
rule has claimed yet, clustered by shape — a frequent shape is a provider worth
writing a rule for.

## Live validation

When the operator has turned it on for their own targets, GitScanner makes one
read-only identity call per finding to confirm whether the key still works and
whose account it is. A confirmed-live key is the one to rotate first. It is off
by default and never enumerates account contents.
