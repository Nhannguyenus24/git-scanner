import type { ProgressEvent, ProgressStage } from './types'

// The live view of a running scan, folded from the event stream.
//
// Everything here is narration around the job's counters, which the page keeps
// polling. A lost or late event can make a line here stale for a moment; it can
// never make a number on the page wrong, so the reducer favours simple and
// forgiving over exact.

/** The steps one repository passes through, in order. probe runs only when
 *  live probing is enabled, and attribute only when something was found. */
export const REPO_STEPS: ProgressStage[] = ['clone', 'walk', 'attribute', 'probe', 'store']

export interface RepoLive {
  id:         string
  repo:       string
  /** The step it is in, or 'repo' once it has finished one way or another. */
  stage:      ProgressStage
  status:     ProgressEvent['status']
  attempt:    number
  blobs:      number
  matches:    number
  findings:   number
  live:       number
  path?:      string
  error?:     string
  appType?:   string
  elapsedMs?: number
  /** Steps this attempt has finished, for the step strip. */
  done:       ProgressStage[]
  at:         string
}

export interface DiscoverLive {
  status: ProgressEvent['status']
  repos:  number
  error?: string
  at:     string
}

export interface LiveState {
  repos:    Record<string, RepoLive>
  discover?: DiscoverLive
  /** Most recent first, capped at LOG_SIZE. */
  log:      ProgressEvent[]
  lastSeq:  number
  ended?:   string
}

export const LOG_SIZE = 80

export const emptyLive: LiveState = { repos: {}, log: [], lastSeq: 0 }

// Walk ticks arrive every ~750ms per repository; logging each one would bury
// every other line, so they update the repository's row and stay out of the log.
const quiet = (ev: ProgressEvent) => ev.stage === 'walk' && ev.status === 'progress'

/** Folds one event into the state. Pure, so the page and the tests share it. */
export function applyEvent(s: LiveState, ev: ProgressEvent): LiveState {
  if (ev.seq && ev.seq <= s.lastSeq) return s // a replay after reconnecting
  const next: LiveState = {
    ...s,
    lastSeq: Math.max(s.lastSeq, ev.seq || 0),
    log: quiet(ev) ? s.log : [ev, ...s.log].slice(0, LOG_SIZE),
  }

  if (ev.stage === 'discover') {
    const prev = s.discover?.repos ?? 0
    next.discover = {
      status: ev.status,
      repos: prev + (ev.status === 'progress' || ev.status === 'done' ? ev.repos ?? 0 : 0),
      error: ev.error,
      at: ev.at,
    }
    return next
  }
  if (ev.stage === 'job') {
    next.ended = ev.status
    return next
  }
  if (!ev.repo_id) return next

  const prev = s.repos[ev.repo_id]
  // A new attempt starts the strip over; the counts it reports replace the old.
  const fresh = ev.stage === 'repo' && ev.status === 'start'
  const base: RepoLive = fresh || !prev ? {
    id: ev.repo_id, repo: ev.repo || prev?.repo || ev.repo_id.slice(0, 8),
    stage: 'clone', status: 'start', attempt: ev.attempt ?? 1,
    blobs: 0, matches: 0, findings: 0, live: 0, done: [], at: ev.at,
  } : prev

  const row: RepoLive = {
    ...base,
    repo: ev.repo || base.repo,
    attempt: ev.attempt ?? base.attempt,
    status: ev.status,
    at: ev.at,
    blobs: ev.blobs ?? base.blobs,
    matches: ev.matches ?? base.matches,
    findings: ev.findings ?? base.findings,
    live: ev.live ?? base.live,
    path: ev.stage === 'walk' ? ev.path ?? base.path : base.path,
    error: ev.error ?? (ev.status === 'start' ? undefined : base.error),
    appType: ev.app_type ?? base.appType,
    elapsedMs: ev.stage === 'repo' ? ev.elapsed_ms ?? base.elapsedMs : base.elapsedMs,
    stage: fresh ? 'clone' : ev.stage,
    done: ev.status === 'done' && ev.stage !== 'repo' && !base.done.includes(ev.stage)
      ? [...base.done, ev.stage] : base.done,
  }
  next.repos = { ...s.repos, [ev.repo_id]: row }
  return next
}

/** True once the repository will do no more work in this attempt. */
export const finished = (r: RepoLive) =>
  r.stage === 'repo' && (r.status === 'done' || r.status === 'skipped' || r.status === 'failed')

/** Repositories still working, the longest-running first. */
export function inFlight(s: LiveState): RepoLive[] {
  return Object.values(s.repos).filter(r => !finished(r)).sort((a, b) => a.at.localeCompare(b.at))
}

/** Repositories that finished most recently, newest first. */
export function recentlyFinished(s: LiveState, n = 8): RepoLive[] {
  return Object.values(s.repos).filter(finished).sort((a, b) => b.at.localeCompare(a.at)).slice(0, n)
}

/** How many repositories are in each step right now, for the stage strip. */
export function stepCounts(s: LiveState): Partial<Record<ProgressStage, number>> {
  const out: Partial<Record<ProgressStage, number>> = {}
  for (const r of inFlight(s)) out[r.stage] = (out[r.stage] ?? 0) + 1
  return out
}

type T = (k: string, v?: Record<string, string | number>) => string

/** One log line, in words. `name` is the repository's name when the event
 *  itself does not carry one — a worker that could not read the repository row
 *  still reports its steps, by id. */
export function describe(ev: ProgressEvent, t: T, name?: string): string {
  const v = {
    repo: ev.repo || name || (ev.repo_id ?? '').slice(0, 8),
    blobs: (ev.blobs ?? 0).toLocaleString(),
    matches: ev.matches ?? 0,
    findings: ev.findings ?? 0,
    repos: ev.repos ?? 0,
    attempt: ev.attempt ?? 1,
    secs: ((ev.elapsed_ms ?? 0) / 1000).toFixed(1),
    live: ev.live ?? 0,
    error: ev.error ?? '',
  }
  return t(`live.ev.${ev.stage}.${ev.status}`, v)
}
