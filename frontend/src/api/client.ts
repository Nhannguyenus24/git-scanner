import type { ScanJob, Finding } from '../types'

export interface FindingsResponse { findings: Finding[] }
export interface VersionResponse  { service: string; version: string; commit: string; go_version: string }

async function req<T>(base: string, method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  const opts: RequestInit = { method, headers }
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(base + path, opts)
  const json = await res.json().catch(() => ({})) as Record<string, unknown>
  if (!res.ok) throw new Error((json['error'] as string | undefined) ?? `HTTP ${res.status}`)
  return json as T
}

export function createClient(rawBase: string) {
  // When running via `vite dev` the proxy rewrites /v1 → localhost:8080/v1,
  // so an empty base works. For a custom host the user sets it in the sidebar.
  const base = rawBase.replace(/\/$/, '')
  const r    = <T>(method: string, path: string, body?: unknown) => req<T>(base, method, path, body)

  return {
    version:     ()                                          => r<VersionResponse>('GET', '/v1/version'),
    scanRepo:    (url: string)                               => r<ScanJob>('POST', '/v1/scans/repo',  { url }),
    scanUser:    (name: string, include_forks: boolean)      => r<ScanJob>('POST', '/v1/scans/user',  { name, include_forks }),
    scanOrg:     (name: string, include_forks: boolean)      => r<ScanJob>('POST', '/v1/scans/org',   { name, include_forks }),
    getJob:      (id: string)                                => r<ScanJob>('GET',  `/v1/scans/${id}`),
    getFindings: (id: string)                                => r<FindingsResponse>('GET', `/v1/scans/${id}/findings`),
    pause:       (id: string)                                => r<unknown>('POST', `/v1/scans/${id}/pause`),
    resume:      (id: string)                                => r<unknown>('POST', `/v1/scans/${id}/resume`),
    cancel:      (id: string)                                => r<unknown>('POST', `/v1/scans/${id}/cancel`),
  }
}

export type ApiClient = ReturnType<typeof createClient>
