import { useEffect, useReducer, useRef, useState } from 'react'
import { applyEvent, emptyLive, type LiveState } from '../live'
import type { ProgressEvent } from '../types'

/** Where the stream stands. 'unavailable' covers a browser without
 *  EventSource and a server without live progress; the page then falls back to
 *  the counters alone, which it shows either way. */
export type StreamStatus = 'connecting' | 'live' | 'reconnecting' | 'ended' | 'unavailable'

type Action = { type: 'event'; ev: ProgressEvent } | { type: 'reset' }

function reducer(s: LiveState, a: Action): LiveState {
  return a.type === 'reset' ? emptyLive : applyEvent(s, a.ev)
}

/**
 * Subscribes to a scan's live progress while `enabled`.
 *
 * `onSettled` runs when a repository or the job finishes, debounced, so the
 * page can refresh its counters and findings straight away instead of waiting
 * for the next poll.
 */
export function useScanEvents(url: string, enabled: boolean, onSettled?: () => void) {
  const [state, dispatch] = useReducer(reducer, emptyLive)
  const [status, setStatus] = useState<StreamStatus>(enabled ? 'connecting' : 'ended')
  const settled = useRef(onSettled)
  settled.current = onSettled

  useEffect(() => { dispatch({ type: 'reset' }) }, [url])

  useEffect(() => {
    if (!enabled) { setStatus(s => (s === 'unavailable' ? s : 'ended')); return }
    if (typeof EventSource === 'undefined') { setStatus('unavailable'); return }

    let timer: ReturnType<typeof setTimeout> | undefined
    const settle = () => {
      clearTimeout(timer)
      timer = setTimeout(() => settled.current?.(), 400)
    }

    setStatus('connecting')
    const es = new EventSource(url)
    es.onopen = () => setStatus('live')
    es.onmessage = (m: MessageEvent<string>) => {
      let ev: ProgressEvent
      try { ev = JSON.parse(m.data) } catch { return }
      dispatch({ type: 'event', ev })
      if (ev.stage === 'job' || (ev.stage === 'repo' && ev.status !== 'start')) settle()
    }
    es.addEventListener('end', () => { es.close(); setStatus('ended'); settle() })
    es.onerror = () => {
      // CLOSED means the browser gave up — the route answered with an error,
      // such as a server without live progress. Otherwise it is retrying, and
      // the Last-Event-ID it sends resumes the stream where it broke.
      setStatus(es.readyState === EventSource.CLOSED ? 'unavailable' : 'reconnecting')
    }
    return () => { clearTimeout(timer); es.close() }
  }, [url, enabled])

  return { live: state, status }
}
