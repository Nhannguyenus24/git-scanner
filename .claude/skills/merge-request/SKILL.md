---
name: merge-request
description: Ship a change on this project — create the branch, commit, open the pull request and title it. Use this whenever work is ready to propose, or the user says "tạo MR", "mở PR", "tạo branch", "push lên", "merge request", "raise a PR", or asks how to name a branch or a PR here. Also consult it before committing anything, because this project is trunk-based and work does not belong directly on main.
---

# Shipping a change

This project is **trunk-based**: `main` is the only long-lived branch, it is
always releasable, and every change reaches it through a short-lived branch and
a pull request. There is no `develop` or release branch.

The remote is GitHub (`gh`, not `glab`), so "merge request" here means **pull
request** — the naming rules below are what the user asked for, and the tool is
`gh pr`.

## The shape of the work

Short-lived means hours, not weeks. A branch that lives long enough to need
`main` merged into it twice is a branch that should have been split. Small PRs
get reviewed properly; large ones get waved through, which is how the expensive
mistakes get in.

Prefer rebasing onto `main` over merging `main` in — it keeps history linear and
the diff reviewable.

## Branch name: `type/short-description`

Lowercase, hyphen-separated, no spaces. Describe the change, not the file.

```
feature/cloudinary-detection-rules
fix/scan-worker-pipe-deadlock
docs/project-skills
refactor/structure
ci/per-service-chart-and-pr-checks
```

## PR title: `type(scope): description`

Conventional Commits. Imperative mood, lowercase after the colon, no trailing
period, under ~70 characters — detail belongs in the body, not the title.

```
feat(scan): detect cloudinary credentials in separate settings
fix(scan): cancel cat-file before Wait to break the blob-walk deadlock
docs(skills): add run, debug-pipeline and detection-rule skills
```

### Types, and the one mismatch to remember

The branch prefix and the commit type are *almost* the same word. `feature/`
pairs with `feat(`, and only that one differs:

| Branch prefix | PR title type | For |
|---|---|---|
| `feature/` | `feat` | new capability |
| `fix/` | `fix` | a bug |
| `docs/` | `docs` | documentation, skills |
| `refactor/` | `refactor` | behaviour-preserving restructuring |
| `perf/` | `perf` | performance |
| `test/` | `test` | tests only |
| `ci/` | `ci` | workflows, pipelines |
| `build/` | `build` | Dockerfile, Makefile, dependencies |
| `chore/` | `chore` | anything that fits nothing above |

### Scope

The area of the codebase, which here usually maps to a package or a service:

`scan` `discovery` `enricher` `notifier` `analytics` `api` `store` `rabbit`
`kafka` `entity` `skills` `docs` `ci` `deploy` `frontend`

Omit it rather than invent one when a change is genuinely cross-cutting.

## The flow

```bash
# 1. branch from current main
git switch main && git pull --ff-only
git switch -c feature/cloudinary-detection-rules

# 2. work, verifying as you go
go build ./... && go vet ./... && go test -count=1 ./test/unit/...

# 3. commit — the message follows the same type(scope): convention
git add <specific files>
git commit

# 4. push and open the PR
git push -u origin feature/cloudinary-detection-rules
gh pr create --title "feat(scan): detect cloudinary credentials in separate settings" --body "$(cat <<'EOF'
## Summary
- what changed and why, in a few bullets

## Test plan
- [ ] how a reviewer confirms it

🤖 Generated with [Claude Code](https://claude.com/claude-code)
EOF
)"
```

Stage named files rather than `git add -A`. This repository scans for leaked
credentials and real secrets have been handled in its working tree; a broad add
is how one reaches a public branch. Review `git status` before committing, and
if a filename is unfamiliar, read the file.

## CI is the gate

`.github/workflows/ci.yaml` runs on every pull request and on pushes to `main`:
`make check` (vet, lint, unit tests) plus the Helm chart render and the frontend
type-check and build. Trunk-based development only works if `main` stays green,
so a red PR is not ready regardless of how small it looks.

`make check` locally runs the same targets — use it before pushing rather than
discovering it in CI.

## Confirm before acting on the remote

Branching, committing and running tests are local and reversible. Pushing and
opening a PR are not: they are visible to everyone with access and send
notifications. Say what you are about to push and to which branch, and get
agreement first. Approval to push once is not approval to push again later.

Never push to `main` directly, and never force-push a shared branch.

## Work already sitting on main

If commits have landed on local `main` that have not been pushed, they bypassed
this flow and should be moved onto a branch before anything is pushed:

```bash
git switch -c docs/project-skills          # branch at the current commit
git switch main
git reset --hard origin/main               # discards nothing: the commits are on the branch
git switch docs/project-skills
```

Run `git status` first and confirm the commits really are on the new branch
before the reset — `reset --hard` is one of the few genuinely destructive
commands here, and it is only safe because the branch created a moment earlier
still points at that work.
