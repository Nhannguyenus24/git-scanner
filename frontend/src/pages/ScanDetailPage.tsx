import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Breadcrumbs from '@mui/material/Breadcrumbs'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Grid from '@mui/material/Grid'
import Link from '@mui/material/Link'
import Typography from '@mui/material/Typography'
import PauseIcon from '@mui/icons-material/Pause'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import StopIcon from '@mui/icons-material/Stop'
import ReplayIcon from '@mui/icons-material/Replay'
import DeleteIcon from '@mui/icons-material/DeleteOutline'
import { useTheme } from '@mui/material/styles'
import ExportMenu from '../components/ExportMenu'
import FindingsTable from '../components/FindingsTable'
import Illustration from '../components/Illustration'
import LiveActivity from '../components/LiveActivity'
import ScanTimeline, { etaSeconds } from '../components/ScanTimeline'
import { PageHeader } from '../components/Layout'
import StatCard from '../components/StatCard'
import { ScanProgress } from '../components/Instrument'
import StateBadge from '../components/StateBadge'
import { Empty, LoadError, Loading, TableSkeleton } from '../components/States'
import { duration, num, timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { useScanEvents } from '../hooks/useScanEvents'
import { stepCounts } from '../live'
import { href, navigate } from '../hooks/useRoute'
import { useTriage } from '../hooks/useTriage'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import { useToast } from '../store/toast'
import { TERMINAL, type Finding } from '../types'

/** Seconds as "4m 20s" — only ever called with an estimate that passed etaSeconds. */
function eta(sec: number): string {
  if (sec < 60) return `${sec}s`
  if (sec < 3600) return `${Math.floor(sec / 60)}m ${sec % 60}s`
  return `${Math.floor(sec / 3600)}h ${Math.floor((sec % 3600) / 60)}m`
}

export default function ScanDetailPage({ id }: { id: string }) {
  const { api } = useSettings()
  const { t } = useT()
  const ch = useTheme().chart
  const notify  = useToast()
  // The list endpoint carries the target and timings; polled until the job ends.
  const scans    = useResource(() => api.listScans(500), [api], 3000)
  const scan     = scans.data?.scans.find(s => s.id === id)
  const active   = scan ? !TERMINAL.has(scan.state) : true
  const findings = useResource(() => api.findings({ scan_id: id, limit: 500 }), [api, id], active ? 4000 : 0)
  const triage   = useTriage(findings.reload)
  // Opened only while the job runs; a finished job's narration is over and the
  // counters say everything. A finished repository refreshes the counters and
  // findings at once rather than at the next poll.
  const { live, status: liveStatus } = useScanEvents(api.scanEventsUrl(id), !!scan && active, () => {
    scans.reload(); findings.reload()
  })

  if (scans.loading) return <Loading />
  if (scans.error && !scans.data) return <LoadError error={scans.error} retry={scans.reload} />
  if (!scan) return <Empty art="no-data" title={t('scan.notFound')} body={t('scan.notFoundBody')} action={<Button href={href('scans')}>{t('scan.back')}</Button>} />

  const act = (name: 'pause' | 'resume' | 'cancel') =>
    api[name](id).then(() => { notify(t(`toast.scan_${name}`)); scans.reload() })
      .catch(e => notify(e instanceof Error ? e.message : 'Failed', 'error'))

  const rescan = () =>
    api.rescan(id).then(j => { notify(t('scan.rescanned')); navigate(href('scans', j.id)) })
      .catch(e => notify(e instanceof Error ? e.message : 'Failed', 'error'))

  const remove = () => {
    if (!window.confirm(t('scan.deleteConfirm'))) return
    api.deleteScan(id).then(() => { notify(t('scan.deleted')); navigate(href('scans')) })
      .catch(e => notify(e instanceof Error ? e.message : 'Failed', 'error'))
  }

  const rows: Finding[] = findings.data?.findings ?? []
  const remaining = etaSeconds(scan)
  const critical = rows.filter(f => f.severity === 'critical' || f.severity === 'high')
  const reachable = rows.filter(f => f.reachable_from_head)
  const repos = new Set(rows.map(f => f.repository.id))
  const finished = scan.state === 'completed'

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
          {TERMINAL.has(scan.state) && <Button startIcon={<ReplayIcon />} onClick={rescan}>{t('scan.rescan')}</Button>}
          <ExportMenu findings={rows} scope={{ filters: { scan_id: id }, total: findings.data?.total }} />
          {TERMINAL.has(scan.state) && <Button color="error" startIcon={<DeleteIcon />} onClick={remove}>{t('scan.delete')}</Button>}
        </>} />

      {scan.error && <Alert severity="error" sx={{ mb: 2 }}>{scan.error}</Alert>}

      {/* The completion summary: what the scan was for, answered once, with the
          links a responder needs next. Shown only on a clean finish — a failed
          job already has its error above, and celebrating it would be absurd. */}
      {finished && (
        <Card sx={{ p: 2.5, mb: 2, display: 'flex', gap: 3, alignItems: 'center', flexWrap: 'wrap' }}>
          <Box sx={{ width: 150, flexShrink: 0, display: { xs: 'none', sm: 'block' } }}>
            <Illustration name={rows.length ? 'detected' : 'celebration'} size={150} />
          </Box>
          <Box sx={{ flex: '1 1 300px' }}>
            <Box sx={{ fontSize: 17, fontWeight: 800 }}>
              {rows.length
                ? t('scan.summaryFound', { n: rows.length, repos: repos.size })
                : t('scan.summaryClean', { repos: scan.repos_scanned })}
            </Box>
            <Box sx={{ color: 'text.secondary', fontSize: 13.5, mt: .5, lineHeight: 1.6 }}>
              {rows.length
                ? t('scan.summaryBody', { critical: critical.length, reachable: reachable.length })
                : t('scan.summaryCleanBody', { blobs: num(scan.blobs_scanned) })}
            </Box>
            {!!rows.length && (
              <Box sx={{ display: 'flex', gap: 1, mt: 1.5, flexWrap: 'wrap' }}>
                <Button size="small" variant="contained"
                  href={href('findings', undefined, { scan_id: id, severity: 'critical', status: 'open' })}>
                  {t('scan.summaryCritical', { n: critical.length })}
                </Button>
                <Button size="small" variant="outlined"
                  href={href('findings', undefined, { scan_id: id, exposure: 'head', status: 'open' })}>
                  {t('scan.summaryReachable', { n: reachable.length })}
                </Button>
                <Button size="small" href={href('repositories')}>{t('scan.summaryRepos')}</Button>
              </Box>
            )}
          </Box>
        </Card>
      )}

      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid item xs={12} md={7}>
          <Card sx={{ p: 2, height: '100%' }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', fontSize: 13, mb: 1, flexWrap: 'wrap', gap: 1 }}>
              <span>{t('scan.reposProgress', { a: scan.repos_scanned, b: scan.repos_total || '?' })}</span>
              <Box component="span" sx={{ display: 'flex', gap: 1.5, alignItems: 'center' }}>
                {remaining !== undefined && (
                  <Box component="span" sx={{ color: 'text.secondary' }}>{t('scan.eta', { time: eta(remaining) })}</Box>
                )}
                <span>{Math.round(scan.progress * 100)}%</span>
              </Box>
            </Box>
            <ScanProgress variant={scan.state === 'queued' ? 'indeterminate' : 'determinate'} value={scan.progress * 100}
              active={scan.state === 'running'} height={8} color={scan.state === 'failed' ? 'error' : TERMINAL.has(scan.state) ? 'success' : 'primary'} />
            {remaining === undefined && !TERMINAL.has(scan.state) && (
              <Box sx={{ fontSize: 11.5, color: 'text.disabled', mt: 1 }}>{t('scan.noEta')}</Box>
            )}

            <Grid container spacing={1.5} sx={{ mt: 1 }}>
              <Grid item xs={6} md={3}><StatCard label={t('scan.reposCount')} value={scan.repos_total} /></Grid>
              <Grid item xs={6} md={3}><StatCard label={t('scan.blobsScanned')} value={scan.blobs_scanned} /></Grid>
              <Grid item xs={6} md={3}><StatCard label={t('scan.newFindings')} value={scan.findings_count} hint={t('scan.newFindingsHint')} accent={ch.severity.medium} /></Grid>
              <Grid item xs={6} md={3}><StatCard label={t('scan.openInJob')} value={rows.filter(f => f.status === 'open').length} accent={ch.severity.critical} /></Grid>
            </Grid>
          </Card>
        </Grid>
        <Grid item xs={12} md={5}>
          <Card sx={{ p: 2, height: '100%' }}>
            <Typography sx={{ fontWeight: 700, mb: 1.5 }}>{t('scan.stages')}</Typography>
            <ScanTimeline scan={scan} steps={liveStatus === 'live' || liveStatus === 'reconnecting' ? stepCounts(live) : undefined} />
          </Card>
        </Grid>
      </Grid>

      {(active || live.log.length > 0) && <LiveActivity live={live} status={liveStatus} />}

      <Card>
        <Box sx={{ px: 2, py: 1.5, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Typography sx={{ fontWeight: 700 }}>{t('scan.findingsTitle')}</Typography>
          <Link href={href('findings', undefined, { scan_id: id })} underline="hover" sx={{ fontSize: 13 }}>{t('scan.openInFindings')}</Link>
        </Box>
        {findings.loading && !findings.data ? <TableSkeleton rows={5} columns={6} /> : (
          <FindingsTable findings={rows} onStatus={triage}
            emptyMessage={active ? t('scan.emptyActive') : t('scan.emptyDone')} />
        )}
      </Card>
    </>
  )
}
