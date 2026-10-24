import { useCallback } from 'react'
import { STATUS_LABEL } from '../format'
import { useSettings } from '../store/settings'
import { useToast } from '../store/toast'
import type { Finding, FindingStatus } from '../types'

/** Applies a triage decision and refreshes whatever list is showing it. */
export function useTriage(reload: () => void) {
  const { api } = useSettings()
  const notify  = useToast()
  return useCallback((f: Finding, status: FindingStatus) => {
    if (f.status === status) return
    api.setStatus(f.id, status)
      .then(() => { notify(`Marked ${STATUS_LABEL[status].toLowerCase()} — ${f.repository.name}/${f.file_path.split('/').pop()}`); reload() })
      .catch(e => notify(e instanceof Error ? e.message : 'Update failed', 'error'))
  }, [api, notify, reload])
}
