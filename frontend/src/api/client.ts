import type {
  AppSettings, Analytics, Count, NotificationFilter, NotificationMessage, NotificationRow, Candidate, Finding, FindingFilter, FindingStatus, Overview, Provider, Repository, ScanJob, ScanSummary,
} from '../types'

export interface VersionResponse { service: string; version: string; commit: string; go_version: string }

async function req<T>(base: string, method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  const opts: RequestInit = { method, headers }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res  = await fetch(base + path, opts)
  const json = await res.json().catch(() => ({})) as Record<string, unknown>
  if (!res.ok) throw new Error((json['error'] as string | undefined) ?? `HTTP ${res.status}`)
  return json as T
}

function query(params: object): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params)) {
    if (v !== undefined && v !== '' && v !== null) q.set(k, String(v))
  }
  const s = q.toString()
  return s ? `?${s}` : ''
}

export function createClient(rawBase: string) {
  const base = rawBase.replace(/\/$/, '')
  const r    = <T>(method: string, path: string, body?: unknown) => req<T>(base, method, path, body)

  return {
    version:      ()                                     => r<VersionResponse>('GET', '/v1/version'),
    overview:     ()                                     => r<Overview>('GET', '/v1/overview'),
    analytics:    ()                                     => r<Analytics>('GET', '/v1/analytics'),
    listScans:    (limit = 100)                          => r<{ scans: ScanSummary[] }>('GET', `/v1/scans?limit=${limit}`),
    getScan:      (id: string)                           => r<ScanJob>('GET', `/v1/scans/${id}`),
    scanRepo:     (url: string)                          => r<ScanJob>('POST', '/v1/scans/repo', { url }),
    scanUser:     (name: string, include_forks: boolean) => r<ScanJob>('POST', '/v1/scans/user', { name, include_forks }),
    scanOrg:      (name: string, include_forks: boolean) => r<ScanJob>('POST', '/v1/scans/org',  { name, include_forks }),
    scanRandom:   (count: number, include_forks: boolean) => r<ScanJob>('POST', '/v1/scans/random', { count, include_forks }),
    pause:        (id: string)                           => r<unknown>('POST', `/v1/scans/${id}/pause`),
    resume:       (id: string)                           => r<unknown>('POST', `/v1/scans/${id}/resume`),
    cancel:       (id: string)                           => r<unknown>('POST', `/v1/scans/${id}/cancel`),
    rescan:       (id: string)                           => r<ScanJob>('POST', `/v1/scans/${id}/rescan`),
    deleteScan:   (id: string)                           => r<unknown>('DELETE', `/v1/scans/${id}`),
    findings:     (f: FindingFilter)                     => r<{ findings: Finding[]; total: number }>('GET', `/v1/findings${query(f)}`),
    setStatus:    (id: string, status: FindingStatus)    => r<unknown>('PATCH', `/v1/findings/${id}`, { status }),
    repositories: ()                                     => r<{ repositories: Repository[] }>('GET', '/v1/repositories'),
    candidates:   ()                                     => r<{ candidates: Candidate[] }>('GET', '/v1/candidates'),
    providers:    ()                                     => r<{ providers: Provider[] }>('GET', '/v1/providers'),
    disclose:     (repoId: string)                       => r<{ recipient: string; findings: number }>('POST', `/v1/repositories/${repoId}/disclose`, {}),
    settings:     ()                                     => r<AppSettings>('GET', '/v1/settings'),
    setSettings:  (s: Partial<Pick<AppSettings, 'notifications_enabled' | 'masking_enabled'>>) => r<AppSettings>('PUT', '/v1/settings', s),
    addUnmaskOwner:    (owner: string)                   => r<AppSettings>('PUT', `/v1/settings/unmask-owners/${encodeURIComponent(owner)}`),
    removeUnmaskOwner: (owner: string)                   => r<AppSettings>('DELETE', `/v1/settings/unmask-owners/${encodeURIComponent(owner)}`),
    notifications: (f: NotificationFilter)               => r<{ notifications: NotificationRow[]; total: number; by_state: Count[] }>('GET', `/v1/notifications${query(f)}`),
    notificationMessage: (id: string)                    => r<NotificationMessage>('GET', `/v1/notifications/${id}/message`),
    resendNotification:  (id: string)                    => r<unknown>('POST', `/v1/notifications/${id}/resend`),
  }
}

export type ApiClient = ReturnType<typeof createClient>
