import { TERMINAL, type ScanState, type ScanSummary } from './types'

// What is worth interrupting someone for, decided from two polls in a row.
//
// Pure functions, so the rule can be tested without a browser: a scan that was
// unfinished last time and is finished now, and a rise in the number of
// credentials the probe confirmed live. The first poll is only a baseline —
// opening the dashboard must not announce every scan that ever finished.

export interface Snapshot {
  states: Record<string, ScanState>
  live:   number
}

export type Alert =
  | { kind: 'scan'; id: string; target: string; state: ScanState; findings: number }
  | { kind: 'live'; added: number }

export function snapshot(scans: ScanSummary[], live: number): Snapshot {
  return { states: Object.fromEntries(scans.map(s => [s.id, s.state])), live }
}

export function diff(prev: Snapshot | null, scans: ScanSummary[], live: number): Alert[] {
  if (!prev) return []
  const out: Alert[] = []
  for (const s of scans) {
    const was = prev.states[s.id]
    // A job first seen already finished was not watched finishing; skip it.
    if (was && !TERMINAL.has(was) && TERMINAL.has(s.state)) {
      out.push({ kind: 'scan', id: s.id, target: s.target.value, state: s.state, findings: s.findings_count })
    }
  }
  if (live > prev.live) out.push({ kind: 'live', added: live - prev.live })
  return out
}
