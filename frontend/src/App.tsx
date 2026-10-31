import { useState } from 'react'
import CommandPalette from './components/CommandPalette'
import Layout from './components/Layout'
import NewScanDialog from './components/NewScanDialog'
import OnboardingTour from './components/OnboardingTour'
import { useHotkeys } from './hooks/useHotkeys'
import { useResource } from './hooks/useResource'
import { href, navigate, useRoute, type Route } from './hooks/useRoute'
import CandidatesPage from './pages/CandidatesPage'
import NotificationsPage from './pages/NotificationsPage'
import SettingsPage from './pages/SettingsPage'
import FindingsPage from './pages/FindingsPage'
import HelpPage from './pages/HelpPage'
import AnalyticsPage from './pages/AnalyticsPage'
import OverviewPage from './pages/OverviewPage'
import ProvidersPage from './pages/ProvidersPage'
import RepositoriesPage from './pages/RepositoriesPage'
import ScanDetailPage from './pages/ScanDetailPage'
import ScansPage from './pages/ScansPage'
import SharedReportPage from './pages/SharedReportPage'
import { usePrefs } from './store/preferences'
import { useSettings } from './store/settings'
import { useT } from './i18n'
import { useToast } from './store/toast'
import type { TargetKind } from './types'

export default function App() {
  const route = useRoute()
  // A share link opens one repository's report and nothing else: no dashboard,
  // no navigation, none of the requests the dashboard makes.
  if (route.page === 'shared' && route.id) return <SharedReportPage key={route.id} token={route.id} />
  return <Dashboard route={route} />
}

function Dashboard({ route }: { route: Route }) {
  const { api } = useSettings()
  const { t } = useT()
  const notify = useToast()
  const { prefs, set } = usePrefs()
  const [scanOpen, setScanOpen] = useState(false)
  const [paletteOpen, setPaletteOpen] = useState(false)
  // The tour is offered once per browser. `tourOpen` can reopen it from settings
  // without clearing the completion flag.
  const [tourOpen, setTourOpen] = useState(!prefs.onboarded)
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

  // The global shortcuts. Everything else the palette offers is reachable
  // through it, which keeps the number of keys anyone has to learn at three.
  useHotkeys([
    { key: 'k', mod: true, inText: true, run: () => setPaletteOpen(o => !o) },
    { key: '[', mod: true, inText: true, run: () => set('sidebarCollapsed', !prefs.sidebarCollapsed) },
    { key: 'n', run: open },
    { key: '/', run: () => setPaletteOpen(true) },
  ], !tourOpen)

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
    case 'notifications': body = <NotificationsPage />; break
    case 'settings':     body = <SettingsPage onRestartTour={() => setTourOpen(true)} />; break
    case 'providers':    body = <ProvidersPage />; break
    case 'help':         body = <HelpPage />; break
    default:             body = <OverviewPage onNewScan={open} />
  }

  return (
    <Layout page={page} onNewScan={open} onCommands={() => setPaletteOpen(true)}
      badges={{ findings: ov.data?.open_findings ?? 0, scans: ov.data?.active_scans ?? 0, candidates: ov.data?.candidates ?? 0 }}>
      {body}
      <NewScanDialog open={scanOpen} onClose={() => setScanOpen(false)} onSubmit={startScan} />
      <CommandPalette open={paletteOpen} onClose={() => setPaletteOpen(false)} onNewScan={open} />
      <OnboardingTour open={tourOpen} onClose={() => setTourOpen(false)} onNewScan={open} />
    </Layout>
  )
}
