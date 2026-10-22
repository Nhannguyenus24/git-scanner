import { useState, useEffect } from 'react'
import { createClient } from '../api/client'

export type ApiStatus = 'checking' | 'ok' | 'error'

export function useApiStatus(baseUrl: string): ApiStatus {
  const [status, setStatus] = useState<ApiStatus>('checking')

  useEffect(() => {
    setStatus('checking')
    createClient(baseUrl)
      .version()
      .then(() => setStatus('ok'))
      .catch(() => setStatus('error'))
  }, [baseUrl])

  return status
}
