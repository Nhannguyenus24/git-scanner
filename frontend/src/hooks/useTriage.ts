import { useCallback, useState } from 'react'
import { useSettings } from '../store/settings'
import { useToast } from '../store/toast'
import { useT } from '../i18n'
import type { Finding, FindingStatus } from '../types'

/** Applies a triage decision and refreshes whatever list is showing it. */
export function useTriage(reload: () => void) {
  const { api } = useSettings()
  const notify  = useToast()
  const { t }   = useT()
  return useCallback((f: Finding, status: FindingStatus) => {
    if (f.status === status) return
    api.setStatus(f.id, status)
      .then(() => { notify(`${t(`status.${status}`)} — ${f.repository.name}/${f.file_path.split('/').pop()}`); reload() })
      .catch(e => notify(e instanceof Error ? e.message : 'Error', 'error'))
  }, [api, notify, t, reload])
}

/** All bulk triage needs of a finding — so a view holding only ids can use it. */
export type TriageTarget = Pick<Finding, 'id' | 'status'>

export interface BulkTriage {
  run:     (findings: TriageTarget[], status: FindingStatus) => Promise<void>
  running: boolean
}

/**
 * Bulk triage, one PATCH per finding.
 *
 * The API has no batch endpoint, so this is a fan-out — but it is a *reported*
 * fan-out: every call is awaited, failures are counted, and the toast says how
 * many of how many landed. A bulk action that half-worked and said "done" is
 * the worst outcome, because the list reloads and looks plausible either way.
 *
 * Concurrency is capped so marking two hundred findings does not open two
 * hundred sockets at once; findings already in the target state are skipped
 * rather than sent and ignored.
 */
export function useBulkTriage(reload: () => void): BulkTriage {
  const { api } = useSettings()
  const notify  = useToast()
  const { t }   = useT()
  const [running, setRunning] = useState(false)

  const run = useCallback(async (findings: TriageTarget[], status: FindingStatus) => {
    const todo = findings.filter(f => f.status !== status)
    if (!todo.length) { notify(t('bulk.nothing'), 'info'); return }

    setRunning(true)
    let ok = 0
    const errors: string[] = []
    const queue = [...todo]
    const worker = async () => {
      for (let f = queue.pop(); f; f = queue.pop()) {
        try { await api.setStatus(f.id, status); ok++ }
        catch (e) { errors.push(e instanceof Error ? e.message : String(e)) }
      }
    }
    await Promise.all(Array.from({ length: Math.min(6, queue.length) }, worker))
    setRunning(false)
    reload()

    if (!errors.length) notify(t('bulk.done', { n: ok, status: t(`status.${status}`) }))
    else notify(t('bulk.partial', { ok, total: todo.length, error: errors[0] }), 'error')
  }, [api, notify, t, reload])

  return { run, running }
}
