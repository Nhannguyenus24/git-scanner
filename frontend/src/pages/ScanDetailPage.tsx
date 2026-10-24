import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Breadcrumbs from '@mui/material/Breadcrumbs'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Grid from '@mui/material/Grid'
import LinearProgress from '@mui/material/LinearProgress'
import Link from '@mui/material/Link'
import Typography from '@mui/material/Typography'
import PauseIcon from '@mui/icons-material/Pause'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import StopIcon from '@mui/icons-material/Stop'
import FindingsTable from '../components/FindingsTable'
import { PageHeader } from '../components/Layout'
import StatCard from '../components/StatCard'
import StateBadge from '../components/StateBadge'
import { Empty, LoadError, Loading } from '../components/States'
import { duration, num, timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { href } from '../hooks/useRoute'
import { useTriage } from '../hooks/useTriage'
import { useSettings } from '../store/settings'
import { useToast } from '../store/toast'
import { TERMINAL } from '../types'

export default function ScanDetailPage({ id }: { id: string }) {
  const { api } = useSettings()
  const notify  = useToast()
  // The list endpoint carries the target and timings; polled until the job ends.
  const scans    = useResource(() => api.listScans(500), [api], 3000)
  const scan     = scans.data?.scans.find(s => s.id === id)
  const active   = scan ? !TERMINAL.has(scan.state) : true
  const findings = useResource(() => api.findings({ scan_id: id, limit: 500 }), [api, id], active ? 4000 : 0)
  const triage   = useTriage(findings.reload)

  if (scans.loading) return <Loading />
  if (scans.error && !scans.data) return <LoadError error={scans.error} retry={scans.reload} />
  if (!scan) return <Empty title="Scan not found" body="It may have been removed by a data wipe." action={<Button href={href('scans')}>Back to scans</Button>} />

  const act = (name: 'pause' | 'resume' | 'cancel') =>
    api[name](id).then(() => { notify(`Scan ${name}d`); scans.reload() })
      .catch(e => notify(e instanceof Error ? e.message : 'Failed', 'error'))

  return (
    <>
      <Breadcrumbs sx={{ mb: 1, fontSize: 13 }}>
        <Link href={href('scans')} underline="hover" color="inherit">Scans</Link>
        <Typography color="text.primary" sx={{ fontSize: 13, fontFamily: 'monospace' }}>{id.slice(0, 8)}</Typography>
      </Breadcrumbs>
      <PageHeader
        title={<Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>{scan.target.value} <StateBadge state={scan.state} size="medium" /></Box>}
        subtitle={`${scan.target.kind} scan · submitted ${timeAgo(scan.created_at)} · ${duration(scan.started_at, scan.finished_at)}`}
        actions={<>
          {scan.state === 'running' && <Button startIcon={<PauseIcon />} onClick={() => act('pause')}>Pause</Button>}
          {scan.state === 'paused'  && <Button startIcon={<PlayArrowIcon />} onClick={() => act('resume')}>Resume</Button>}
          {!TERMINAL.has(scan.state) && <Button color="error" startIcon={<StopIcon />} onClick={() => act('cancel')}>Cancel</Button>}
        </>} />

      {scan.error && <Alert severity="error" sx={{ mb: 2 }}>{scan.error}</Alert>}

      <Card sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', fontSize: 13, mb: 1 }}>
          <span>{scan.repos_scanned} of {scan.repos_total || '?'} repositories</span>
          <span>{Math.round(scan.progress * 100)}%</span>
        </Box>
        <LinearProgress variant={scan.state === 'queued' ? 'indeterminate' : 'determinate'} value={scan.progress * 100}
          sx={{ height: 8, borderRadius: 4 }} color={scan.state === 'failed' ? 'error' : TERMINAL.has(scan.state) ? 'success' : 'primary'} />
      </Card>

      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid item xs={6} md={3}><StatCard label="Repositories" value={scan.repos_total} /></Grid>
        <Grid item xs={6} md={3}><StatCard label="Blobs scanned" value={num(scan.blobs_scanned)} /></Grid>
        <Grid item xs={6} md={3}><StatCard label="New findings" value={scan.findings_count} hint="first seen by this job" accent="#f5a623" /></Grid>
        <Grid item xs={6} md={3}><StatCard label="Open in this job" value={findings.data?.findings.filter(f => f.status === 'open').length ?? '…'} accent="#f07070" /></Grid>
      </Grid>

      <Card>
        <Box sx={{ px: 2, py: 1.5, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Typography sx={{ fontWeight: 700 }}>Findings recorded by this scan</Typography>
          <Link href={href('findings', undefined, { scan_id: id })} underline="hover" sx={{ fontSize: 13 }}>Open in Findings →</Link>
        </Box>
        {findings.loading ? <Loading /> : (
          <FindingsTable findings={findings.data?.findings ?? []} onStatus={triage}
            emptyMessage={active ? 'Nothing yet — results appear as repositories finish.' : 'This scan recorded no new findings. A credential already found by an earlier scan stays attributed to that one.'} />
        )}
      </Card>
    </>
  )
}
