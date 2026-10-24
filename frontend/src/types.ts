export type ScanState     = 'queued' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled'
export type TargetKind    = 'repo' | 'user' | 'org'
export type Severity      = 'critical' | 'high' | 'medium' | 'low' | 'info'
export type FindingStatus = 'open' | 'rotated' | 'false_positive' | 'ignored'

export const SEVERITIES: Severity[]        = ['critical', 'high', 'medium', 'low', 'info']
export const STATUSES:   FindingStatus[]   = ['open', 'rotated', 'false_positive', 'ignored']
export const TERMINAL = new Set<ScanState>(['completed', 'failed', 'cancelled'])

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

/** A job as the server lists it, with the target it was submitted for. */
export interface ScanSummary extends ScanJob {
  target:       { kind: TargetKind | 'allowlist'; value: string }
  created_at:   string
  started_at?:  string
  finished_at?: string
}

export interface RepoRef { id: string; owner: string; name: string; url: string }

export interface Finding {
  id:                  string
  scan_job_id:         string
  repository:          RepoRef
  rule_id:             string
  provider:            string
  severity:            Severity
  status:              FindingStatus
  masked_preview:      string
  file_path:           string
  line_number:         number
  commit_sha:          string
  author_name:         string
  committed_at?:       string
  reachable_from_head: boolean
  detected_at:         string
  metadata:            Record<string, string>
}

export interface FindingFilter {
  severity?:      Severity | ''
  provider?:      string
  status?:        FindingStatus | ''
  repository_id?: string
  scan_id?:       string
  q?:             string
  limit?:         number
  offset?:        number
}

export interface Repository {
  id:               string
  owner:            string
  name:             string
  url:              string
  app_type:         string
  stars:            number
  is_fork:          boolean
  last_scanned_at?: string
  findings:         number
  open:             number
  critical:         number
  high:             number
  medium:           number
  low:              number
}

export interface Count { key: string; count: number }
export interface TimePoint { period: string; findings: number }

export interface Analytics {
  timeline:     TimePoint[]
  by_app_type:  Count[]
  by_rule:      Count[]
  live_probes:  Count[]
}

export interface Overview {
  findings:            number
  open_findings:       number
  reachable_from_head: number
  repositories:        number
  exposed_repos:       number
  active_scans:        number
  candidates:          number
  by_severity:         Count[]
  by_provider:         Count[]
  by_status:           Count[]
  by_scan_state:       Count[]
}

export interface Candidate {
  id:            string
  guessed_name:  string
  env_var_names: string[]
  nearby_hosts:  string[]
  sample_masked: string
  charset_class: string
  length_min:    number
  length_max:    number
  occurrences:   number
  repo_count:    number
  status:        'new' | 'triaged' | 'mapped' | 'ignored'
  first_seen_at: string
  last_seen_at:  string
}
