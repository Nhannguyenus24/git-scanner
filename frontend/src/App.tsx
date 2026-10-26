import { useState } from 'react'
import Layout from './components/Layout'
import NewScanDialog from './components/NewScanDialog'
import { useResource } from './hooks/useResource'
import { href, navigate, useRoute } from './hooks/useRoute'
import CandidatesPage from './pages/CandidatesPage'
import FindingsPage from './pages/FindingsPage'
import HelpPage from './pages/HelpPage'
import AnalyticsPage from './pages/AnalyticsPage'
import OverviewPage from './pages/OverviewPage'
import RepositoriesPage from './pages/RepositoriesPage'
import ScanDetailPage from './pages/ScanDetailPage'
import ScansPage from './pages/ScansPage'
import { useSettings } from './store/settings'
import { useT } from './i18n'
import { useToast } from './store/toast'
import type { TargetKind } from './types'

export default function App() {
  const route  = useRoute()
  const { api } = useSettings()
  const { t } = useT()
  const notify = useToast()
  const [scanOpen, setScanOpen] = useState(false)
  const ov = useResource(() => api.overview(), [api], 10000)

  const startScan = async (kind: TargetKind, value: string, forks: boolean) => {
    const job = kind === 'repo' ? await api.scanRepo(value)
      : kind === 'user' ? await api.scanUser(value, forks)
      : kind === 'org' ? await api.scanOrg(value, forks)
      : await api.scanRandom(Number(value) || 0, forks)
    notify(t('toast.scanQueued', { kind, value }))
    navigate(href('scans', job.id))
  }

  const open = () => setScanOpen(true)
  // /repositories/<id> is a shortcut for that repository's findings.
  const page = route.page === 'repositories' && route.id ? 'findings' : route.page

  let body
  switch (page) {
    case 'scans':        body = route.id ? <ScanDetailPage key={route.id} id={route.id} /> : <ScansPage onNewScan={open} />; break
    case 'findings':     body = <FindingsPage route={route.id && route.page === 'repositories'
                            ? { ...route, params: new URLSearchParams({ repository_id: route.id }) } : route} />; break
    case 'analytics':    body = <AnalyticsPage />; break
    case 'repositories': body = <RepositoriesPage />; break
    case 'candidates':   body = <CandidatesPage />; break
    case 'help':         body = <HelpPage />; break
    default:             body = <OverviewPage onNewScan={open} />
  }

  return (
    <Layout page={page} onNewScan={open}
      badges={{ findings: ov.data?.open_findings ?? 0, scans: ov.data?.active_scans ?? 0, candidates: ov.data?.candidates ?? 0 }}>
      {body}
      <NewScanDialog open={scanOpen} onClose={() => setScanOpen(false)} onSubmit={startScan} />
    </Layout>
  )
}
