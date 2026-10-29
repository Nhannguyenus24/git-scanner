import { describe, expect, it } from 'vitest'
import { etaSeconds, stagesFor } from '../src/components/ScanTimeline'
import type { ScanState, ScanSummary } from '../src/types'

// t() is replaced by the key itself: these tests are about which stage is
// active, not about what the label says in seven languages.
const t = (k: string) => k

const job = (over: Partial<ScanSummary> = {}): ScanSummary => ({
  id: 'j1', state: 'running' as ScanState, progress: .5,
  repos_total: 10, repos_scanned: 5, blobs_scanned: 1000, findings_count: 2,
  target: { kind: 'org', value: 'acme' },
  created_at: '2026-02-02T00:00:00Z', started_at: '2026-02-02T00:00:00Z',
  ...over,
})

const stage = (s: ScanSummary, key: string) => stagesFor(s, t).find(x => x.key === key)

describe('stagesFor', () => {
  it('waits on discovery while the repository count is unknown', () => {
    const s = job({ state: 'queued', repos_total: 0, repos_scanned: 0 })
    expect(stage(s, 'stage.discover')?.state).toBe('active')
    expect(stage(s, 'stage.scan')?.state).toBe('pending')
  })

  it('moves to scanning once repositories are queued', () => {
    const s = job()
    expect(stage(s, 'stage.discover')?.state).toBe('done')
    expect(stage(s, 'stage.scan')?.state).toBe('active')
    expect(stage(s, 'stage.finalize')?.state).toBe('pending')
  })

  it('marks everything done on a clean finish', () => {
    const s = job({ state: 'completed', progress: 1, repos_scanned: 10, finished_at: '2026-02-02T01:00:00Z' })
    expect(stagesFor(s, t).filter(x => x.state === 'pending')).toHaveLength(0)
    expect(stage(s, 'stage.completed')?.state).toBe('done')
  })

  it('marks the stage that was running as failed, not the whole list', () => {
    const s = job({ state: 'failed', error: 'clone refused' })
    expect(stage(s, 'stage.scan')?.state).toBe('failed')
    expect(stage(s, 'stage.discover')?.state).toBe('done')
    expect(stage(s, 'stage.failed')?.detail).toBe('clone refused')
  })

  it('shows what a cancelled job skipped rather than claiming success', () => {
    const s = job({ state: 'cancelled' })
    expect(stage(s, 'stage.finalize')?.state).toBe('skipped')
    expect(stage(s, 'stage.cancelled')?.state).toBe('skipped')
  })

  it('names the three steps of the scanning stage', () => {
    expect(stage(job(), 'stage.scan')?.steps)
      .toEqual(['stage.step.clone', 'stage.step.walk', 'stage.step.detect'])
  })
})

describe('etaSeconds', () => {
  const started = (secondsAgo: number) => new Date(Date.now() - secondsAgo * 1000).toISOString()

  it('estimates from the observed rate', () => {
    // 5 of 10 repositories in 100s → about 100s left.
    const s = etaSeconds(job({ started_at: started(100), repos_scanned: 5, repos_total: 10 }))
    expect(s).toBeGreaterThan(80)
    expect(s).toBeLessThan(120)
  })

  it('says nothing from too few samples', () => {
    expect(etaSeconds(job({ started_at: started(100), repos_scanned: 2 }))).toBeUndefined()
  })

  it('says nothing before the repository count is known', () => {
    expect(etaSeconds(job({ started_at: started(100), repos_total: 0, repos_scanned: 0 }))).toBeUndefined()
  })

  it('says nothing for a paused or finished job', () => {
    expect(etaSeconds(job({ state: 'paused', started_at: started(100) }))).toBeUndefined()
    expect(etaSeconds(job({ state: 'completed', started_at: started(100) }))).toBeUndefined()
  })

  it('says nothing in the first few seconds', () => {
    expect(etaSeconds(job({ started_at: started(2), repos_scanned: 5 }))).toBeUndefined()
  })
})
