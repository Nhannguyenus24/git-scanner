import type { Overview, Repository } from './types'

// The posture score, and the per-repository risk score behind the repositories
// table.
//
// Both are derived here, in the browser, from the server's *aggregates* — the
// /v1/overview counters and the per-repository counts, each of which the server
// computed over the whole database. That is the condition under which deriving
// a number client-side is honest: nothing here reads a page of findings and
// extrapolates. When the API grows a scored endpoint of its own, this file is
// what it replaces; until then the maths is written out so the number can be
// explained rather than trusted.
//
// Every factor returns both a penalty and the sentence that justifies it. A
// score with no explanation is a number people learn to ignore.

export interface Factor {
  /** i18n key for the factor's name. */
  key:     string
  /** Points subtracted from 100. */
  penalty: number
  /** The measured quantity, for the explanation line. */
  value:   number
  /** Worst case this factor can cost. */
  max:     number
  detail:  Record<string, string | number>
}

export interface Posture {
  score:   number
  band:    'good' | 'fair' | 'poor' | 'critical'
  factors: Factor[]
  /** False when nothing has been scanned: a score of 100 would be a lie. */
  scored:  boolean
}

/** A saturating penalty: the first few cost most, and it never runs away. */
function ramp(value: number, scale: number, max: number): number {
  if (value <= 0) return 0
  return Math.round(max * (1 - Math.exp(-value / scale)))
}

export function posture(o: Overview, repos?: Repository[]): Posture {
  const sev = (k: string) => o.by_severity.find(c => c.key === k)?.count ?? 0
  const critical = sev('critical')
  const high = sev('high')
  const unnotified = repos
    ? repos.filter(r => r.open > 0 && r.notification?.state !== 'sent').length
    : o.awaiting_notice

  const factors: Factor[] = [
    {
      key: 'posture.f.critical', value: critical, max: 30,
      penalty: ramp(critical, 4, 30),
      detail: { n: critical },
    },
    {
      key: 'posture.f.high', value: high, max: 15,
      penalty: ramp(high, 8, 15),
      detail: { n: high },
    },
    {
      // Reachable from HEAD is the sharpest signal the scanner produces: anyone
      // who clones the default branch today gets the credential.
      key: 'posture.f.reachable', value: o.reachable_from_head, max: 25,
      penalty: ramp(o.reachable_from_head, 5, 25),
      detail: { n: o.reachable_from_head },
    },
    {
      key: 'posture.f.exposed', value: o.exposed_repos, max: 15,
      penalty: o.repositories > 0
        ? Math.round((o.exposed_repos / o.repositories) * 15)
        : 0,
      detail: { a: o.exposed_repos, b: o.repositories },
    },
    {
      key: 'posture.f.open', value: o.open_findings, max: 10,
      penalty: o.findings > 0 ? Math.round((o.open_findings / o.findings) * 10) : 0,
      detail: { a: o.open_findings, b: o.findings },
    },
    {
      key: 'posture.f.unnotified', value: unnotified, max: 5,
      penalty: ramp(unnotified, 4, 5),
      detail: { n: unnotified },
    },
  ]

  const score = Math.max(0, 100 - factors.reduce((n, f) => n + f.penalty, 0))
  return {
    score,
    band: score >= 85 ? 'good' : score >= 65 ? 'fair' : score >= 40 ? 'poor' : 'critical',
    factors,
    scored: o.repositories > 0,
  }
}

export interface RepoRisk {
  score: number
  band:  'low' | 'medium' | 'high' | 'critical'
  /** i18n key for the one thing worth doing next on this repository. */
  action: string
}

/**
 * A repository's risk, 0–100 where 100 is worst — the inverse direction from the
 * posture score on purpose: posture is "how are we doing", risk is "how bad is
 * this one". Severity counts are weighted by how much damage the credential
 * class usually does, not by how many there are.
 */
export function repoRisk(r: Repository): RepoRisk {
  if (r.open === 0) return { score: 0, band: 'low', action: 'repos.action.clear' }
  const weighted = r.critical * 10 + r.high * 5 + r.medium * 2 + r.low
  const score = Math.min(100, Math.round(100 * (1 - Math.exp(-weighted / 18))))
  const band = score >= 75 ? 'critical' : score >= 50 ? 'high' : score >= 25 ? 'medium' : 'low'

  const action =
    r.critical > 0 ? 'repos.action.rotate'
    : r.notification?.state === 'failed' ? 'repos.action.retryNotice'
    : !r.notification ? 'repos.action.notify'
    : r.notification.state === 'pending' ? 'repos.action.awaiting'
    : 'repos.action.review'

  return { score, band, action }
}

export const BAND_KEY: Record<RepoRisk['band'], 'critical' | 'high' | 'medium' | 'low'> = {
  critical: 'critical', high: 'high', medium: 'medium', low: 'low',
}
