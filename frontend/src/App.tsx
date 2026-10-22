import { useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Snackbar from '@mui/material/Snackbar'
import Typography from '@mui/material/Typography'
import JobDetail from './components/JobDetail'
import JobSidebar from './components/JobSidebar'
import NewScanDialog from './components/NewScanDialog'
import TopBar from './components/TopBar'
import { useApiStatus } from './hooks/useApiStatus'
import { usePoller } from './hooks/usePoller'
import { createClient } from './api/client'
import { useStore } from './store/jobs'
import type { LocalJob, TargetKind } from './types'

interface Toast { msg: string; severity: 'success' | 'error' }

export default function App() {
  const { state, dispatch } = useStore()
  const { jobs, selectedId, baseUrl } = state
  const apiStatus = useApiStatus(baseUrl)
  usePoller()

  const [scanOpen, setScanOpen] = useState(false)
  const [toast,    setToast]    = useState<Toast | null>(null)

  const selected = jobs.find(j => j.id === selectedId) ?? null

  const handleNewScan = async (kind: TargetKind, value: string, forks: boolean) => {
    const api = createClient(baseUrl)
    let raw
    if (kind === 'repo')      raw = await api.scanRepo(value)
    else if (kind === 'user') raw = await api.scanUser(value, forks)
    else                      raw = await api.scanOrg(value, forks)

    const job: LocalJob = { ...raw, target: { kind, value }, submittedAt: new Date().toISOString() }
    dispatch({ type: 'ADD_JOB', job })
    setToast({ msg: `Scan queued — ${kind}: ${value}`, severity: 'success' })
  }

  const handleAddById = (id: string) => {
    const existing = jobs.find(j => j.id === id)
    if (existing) { dispatch({ type: 'SELECT', id }); return }

    createClient(baseUrl)
      .getJob(id)
      .then(raw => {
        const job: LocalJob = { ...raw, target: { kind: 'unknown', value: id.slice(0, 8) + '…' }, submittedAt: new Date().toISOString() }
        dispatch({ type: 'ADD_JOB', job })
      })
      .catch(e => setToast({ msg: e instanceof Error ? e.message : 'Not found', severity: 'error' }))
  }

  const handleAction = async (action: 'pause' | 'resume' | 'cancel', id: string) => {
    const api = createClient(baseUrl)
    try {
      if (action === 'pause')  await api.pause(id)
      if (action === 'resume') await api.resume(id)
      if (action === 'cancel') await api.cancel(id)
      const updated = await api.getJob(id)
      dispatch({ type: 'UPDATE_JOB', id, updates: updated })
      setToast({ msg: `${action} applied`, severity: 'success' })
    } catch (e) {
      setToast({ msg: e instanceof Error ? e.message : 'Error', severity: 'error' })
    }
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <TopBar
        baseUrl={baseUrl}
        onBaseUrl={url => dispatch({ type: 'SET_BASE', url })}
        status={apiStatus}
      />

      <Box sx={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
        <JobSidebar
          jobs={jobs}
          selectedId={selectedId}
          onSelect={id => dispatch({ type: 'SELECT', id })}
          onNewScan={() => setScanOpen(true)}
          onAddById={handleAddById}
        />

        <Box sx={{ flex: 1, overflow: 'auto', minWidth: 0 }}>
          {selected ? (
            <JobDetail key={selected.id} job={selected} baseUrl={baseUrl} onAction={handleAction} />
          ) : (
            <Box sx={{
              height: '100%', display: 'flex', flexDirection: 'column',
              alignItems: 'center', justifyContent: 'center',
              color: 'text.secondary', gap: 1, p: 3, textAlign: 'center',
            }}>
              <Typography sx={{ fontSize: 48, lineHeight: 1 }}>🔍</Typography>
              <Typography variant="h6" sx={{ fontWeight: 700, color: 'text.primary' }}>Select a scan</Typography>
              <Typography variant="body2">Start a new scan or choose one from the sidebar.</Typography>
              <Button variant="contained" sx={{ mt: 1 }} onClick={() => setScanOpen(true)}>
                New Scan
              </Button>
            </Box>
          )}
        </Box>
      </Box>

      <NewScanDialog open={scanOpen} onClose={() => setScanOpen(false)} onSubmit={handleNewScan} />

      <Snackbar
        open={!!toast}
        autoHideDuration={3500}
        onClose={() => setToast(null)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      >
        {toast
          ? <Alert severity={toast.severity} onClose={() => setToast(null)} variant="filled">{toast.msg}</Alert>
          : undefined}
      </Snackbar>
    </Box>
  )
}
