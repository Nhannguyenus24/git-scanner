import { describe, expect, it } from 'vitest'
import { diff, snapshot } from '../src/alerts'
import type { ScanSummary } from '../src/types'

const scan = (id: string, state: ScanSummary['state'], findings = 0): ScanSummary => ({
  id, state, progress: 1, repos_total: 1, repos_scanned: 1, blobs_scanned: 1, findings_count: findings,
  target: { kind: 'repo', value: `acme/${id}` }, created_at: '2026-01-01T00:00:00Z',
})

describe('desktop alerts', () => {
  it('announces nothing on the first poll', () => {
    expect(diff(null, [scan('a', 'completed')], 5)).toEqual([])
  })

  it('announces a scan seen running and now finished, once', () => {
    const before = snapshot([scan('a', 'running'), scan('b', 'running')], 0)
    const now = [scan('a', 'completed', 4), scan('b', 'running')]
    expect(diff(before, now, 0)).toEqual([{ kind: 'scan', id: 'a', target: 'acme/a', state: 'completed', findings: 4 }])
    expect(diff(snapshot(now, 0), now, 0)).toEqual([])
  })

  it('does not announce a job first seen already finished', () => {
    expect(diff(snapshot([], 0), [scan('c', 'failed')], 0)).toEqual([])
  })

  it('announces only a rise in confirmed-live keys', () => {
    expect(diff(snapshot([], 2), [], 5)).toEqual([{ kind: 'live', added: 3 }])
    expect(diff(snapshot([], 5), [], 4)).toEqual([])
  })
})
