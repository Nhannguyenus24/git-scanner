import React, { createContext, useContext, useEffect, useMemo, useState } from 'react'
import { createClient, type ApiClient } from '../api/client'

// Only the API address is remembered in the browser. Everything the dashboard
// shows comes from the server: an earlier version kept the job list in
// localStorage, so scans started anywhere else — the CLI, another browser, or
// before a database wipe — were invisible or pointed at jobs that were gone.

interface Ctx { baseUrl: string; setBaseUrl: (u: string) => void; api: ApiClient }
const SettingsCtx = createContext<Ctx | null>(null)

function loadBase(): string {
  try { return localStorage.getItem('gs-base') || 'http://localhost:8080' } catch { return 'http://localhost:8080' }
}

export function SettingsProvider({ children }: { children: React.ReactNode }) {
  const [baseUrl, setBaseUrl] = useState(loadBase)
  useEffect(() => {
    try {
      localStorage.setItem('gs-base', baseUrl)
      localStorage.removeItem('gs-jobs') // the old client-side job list; never read again
    } catch { /* storage unavailable: the default address still works */ }
  }, [baseUrl])
  const api = useMemo(() => createClient(baseUrl), [baseUrl])
  return <SettingsCtx.Provider value={{ baseUrl, setBaseUrl, api }}>{children}</SettingsCtx.Provider>
}

export function useSettings(): Ctx {
  const ctx = useContext(SettingsCtx)
  if (!ctx) throw new Error('useSettings must be inside SettingsProvider')
  return ctx
}
