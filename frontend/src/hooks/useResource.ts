import { useCallback, useEffect, useRef, useState } from 'react'

export interface Resource<T> {
  data:    T | undefined
  error:   string
  loading: boolean // true only until the first response; refreshes keep the old data on screen
  reload:  () => void
}

/**
 * Loads data and, when pollMs is set, keeps it fresh. A refresh replaces the
 * data in place rather than flashing a spinner, and a response that arrives
 * after a newer request was started is discarded.
 */
export function useResource<T>(load: () => Promise<T>, deps: unknown[], pollMs = 0): Resource<T> {
  const [data,    setData]    = useState<T>()
  const [error,   setError]   = useState('')
  const [loading, setLoading] = useState(true)
  const seq = useRef(0)

  // eslint-disable-next-line react-hooks/exhaustive-deps
  const run = useCallback(() => {
    const mine = ++seq.current
    load()
      .then(d  => { if (mine === seq.current) { setData(d); setError('') } })
      .catch(e => { if (mine === seq.current) setError(e instanceof Error ? e.message : String(e)) })
      .finally(() => { if (mine === seq.current) setLoading(false) })
  }, deps)

  useEffect(() => { setLoading(true); run() }, [run])

  useEffect(() => {
    if (!pollMs) return
    const t = setInterval(() => { if (document.visibilityState === 'visible') run() }, pollMs)
    return () => clearInterval(t)
  }, [run, pollMs])

  return { data, error, loading, reload: run }
}
