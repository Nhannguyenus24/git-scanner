import { useEffect, useState } from 'react'
import { createClient } from '../api/client'

export type ApiStatus = 'checking' | 'ok' | 'error'

/** Probes the API on change and then every 15s, so a restart shows up without a reload. */
export function useApiStatus(baseUrl: string): ApiStatus {
  const [status, setStatus] = useState<ApiStatus>('checking')
  useEffect(() => {
    let live = true
    const probe = () => createClient(baseUrl).version()
      .then(() => live && setStatus('ok'))
      .catch(() => live && setStatus('error'))
    setStatus('checking')
    probe()
    const t = setInterval(probe, 15000)
    return () => { live = false; clearInterval(t) }
  }, [baseUrl])
  return status
}
