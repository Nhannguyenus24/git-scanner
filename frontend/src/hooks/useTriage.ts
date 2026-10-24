import { useCallback } from 'react'
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
