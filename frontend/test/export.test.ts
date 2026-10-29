import { describe, expect, it } from 'vitest'
import { executiveMarkdown, findingsCsv, findingsJson, incidentMarkdown, toCsv } from '../src/export'
import type { Finding, Repository } from '../src/types'

// The one thing these tests exist for: no export may carry a plaintext
// credential, in any format, even when the server sent one.
const SECRET = 'ghp_PLAINTEXT_MUST_NEVER_APPEAR'

const finding = (over: Partial<Finding> = {}): Finding => ({
  id: 'f1', scan_job_id: 'j1',
  repository: { id: 'r1', owner: 'acme', name: 'web', url: 'https://github.com/acme/web' },
  rule_id: 'github-pat', provider: 'github', severity: 'critical', status: 'open',
  masked_preview: 'ghp_****abcd', file_path: 'config/app.yml', line_number: 12,
  commit_sha: 'abc123', author_name: 'Dana', committed_at: '2026-01-02T03:04:05Z',
  reachable_from_head: true, detected_at: '2026-02-02T03:04:05Z',
  metadata: { probe_status: 'live' },
  secret: SECRET,
  ...over,
})

const repo: Repository = {
  id: 'r1', owner: 'acme', name: 'web', url: 'https://github.com/acme/web',
  app_type: 'web', stars: 3, is_fork: false, findings: 2, open: 2, critical: 1, high: 1, medium: 0, low: 0,
}

const scope = { filters: { severity: 'critical' }, total: 9 }

describe('no export leaks a credential', () => {
  const fs = [finding()]
  it('CSV', () => expect(findingsCsv(fs, scope)).not.toContain(SECRET))
  it('JSON', () => expect(findingsJson(fs, scope)).not.toContain(SECRET))
  it('incident report', () => expect(incidentMarkdown(fs, scope)).not.toContain(SECRET))
  it('executive summary', () => expect(executiveMarkdown(fs, [repo], [], scope)).not.toContain(SECRET))

  it('still carries the masked preview', () => {
    expect(findingsCsv(fs, scope)).toContain('ghp_****abcd')
    expect(JSON.parse(findingsJson(fs, scope)).findings[0].masked_preview).toBe('ghp_****abcd')
  })

  it('does not gain a field when Finding does', () => {
    // A column list, not a spread: an added field cannot ride along.
    const row = JSON.parse(findingsJson([finding({ metadata: { leaked: SECRET } })], scope)).findings[0]
    expect(JSON.stringify(row)).not.toContain(SECRET)
  })
})

describe('every export states its scope', () => {
  it('CSV writes the filters and the generation time above the header', () => {
    const csv = findingsCsv([finding()], scope)
    expect(csv).toContain('# filters: severity=critical')
    expect(csv).toContain('# rows: 1 of 9 matching')
    expect(csv.split('\r\n').find(l => l.startsWith('id,'))).toBeTruthy()
  })

  it('JSON records the filters and the masked flag', () => {
    const j = JSON.parse(findingsJson([finding()], scope))
    expect(j.filters).toEqual({ severity: 'critical' })
    expect(j.masked).toBe(true)
    expect(j.total_matching).toBe(9)
  })

  it('an unfiltered export says so rather than leaving it blank', () => {
    expect(findingsCsv([finding()], { filters: {} })).toContain('none (all findings)')
  })
})

describe('toCsv', () => {
  it('quotes separators, quotes and newlines', () => {
    expect(toCsv(['a'], [['x,y']])).toContain('"x,y"')
    expect(toCsv(['a'], [['he said "no"']])).toContain('"he said ""no"""')
    expect(toCsv(['a'], [['line1\nline2']])).toContain('"line1\nline2"')
  })

  it('defuses a cell a spreadsheet would run as a formula', () => {
    expect(toCsv(['a'], [['=1+1']])).toContain("'=1+1")
    expect(toCsv(['a'], [['@SUM(A1)']])).toContain("'@SUM(A1)")
  })
})

describe('incident report', () => {
  it('orders the work by what is exploitable now', () => {
    const md = incidentMarkdown([finding(), finding({ id: 'f2', reachable_from_head: false, metadata: {} })], scope)
    expect(md.indexOf('confirmed live')).toBeLessThan(md.indexOf('reachable from HEAD'))
    expect(md).toContain('Repositories affected: 1')
  })
})
