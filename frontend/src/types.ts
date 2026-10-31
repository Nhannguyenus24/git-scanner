export type ScanState     = 'queued' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled'
export type TargetKind    = 'repo' | 'user' | 'org' | 'random'
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
  /** The unmasked credential: only while masking is off, for an unmask-listed owner. */
  secret?:             string
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
  /** The latest disclosure mail; absent when the repository was never contacted. */
  notification?:    Notice
}

export type NoticeState = 'pending' | 'sent' | 'failed' | 'suppressed'

export interface Notice {
  state:      NoticeState
  recipient:  string
  error?:     string
  created_at: string
  sent_at?:   string
}

export interface AppSettings {
  notifications_enabled: boolean
  masking_enabled:       boolean
  unmask_owners:         string[]
  /** False when the server has no SECRET_SEAL_KEY: nothing can be unmasked. */
  sealing_available:     boolean
}

/** One disclosure attempt, as the notifications log lists it. */
export interface NotificationRow {
  id:          string
  repository:  RepoRef
  recipient:   string
  findings:    number
  state:       NoticeState
  error?:      string
  created_at:  string
  sent_at?:    string
}

export interface NotificationFilter {
  state?:  NoticeState | ''
  q?:      string
  limit?:  number
  offset?: number
}

/** A disclosure re-rendered from the findings it covered. */
export interface NotificationMessage { to: string; subject: string; text: string; html: string }

/** A provider the detection engine can attribute a finding to, as the API
 *  derives it from the live ruleset. `severity` is the most serious of the
 *  provider's rules; `rules` is how many target it. */
export interface Provider {
  slug:     string
  severity: Severity
  rules:    number
  /** One entry per rule. Absent from an older api, so read it as optional. */
  rule_details?: RuleDetail[]
}

/** What makes a rule's match trustworthy: the value's own prefix, the setting
 *  name before it, a connection URL, a PEM block, or only the value's layout. */
export type RuleAnchor = 'prefix' | 'setting' | 'url' | 'block' | 'shape'

/** One detection rule, described for people. `example` is a drawing of the
 *  layout ("•" for every variable character), never a real value. */
export interface RuleDetail {
  id:       string
  severity: Severity
  anchor:   RuleAnchor
  shape:    string
  example:  string
  literals: string[]
  keywords: string[]
}

/** One step of a running scan, as streamed by GET /v1/scans/{id}/events.
 *  Live narration only: the job's counters stay the source of truth, and an
 *  event can be lost without any number on the page being wrong. */
export type ProgressStage =
  'discover' | 'queue' | 'clone' | 'walk' | 'attribute' | 'probe' | 'store' | 'repo' | 'job'
export type ProgressStatus =
  'start' | 'progress' | 'done' | 'failed' | 'skipped' | 'deferred' | 'paused' | 'cancelled'

export interface ProgressEvent {
  seq:            number
  job_id:         string
  repo_id?:       string
  repo?:          string
  stage:          ProgressStage
  status:         ProgressStatus
  at:             string
  elapsed_ms?:    number
  attempt?:       number
  blobs?:         number
  matches?:       number
  findings?:      number
  reclassified?:  number
  live?:          number
  page?:          number
  repos?:         number
  path?:          string
  app_type?:      string
  error?:         string
}

/** How much a share link reveals: counts only, or locations too. */
export type ShareDetail = 'summary' | 'full'

/** A read-only link to one repository's results. `token` is present only in
 *  the response that creates it — the server keeps a hash, not the token. */
export interface ShareLink {
  id:               string
  repository_id:    string
  detail:           ShareDetail
  include_resolved: boolean
  state:            'active' | 'expired' | 'revoked'
  created_at:       string
  expires_at:       string
  revoked_at?:      string
  views:            number
  last_viewed_at?:  string
  token?:           string
}

/** What a share link opens. `findings` and `guide` are empty at the summary level. */
export interface SharedReport {
  repo:             string
  repo_url:         string
  detail:           ShareDetail
  include_resolved: boolean
  generated_at:     string
  expires_at:       string
  last_scanned_at?: string
  summary: {
    total: number; open: number; live: number; on_head: number; files: number
    by_severity: Count[]; by_provider: Count[]; by_status: Count[]
  }
  findings: {
    provider: string; rule: string; severity: Severity; status: string; location: string; masked: string
    commit?: string; committed_at?: string; link?: string; on_head: boolean; live: boolean
  }[]
  guide: { title: string; text: string[]; commands: string[] }[]
}

export interface Count { key: string; count: number }
export interface TimePoint { period: string; findings: number }

export interface Analytics {
  timeline:     TimePoint[]
  by_app_type:  Count[]
  by_rule:      Count[]
  live_probes:  Count[]
  notifications: Count[]
}

export interface Overview {
  findings:            number
  open_findings:       number
  reachable_from_head: number
  repositories:        number
  exposed_repos:       number
  active_scans:        number
  candidates:          number
  notified_repos:      number
  awaiting_notice:     number
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
