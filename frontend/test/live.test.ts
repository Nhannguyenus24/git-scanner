import { describe, expect, it } from 'vitest'
import { DICT_UI } from '../src/i18n.ui'
import { applyEvent, describe as describeEvent, emptyLive, inFlight, recentlyFinished, REPO_STEPS, stepCounts, type LiveState } from '../src/live'
import type { ProgressEvent } from '../src/types'

let seq = 0
const ev = (e: Partial<ProgressEvent>): ProgressEvent =>
  ({ seq: ++seq, job_id: 'j', at: new Date(1_700_000_000_000 + seq * 1000).toISOString(), stage: 'repo', status: 'start', ...e })

const fold = (events: ProgressEvent[], s: LiveState = emptyLive) => events.reduce(applyEvent, s)

describe('the live reducer', () => {
  it('follows one repository through its steps', () => {
    const s = fold([
      ev({ repo_id: 'r1', repo: 'acme/api', stage: 'repo', status: 'start', attempt: 1 }),
      ev({ repo_id: 'r1', stage: 'clone', status: 'start' }),
      ev({ repo_id: 'r1', stage: 'clone', status: 'done', elapsed_ms: 1200 }),
      ev({ repo_id: 'r1', stage: 'walk', status: 'start' }),
      ev({ repo_id: 'r1', stage: 'walk', status: 'progress', blobs: 5000, matches: 2, path: 'config/app.yml' }),
    ])
    const [r] = inFlight(s)
    expect(r).toMatchObject({ repo: 'acme/api', stage: 'walk', blobs: 5000, matches: 2, path: 'config/app.yml', done: ['clone'] })
    expect(stepCounts(s)).toEqual({ walk: 1 })
    // Walk ticks update the row but stay out of the log.
    expect(s.log.map(e => `${e.stage}:${e.status}`)).toEqual(['walk:start', 'clone:done', 'clone:start', 'repo:start'])
  })

  it('moves a finished repository out of the in-flight list', () => {
    const s = fold([
      ev({ repo_id: 'r1', repo: 'acme/api', stage: 'repo', status: 'start' }),
      ev({ repo_id: 'r1', stage: 'repo', status: 'done', findings: 3, blobs: 9000, elapsed_ms: 4200 }),
    ])
    expect(inFlight(s)).toEqual([])
    expect(recentlyFinished(s)[0]).toMatchObject({ repo: 'acme/api', findings: 3, blobs: 9000, elapsedMs: 4200 })
  })

  it('starts the strip over on a retry', () => {
    const s = fold([
      ev({ repo_id: 'r1', stage: 'repo', status: 'start', attempt: 1 }),
      ev({ repo_id: 'r1', stage: 'clone', status: 'done' }),
      ev({ repo_id: 'r1', stage: 'repo', status: 'failed', error: 'early EOF' }),
      ev({ repo_id: 'r1', stage: 'repo', status: 'start', attempt: 2 }),
    ])
    expect(inFlight(s)[0]).toMatchObject({ attempt: 2, done: [], error: undefined })
  })

  it('ignores an event it has already seen, as after a reconnect', () => {
    const first = ev({ repo_id: 'r1', stage: 'repo', status: 'start' })
    const s = fold([first, first])
    expect(s.log).toHaveLength(1)
  })

  it('adds up discovery pages and records the job ending', () => {
    const s = fold([
      ev({ stage: 'discover', status: 'start' }),
      ev({ stage: 'discover', status: 'progress', repos: 30 }),
      ev({ stage: 'discover', status: 'done', repos: 12 }),
      ev({ stage: 'job', status: 'done' }),
    ])
    expect(s.discover).toMatchObject({ status: 'done', repos: 42 })
    expect(s.ended).toBe('done')
  })
})

describe('every live key the UI can build exists', () => {
  // These keys are assembled at runtime from the event's stage and status, so
  // the source scan in i18n.test cannot see them.
  const emitted: [string, string[]][] = [
    ['discover', ['start', 'progress', 'done', 'paused', 'deferred', 'failed']],
    ['repo', ['start', 'done', 'failed', 'skipped']],
    ['clone', ['start', 'done', 'failed']],
    ['walk', ['start', 'progress', 'done', 'failed']],
    ['attribute', ['start', 'done']],
    ['probe', ['start', 'done']],
    ['store', ['start', 'done', 'failed']],
    ['job', ['done', 'failed', 'cancelled']],
  ]
  it('for log lines, steps, results and statuses', () => {
    const keys = [
      ...emitted.flatMap(([stage, ss]) => ss.map(st => `live.ev.${stage}.${st}`)),
      ...REPO_STEPS.flatMap(s => [`live.step.${s}`, `live.stepName.${s}`]),
      'live.step.repo', 'live.step.queue',
      ...['done', 'failed', 'skipped'].map(s => `live.result.${s}`),
      ...['start', 'progress', 'done', 'paused', 'deferred', 'failed'].map(s => `live.discover.${s}`),
      ...['connecting', 'live', 'reconnecting', 'ended', 'unavailable'].map(s => `live.status.${s}`),
    ]
    expect(keys.filter(k => !(k in DICT_UI))).toEqual([])
  })

  it('describes an event in words without leaving a placeholder', () => {
    const t = (k: string, v?: Record<string, string | number>) =>
      (DICT_UI[k]?.en ?? k).replace(/\{(\w+)\}/g, (_, n) => String(v?.[n] ?? `{${n}}`))
    const line = describeEvent(ev({ repo: 'acme/api', repo_id: 'r1', stage: 'repo', status: 'done', findings: 2, blobs: 1234, elapsed_ms: 5400 }), t)
    expect(line).toBe('acme/api: done in 5.4s · 2 new findings in 1,234 blobs')
  })
})
