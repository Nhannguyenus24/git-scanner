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
import { useT } from '../i18n'
import { useToast } from '../store/toast'
import { TERMINAL } from '../types'

export default function ScanDetailPage({ id }: { id: string }) {
  const { api } = useSettings()
  const { t } = useT()
  const notify  = useToast()
  // The list endpoint carries the target and timings; polled until the job ends.
  const scans    = useResource(() => api.listScans(500), [api], 3000)
  const scan     = scans.data?.scans.find(s => s.id === id)
  const active   = scan ? !TERMINAL.has(scan.state) : true
  const findings = useResource(() => api.findings({ scan_id: id, limit: 500 }), [api, id], active ? 4000 : 0)
  const triage   = useTriage(findings.reload)

  if (scans.loading) return <Loading />
  if (scans.error && !scans.data) return <LoadError error={scans.error} retry={scans.reload} />
  if (!scan) return <Empty title={t('scan.notFound')} body={t('scan.notFoundBody')} action={<Button href={href('scans')}>{t('scan.back')}</Button>} />

  const act = (name: 'pause' | 'resume' | 'cancel') =>
    api[name](id).then(() => { notify(`Scan ${name}d`); scans.reload() })
      .catch(e => notify(e instanceof Error ? e.message : 'Failed', 'error'))

  return (
    <>
      <Breadcrumbs sx={{ mb: 1, fontSize: 13 }}>
        <Link href={href('scans')} underline="hover" color="inherit">{t('scans.title')}</Link>
        <Typography color="text.primary" sx={{ fontSize: 13, fontFamily: 'monospace' }}>{id.slice(0, 8)}</Typography>
      </Breadcrumbs>
      <PageHeader
        title={<Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>{scan.target.value} <StateBadge state={scan.state} size="medium" /></Box>}
        subtitle={t('scan.submitted', { kind: scan.target.kind, ago: timeAgo(scan.created_at), dur: duration(scan.started_at, scan.finished_at) })}
        actions={<>
          {scan.state === 'running' && <Button startIcon={<PauseIcon />} onClick={() => act('pause')}>{t('scan.pause')}</Button>}
          {scan.state === 'paused'  && <Button startIcon={<PlayArrowIcon />} onClick={() => act('resume')}>{t('scan.resume')}</Button>}
          {!TERMINAL.has(scan.state) && <Button color="error" startIcon={<StopIcon />} onClick={() => act('cancel')}>{t('scan.cancel')}</Button>}
        </>} />

      {scan.error && <Alert severity="error" sx={{ mb: 2 }}>{scan.error}</Alert>}

      <Card sx={{ p: 2, mb: 2 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', fontSize: 13, mb: 1 }}>
          <span>{t('scan.reposProgress', { a: scan.repos_scanned, b: scan.repos_total || '?' })}</span>
          <span>{Math.round(scan.progress * 100)}%</span>
        </Box>
        <LinearProgress variant={scan.state === 'queued' ? 'indeterminate' : 'determinate'} value={scan.progress * 100}
          sx={{ height: 8, borderRadius: 4 }} color={scan.state === 'failed' ? 'error' : TERMINAL.has(scan.state) ? 'success' : 'primary'} />
      </Card>

      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid item xs={6} md={3}><StatCard label={t('scan.reposCount')} value={scan.repos_total} /></Grid>
        <Grid item xs={6} md={3}><StatCard label={t('scan.blobsScanned')} value={num(scan.blobs_scanned)} /></Grid>
        <Grid item xs={6} md={3}><StatCard label={t('scan.newFindings')} value={scan.findings_count} hint={t('scan.newFindingsHint')} accent="#a9781a" /></Grid>
        <Grid item xs={6} md={3}><StatCard label={t('scan.openInJob')} value={findings.data?.findings.filter(f => f.status === 'open').length ?? '…'} accent="#9c3b2e" /></Grid>
      </Grid>

      <Card>
        <Box sx={{ px: 2, py: 1.5, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Typography sx={{ fontWeight: 700 }}>{t('scan.findingsTitle')}</Typography>
          <Link href={href('findings', undefined, { scan_id: id })} underline="hover" sx={{ fontSize: 13 }}>{t('scan.openInFindings')}</Link>
        </Box>
        {findings.loading ? <Loading /> : (
          <FindingsTable findings={findings.data?.findings ?? []} onStatus={triage}
            emptyMessage={active ? t('scan.emptyActive') : t('scan.emptyDone')} />
        )}
      </Card>
    </>
  )
}
