import React, { createContext, useContext, useReducer, useEffect } from 'react'
import type { LocalJob } from '../types'

// ── state ─────────────────────────────────────────────────────────────────────
interface State {
  jobs:       LocalJob[]
  selectedId: string | null
  baseUrl:    string
}

type Action =
  | { type: 'ADD_JOB';    job: LocalJob }
  | { type: 'UPDATE_JOB'; id: string; updates: Partial<LocalJob> }
  | { type: 'SELECT';     id: string | null }
  | { type: 'SET_BASE';   url: string }

function reducer(s: State, a: Action): State {
  switch (a.type) {
    case 'ADD_JOB':
      return { ...s, jobs: [a.job, ...s.jobs], selectedId: a.job.id }
    case 'UPDATE_JOB':
      return { ...s, jobs: s.jobs.map(j => j.id === a.id ? { ...j, ...a.updates } : j) }
    case 'SELECT':
      return { ...s, selectedId: a.id }
    case 'SET_BASE':
      return { ...s, baseUrl: a.url }
  }
}

function load<T>(key: string, fallback: T): T {
  try { return JSON.parse(localStorage.getItem(key) ?? 'null') ?? fallback } catch { return fallback }
}

const initial: State = {
  jobs:       load('gs-jobs', []),
  selectedId: null,
  baseUrl:    load('gs-base', 'http://localhost:8080'),
}

// ── context ───────────────────────────────────────────────────────────────────
type Ctx = { state: State; dispatch: React.Dispatch<Action> }
const StoreCtx = createContext<Ctx | null>(null)

export function StoreProvider({ children }: { children: React.ReactNode }) {
  const [state, dispatch] = useReducer(reducer, initial)

  useEffect(() => { try { localStorage.setItem('gs-jobs', JSON.stringify(state.jobs)) } catch {} }, [state.jobs])
  useEffect(() => { try { localStorage.setItem('gs-base', state.baseUrl) } catch {} }, [state.baseUrl])

  return <StoreCtx.Provider value={{ state, dispatch }}>{children}</StoreCtx.Provider>
}

export function useStore(): Ctx {
  const ctx = useContext(StoreCtx)
  if (!ctx) throw new Error('useStore must be inside StoreProvider')
  return ctx
}
