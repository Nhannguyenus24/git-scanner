import { useEffect, useRef } from 'react'
import { createClient } from '../api/client'
import { useStore } from '../store/jobs'
import { TERMINAL } from '../types'

/** Polls all non-terminal jobs every 3 s and pushes state updates to the store. */
export function usePoller() {
  const { state: { jobs, baseUrl }, dispatch } = useStore()
  const timerRef = useRef<ReturnType<typeof setInterval>>()

  // key changes when any job's id/state changes → restarts the effect
  const activeKey = jobs
    .filter(j => !TERMINAL.has(j.state))
    .map(j => j.id)
    .join(',')

  useEffect(() => {
    clearInterval(timerRef.current)

    const active = jobs.filter(j => !TERMINAL.has(j.state))
    if (!active.length) return

    const api = createClient(baseUrl)

    const poll = async () => {
      const results = await Promise.allSettled(active.map(j => api.getJob(j.id)))
      results.forEach((r, i) => {
        if (r.status === 'fulfilled') {
          dispatch({ type: 'UPDATE_JOB', id: active[i].id, updates: r.value })
        }
      })
    }

    timerRef.current = setInterval(poll, 3_000)
    return () => clearInterval(timerRef.current)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeKey, baseUrl])
}
