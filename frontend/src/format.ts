import type { FindingStatus } from './types'

// paletteFor assigns each key a stable colour by its position in the given order,
// so a filter that drops a series never repaints the survivors. The palette and
// the "other" colour come from the active theme (theme.chart), so they switch
// with light/dark.
export function paletteFor(keys: string[], categorical: string[], other: string): Record<string, string> {
  const out: Record<string, string> = {}
  keys.forEach((k, i) => { out[k] = i < categorical.length ? categorical[i] : other })
  return out
}

export const STATUS_LABEL: Record<FindingStatus, string> = {
  open:           'Open',
  rotated:        'Rotated',
  false_positive: 'False positive',
  ignored:        'Ignored',
}

export function timeAgo(iso?: string): string {
  if (!iso) return '—'
  const s = (Date.now() - new Date(iso).getTime()) / 1000
  if (s < 60)     return 'just now'
  if (s < 3600)   return `${Math.floor(s / 60)}m ago`
  if (s < 86400)  return `${Math.floor(s / 3600)}h ago`
  if (s < 604800) return `${Math.floor(s / 86400)}d ago`
  return new Date(iso).toLocaleDateString()
}

export function duration(from?: string, to?: string): string {
  if (!from) return '—'
  const s = Math.max(0, ((to ? new Date(to) : new Date()).getTime() - new Date(from).getTime()) / 1000)
  if (s < 60)   return `${Math.round(s)}s`
  if (s < 3600) return `${Math.floor(s / 60)}m ${Math.round(s % 60)}s`
  return `${Math.floor(s / 3600)}h ${Math.floor((s % 3600) / 60)}m`
}

export const num = (n: number) => n.toLocaleString()

/** A browsable GitHub link for a finding, or undefined when it cannot be pinned. */
export function fileLink(repoUrl: string, path: string, line: number, sha: string, atHead: boolean): string | undefined {
  const base = repoUrl.replace(/\.git$/, '')
  if (!base.startsWith('https://github.com/')) return undefined
  const ref = sha || (atHead ? 'HEAD' : '')
  if (!ref) return undefined
  return `${base}/blob/${ref}/${path.split('/').map(encodeURIComponent).join('/')}${line ? `#L${line}` : ''}`
}

/** The GitHub page for the commit that introduced a finding. */
export function commitLink(repoUrl: string, sha: string): string | undefined {
  const base = repoUrl.replace(/\.git$/, '')
  return sha && base.startsWith('https://github.com/') ? `${base}/commit/${sha}` : undefined
}

/** YYYY-MM → "Mon 'YY", for timeline axes. */
export function monthLabel(period: string): string {
  const [y, m] = period.split('-').map(Number)
  if (!y || !m) return period
  const mon = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'][m - 1]
  return `${mon} '${String(y).slice(2)}`
}
