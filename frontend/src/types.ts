export type ScanState    = 'queued' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled'
export type TargetKind   = 'repo'   | 'user'    | 'org'
export type Severity     = 'critical' | 'high'  | 'medium' | 'low' | 'info'
export type FindingStatus = 'open'  | 'rotated' | 'false_positive' | 'ignored'

export interface ScanJob {
  id:             string
  state:          ScanState
  progress:       number
  repos_total:    number
  repos_scanned:  number
  blobs_scanned:  number
  findings_count: number
  error?:         string
}

export interface Finding {
  id:                  string
  rule_id:             string
  provider:            string
  severity:            Severity
  masked_preview:      string
  file_path:           string
  line_number:         number
  blob_oid:            string
  reachable_from_head: boolean
  status:              FindingStatus
}

export interface Target {
  kind:  TargetKind | 'unknown'
  value: string
}

/** ScanJob enriched with local metadata (target, submitted time). */
export interface LocalJob extends ScanJob {
  target:       Target
  submittedAt:  string   // ISO string
}

export const TERMINAL = new Set<ScanState>(['completed', 'failed', 'cancelled'])
