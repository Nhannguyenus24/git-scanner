import type { Finding, Repository, ScanSummary } from './types'

// Exports of what the browser already has on screen.
//
// One rule governs this file: `secret` never leaves it. The server sends the
// unmasked value only while masking is off and only for an unmask-listed owner,
// and that is a decision about one person looking at one drawer — not about a
// file that will be mailed around, committed, or pasted into a ticket. Every
// exporter below builds its rows from an explicit field list, so a field added
// to Finding later cannot appear in a download by accident.

export interface ExportScope {
  /** How the rows were filtered, printed in the file so a reader knows the scope. */
  filters: Record<string, string>
  total?:  number
}

const stamp = () => new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19)

/** Hands the browser a file without a round trip to the server. */
export function download(filename: string, body: string, mime: string) {
  const url = URL.createObjectURL(new Blob([body], { type: `${mime};charset=utf-8` }))
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  // Revoked on the next tick: Safari needs the object to outlive the click.
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}

/** RFC 4180 quoting. A leading =, +, - or @ is prefixed with a quote so a
 *  spreadsheet treats the cell as text instead of a formula. */
function cell(v: unknown): string {
  let s = v === undefined || v === null ? '' : String(v)
  if (/^[=+\-@\t\r]/.test(s)) s = `'${s}`
  return /[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s
}

export function toCsv(headers: string[], rows: unknown[][]): string {
  return [headers, ...rows].map(r => r.map(cell).join(',')).join('\r\n') + '\r\n'
}

const FINDING_COLUMNS = [
  'id', 'severity', 'status', 'provider', 'rule_id', 'masked_preview',
  'repository', 'repository_url', 'file_path', 'line_number',
  'reachable_from_head', 'commit_sha', 'author_name', 'committed_at',
  'detected_at', 'probe_status',
]

const findingRow = (f: Finding) => [
  f.id, f.severity, f.status, f.provider || 'unmapped', f.rule_id, f.masked_preview,
  `${f.repository.owner}/${f.repository.name}`, f.repository.url, f.file_path, f.line_number,
  f.reachable_from_head, f.commit_sha, f.author_name, f.committed_at ?? '',
  f.detected_at, f.metadata?.probe_status ?? '',
]

export function findingsCsv(findings: Finding[], scope: ExportScope): string {
  // The scope goes in as comment rows above the header: a bare CSV of 40 rows
  // out of 4000 is indistinguishable from the whole database a week later.
  const head = [
    `# GitScanner findings export`,
    `# generated: ${new Date().toISOString()}`,
    `# rows: ${findings.length}${scope.total !== undefined ? ` of ${scope.total} matching` : ''}`,
    `# filters: ${describeFilters(scope.filters)}`,
    `# values are masked previews; plaintext credentials are never exported`,
  ].join('\r\n')
  return `${head}\r\n${toCsv(FINDING_COLUMNS, findings.map(findingRow))}`
}

export function findingsJson(findings: Finding[], scope: ExportScope): string {
  return JSON.stringify({
    generated_at: new Date().toISOString(),
    filters: scope.filters,
    returned: findings.length,
    total_matching: scope.total,
    masked: true,
    findings: findings.map(f => ({
      id: f.id, severity: f.severity, status: f.status,
      provider: f.provider || 'unmapped', rule_id: f.rule_id,
      masked_preview: f.masked_preview,
      repository: { id: f.repository.id, owner: f.repository.owner, name: f.repository.name, url: f.repository.url },
      file_path: f.file_path, line_number: f.line_number,
      reachable_from_head: f.reachable_from_head,
      commit_sha: f.commit_sha, author_name: f.author_name, committed_at: f.committed_at,
      detected_at: f.detected_at, scan_job_id: f.scan_job_id,
      probe_status: f.metadata?.probe_status,
    })),
  }, null, 2)
}

export function describeFilters(filters: Record<string, string>): string {
  const live = Object.entries(filters).filter(([, v]) => v)
  return live.length ? live.map(([k, v]) => `${k}=${v}`).join(', ') : 'none (all findings)'
}

/** An incident write-up: what was found, where, and what to do, in the order a
 *  responder works. Repositories come first because rotation is per credential
 *  but notification is per repository. */
export function incidentMarkdown(findings: Finding[], scope: ExportScope): string {
  const byRepo = new Map<string, Finding[]>()
  for (const f of findings) {
    const k = `${f.repository.owner}/${f.repository.name}`
    byRepo.set(k, [...(byRepo.get(k) ?? []), f])
  }
  const sevCount = (fs: Finding[], s: string) => fs.filter(f => f.severity === s).length
  const live = findings.filter(f => f.metadata?.probe_status === 'live')
  const atHead = findings.filter(f => f.reachable_from_head)

  const lines: string[] = [
    `# Credential exposure report`,
    ``,
    `- Generated: ${new Date().toISOString()}`,
    `- Scope: ${describeFilters(scope.filters)}`,
    `- Findings in this report: ${findings.length}${scope.total !== undefined ? ` (of ${scope.total} matching)` : ''}`,
    `- Repositories affected: ${byRepo.size}`,
    `- Reachable from HEAD: ${atHead.length}`,
    `- Confirmed live by probe: ${live.length}`,
    ``,
    `> Credential values in this report are masked previews. The plaintext is never exported.`,
    ``,
    `## Severity`,
    ``,
    `| Severity | Count |`,
    `|---|---|`,
    ...['critical', 'high', 'medium', 'low', 'info']
      .map(s => `| ${s} | ${sevCount(findings, s)} |`),
    ``,
    `## Priority order`,
    ``,
    `1. Rotate every credential confirmed live by the probe (${live.length}).`,
    `2. Rotate every credential still reachable from HEAD (${atHead.length}) — these are readable by cloning the default branch today.`,
    `3. Rotate the remainder: a credential removed from HEAD is still in the history and still valid until it is revoked.`,
    `4. Purge the history or accept that the value is permanently public, then notify the repository owner.`,
    ``,
    `## By repository`,
    ``,
  ]

  for (const [repo, fs] of [...byRepo.entries()].sort((a, b) => b[1].length - a[1].length)) {
    lines.push(`### ${repo}`, ``, `${fs[0].repository.url}`, ``,
      `| Severity | Provider | Rule | Location | Exposure | Status | Value |`,
      `|---|---|---|---|---|---|---|`)
    for (const f of fs) {
      lines.push(`| ${f.severity} | ${f.provider || 'unmapped'} | \`${f.rule_id}\` | \`${f.file_path}:${f.line_number}\` | ` +
        `${f.reachable_from_head ? 'at HEAD' : 'history only'} | ${f.status} | \`${f.masked_preview}\` |`)
    }
    lines.push(``)
  }
  return lines.join('\n')
}

/** A one-page summary for someone who will not read the table. */
export function executiveMarkdown(findings: Finding[], repos: Repository[], scans: ScanSummary[], scope: ExportScope): string {
  const open = findings.filter(f => f.status === 'open')
  const crit = open.filter(f => f.severity === 'critical' || f.severity === 'high')
  const exposed = repos.filter(r => r.open > 0)
  const lastScan = scans[0]
  return [
    `# Executive summary — credential exposure`,
    ``,
    `Generated ${new Date().toISOString()} · scope: ${describeFilters(scope.filters)}`,
    ``,
    `## Position`,
    ``,
    `- **${open.length}** open findings, of which **${crit.length}** are high or critical.`,
    `- **${exposed.length}** of **${repos.length}** scanned repositories currently hold at least one open credential.`,
    `- **${open.filter(f => f.reachable_from_head).length}** are reachable from the default branch right now.`,
    `- Last scan: ${lastScan ? `${lastScan.target.value} (${lastScan.state}, ${new Date(lastScan.created_at).toLocaleDateString()})` : 'none recorded'}.`,
    ``,
    `## What this means`,
    ``,
    `A credential in git history stays valid until it is revoked at the provider.`,
    `Removing the file does not revoke it, and a rewritten history does not reach`,
    `the clones and forks already taken. Rotation at the provider is the only fix.`,
    ``,
    `## Recommended next steps`,
    ``,
    `1. Rotate the ${crit.length} high and critical credentials.`,
    `2. Contact the ${exposed.filter(r => !r.notification || r.notification.state !== 'sent').length} repositories that have not yet been notified.`,
    `3. Re-scan after rotation to confirm the findings close.`,
    ``,
    `_Credential values are never included in this report._`,
  ].join('\n')
}

export const exportName = (kind: string, ext: string) => `gitscanner-${kind}-${stamp()}.${ext}`
