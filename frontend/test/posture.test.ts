import { describe, expect, it } from 'vitest'
import { posture, repoRisk } from '../src/posture'
import type { Count, Overview, Repository } from '../src/types'

const counts = (o: Record<string, number>): Count[] =>
  Object.entries(o).map(([key, count]) => ({ key, count }))

const overview = (over: Partial<Overview> = {}): Overview => ({
  findings: 0, open_findings: 0, reachable_from_head: 0,
  repositories: 0, exposed_repos: 0, active_scans: 0, candidates: 0,
  notified_repos: 0, awaiting_notice: 0,
  by_severity: [], by_provider: [], by_status: [], by_scan_state: [],
  ...over,
})

const repo = (over: Partial<Repository> = {}): Repository => ({
  id: 'r', owner: 'acme', name: 'web', url: 'u', app_type: '', stars: 0, is_fork: false,
  findings: 0, open: 0, critical: 0, high: 0, medium: 0, low: 0,
  ...over,
})

describe('posture', () => {
  it('refuses to score an empty database', () => {
    const p = posture(overview())
    expect(p.scored).toBe(false)
  })

  it('is 100 when everything scanned is clean', () => {
    const p = posture(overview({ repositories: 10, findings: 4, open_findings: 0 }))
    expect(p.score).toBe(100)
    expect(p.band).toBe('good')
    expect(p.scored).toBe(true)
  })

  it('never goes below zero', () => {
    const p = posture(overview({
      repositories: 10, exposed_repos: 10, findings: 900, open_findings: 900,
      reachable_from_head: 400, awaiting_notice: 50,
      by_severity: counts({ critical: 500, high: 300 }),
    }))
    expect(p.score).toBe(0)
    expect(p.band).toBe('critical')
  })

  it('charges more for the first critical finding than for the tenth', () => {
    const at = (n: number) => posture(overview({ repositories: 5, findings: n, open_findings: n, by_severity: counts({ critical: n }) })).score
    const firstFew = at(0) - at(1)
    const laterFew = at(9) - at(10)
    expect(firstFew).toBeGreaterThan(laterFew)
  })

  it('adds up to the published total', () => {
    const p = posture(overview({
      repositories: 4, exposed_repos: 2, findings: 10, open_findings: 6,
      reachable_from_head: 3, awaiting_notice: 1, by_severity: counts({ critical: 2, high: 1 }),
    }))
    const sum = p.factors.reduce((n, f) => n + f.penalty, 0)
    expect(p.score).toBe(100 - sum)
    expect(p.factors.every(f => f.penalty <= f.max)).toBe(true)
  })

  it('prefers the repository list over the server counter for unnotified repositories', () => {
    const o = overview({ repositories: 2, exposed_repos: 2, findings: 2, open_findings: 2, awaiting_notice: 0 })
    const withRepos = posture(o, [
      repo({ open: 1 }),
      repo({ id: 'r2', open: 1, notification: { state: 'sent', recipient: 'x', created_at: 'now' } }),
    ])
    const factor = withRepos.factors.find(f => f.key === 'posture.f.unnotified')!
    expect(factor.value).toBe(1)
  })
})

describe('repoRisk', () => {
  it('is zero and clear when nothing is open', () => {
    const r = repoRisk(repo({ findings: 3 }))
    expect(r.score).toBe(0)
    expect(r.action).toBe('repos.action.clear')
  })

  it('weights a critical finding above several low ones', () => {
    expect(repoRisk(repo({ open: 1, critical: 1 })).score)
      .toBeGreaterThan(repoRisk(repo({ open: 5, low: 5 })).score)
  })

  it('stays inside 0–100', () => {
    const r = repoRisk(repo({ open: 999, critical: 500, high: 400 }))
    expect(r.score).toBeLessThanOrEqual(100)
    expect(r.band).toBe('critical')
  })

  it('recommends rotation before notification when a critical key is open', () => {
    expect(repoRisk(repo({ open: 1, critical: 1 })).action).toBe('repos.action.rotate')
    expect(repoRisk(repo({ open: 1, medium: 1 })).action).toBe('repos.action.notify')
    expect(repoRisk(repo({ open: 1, medium: 1, notification: { state: 'failed', recipient: 'x', created_at: 'n' } })).action)
      .toBe('repos.action.retryNotice')
  })
})
