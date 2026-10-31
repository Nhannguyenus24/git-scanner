import { useEffect, useRef } from 'react'
import { diff, snapshot, type Alert, type Snapshot } from '../alerts'
import { href, navigate } from './useRoute'
import { useSettings } from '../store/settings'
import { useToast } from '../store/toast'
import { useT } from '../i18n'

// Browser notifications for a finished scan or a newly confirmed live key.
//
// These are local notifications from an open dashboard — a background tab is
// enough, a closed one is not. Push to a closed browser needs a push service
// and stored subscriptions on the server, which do not exist; this does not
// pretend otherwise.
//
// While the tab is visible the same news goes to the in-page toast instead, so
// nobody gets a system notification about the page they are looking at. Every
// notification carries a tag, so two open tabs replace each other's rather
// than stacking duplicates.

export const alertsSupported = () => typeof window !== 'undefined' && 'Notification' in window

const POLL_MS = 20000

export function useDesktopAlerts(enabled: boolean) {
  const { api } = useSettings()
  const notify = useToast()
  const { t } = useT()
  const last = useRef<Snapshot | null>(null)

  useEffect(() => {
    last.current = null
    if (!enabled || !alertsSupported()) return
    let stopped = false

    const say = (a: Alert) => {
      const title = a.kind === 'scan'
        ? t(a.state === 'completed' ? 'alerts.scanDone' : 'alerts.scanEnded', { target: a.target, state: t(`state.${a.state}`) })
        : t('alerts.live', { n: a.added })
      const body = a.kind === 'scan' ? t('alerts.scanBody', { n: a.findings }) : t('alerts.liveBody')
      const to = a.kind === 'scan' ? href('scans', a.id) : href('findings', undefined, { exposure: 'live', status: 'open' })
      if (!document.hidden || Notification.permission !== 'granted') {
        notify(`${title} — ${body}`, a.kind === 'live' ? 'error' : 'info')
        return
      }
      const n = new Notification(title, { body, tag: a.kind === 'scan' ? `scan-${a.id}` : 'live-keys', icon: '/icon.svg' })
      n.onclick = () => { window.focus(); navigate(to); n.close() }
    }

    const poll = async () => {
      try {
        const [{ scans }, analytics] = await Promise.all([api.listScans({ limit: 30 }), api.analytics()])
        const live = analytics.live_probes.find(c => c.key === 'live')?.count ?? 0
        if (stopped) return
        diff(last.current, scans, live).forEach(say)
        last.current = snapshot(scans, live)
      } catch {
        // An unreachable API is already shown in the top bar; a poll that
        // fails simply leaves the baseline where it was.
      }
    }
    poll()
    const timer = window.setInterval(poll, POLL_MS)
    return () => { stopped = true; window.clearInterval(timer) }
  }, [enabled, api, notify, t])
}
