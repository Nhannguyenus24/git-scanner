import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Grid from '@mui/material/Grid'
import LinearProgress from '@mui/material/LinearProgress'
import Link from '@mui/material/Link'
import Typography from '@mui/material/Typography'
import { useTheme } from '@mui/material/styles'
import BarList from '../components/BarList'
import { PageHeader } from '../components/Layout'
import StatCard from '../components/StatCard'
import StateBadge from '../components/StateBadge'
import { Empty, LoadError, Loading } from '../components/States'
import { num, timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import { TERMINAL, type FindingStatus, type Severity } from '../types'

export default function OverviewPage({ onNewScan }: { onNewScan: () => void }) {
  const { api } = useSettings()
  const { t } = useT()
  const ch = useTheme().chart
  const ov    = useResource(() => api.overview(), [api], 5000)
  const scans = useResource(() => api.listScans(6), [api], 5000)

  if (ov.loading) return <Loading />
  if (ov.error && !ov.data) return <LoadError error={ov.error} retry={ov.reload} />
  const o = ov.data!

  if (o.repositories === 0 && !scans.data?.scans.length) {
    return (
      <Empty title={t('overview.emptyTitle')} body={t('overview.emptyBody')}
        action={<Button variant="contained" onClick={onNewScan} sx={{ mt: 1 }}>{t('common.startScan')}</Button>} />
    )
  }

  const critical = o.by_severity.find(c => c.key === 'critical')?.count ?? 0
  const findings = (p: Record<string, string>) => navigate(href('findings', undefined, { status: 'open', ...p }))

  return (
    <>
      <PageHeader title={t('overview.title')} subtitle={t('overview.subtitle')} />

      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid item xs={6} md={2}><StatCard label={t('overview.openFindings')} value={num(o.open_findings)} hint={t('overview.totalHint', { n: num(o.findings) })} accent={ch.categorical[0]} onClick={() => findings({})} /></Grid>
        <Grid item xs={6} md={2}><StatCard label={t('overview.critical')} value={num(critical)} accent={ch.severity.critical} onClick={() => findings({ severity: 'critical' })} /></Grid>
        <Grid item xs={6} md={2}><StatCard label={t('overview.liveAtHead')} value={num(o.reachable_from_head)} hint={t('overview.liveHint')} accent={ch.severity.critical} /></Grid>
        <Grid item xs={6} md={2}><StatCard label={t('overview.exposedRepos')} value={`${o.exposed_repos}/${o.repositories}`} accent={ch.categorical[2]} onClick={() => navigate(href('repositories'))} /></Grid>
        <Grid item xs={6} md={2}><StatCard label={t('overview.activeScans')} value={o.active_scans} accent={ch.categorical[0]} onClick={() => navigate(href('scans'))} /></Grid>
        <Grid item xs={6} md={2}><StatCard label={t('overview.unmapped')} value={o.candidates} hint={t('overview.unmappedHint')} accent={ch.other} onClick={() => navigate(href('candidates'))} /></Grid>
      </Grid>

      <Grid container spacing={2}>
        <Grid item xs={12} md={4}>
          <Card sx={{ p: 2, height: '100%' }}>
            <Typography sx={{ fontWeight: 700, mb: 1.5 }}>{t('overview.bySeverity')}</Typography>
            <BarList items={o.by_severity} label={k => t(`sev.${k}`)} color={k => ch.severity[k as Severity] ?? ch.other}
              onClick={k => findings({ severity: k })} />
          </Card>
        </Grid>
        <Grid item xs={12} md={4}>
          <Card sx={{ p: 2, height: '100%' }}>
            <Typography sx={{ fontWeight: 700, mb: 1.5 }}>{t('overview.byProvider')}</Typography>
            <BarList items={o.by_provider.slice(0, 10)} label={k => (k === 'unmapped' ? t('findings.unmapped') : k)}
              color={k => k === 'unmapped' ? ch.other : ch.categorical[0]} onClick={k => findings({ provider: k })} />
          </Card>
        </Grid>
        <Grid item xs={12} md={4}>
          <Card sx={{ p: 2, height: '100%' }}>
            <Typography sx={{ fontWeight: 700, mb: 1.5 }}>{t('overview.triage')}</Typography>
            <BarList items={o.by_status} label={k => t(`status.${k as FindingStatus}`)}
              color={k => k === 'open' ? ch.severity.critical : k === 'rotated' ? ch.categorical[0] : ch.other}
              onClick={k => navigate(href('findings', undefined, { status: k }))} />
          </Card>
        </Grid>

        <Grid item xs={12}>
          <Card sx={{ p: 2 }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
              <Typography sx={{ fontWeight: 700 }}>{t('overview.recentScans')}</Typography>
              <Link href={href('scans')} underline="hover" sx={{ fontSize: 13 }}>{t('overview.allScans')}</Link>
            </Box>
            {(scans.data?.scans ?? []).map(s => (
              <Box key={s.id} component="a" href={href('scans', s.id)} sx={{
                display: 'grid', gridTemplateColumns: { xs: '1fr auto', md: '110px 1fr 200px 90px 90px' }, gap: 2, alignItems: 'center',
                py: 1.25, px: 1, borderRadius: 1, color: 'inherit', textDecoration: 'none', '&:hover': { bgcolor: 'action.hover' },
              }}>
                <StateBadge state={s.state} />
                <Box sx={{ minWidth: 0 }}>
                  <Box sx={{ fontWeight: 600, fontSize: 14, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{s.target.value}</Box>
                  <Box sx={{ fontSize: 12, color: 'text.secondary' }}>{s.target.kind} · {timeAgo(s.created_at)}</Box>
                </Box>
                <Box sx={{ display: { xs: 'none', md: 'block' } }}>
                  <LinearProgress variant="determinate" value={s.progress * 100}
                    color={s.state === 'failed' ? 'error' : TERMINAL.has(s.state) ? 'success' : 'primary'} sx={{ height: 6, borderRadius: 3 }} />
                  <Box sx={{ fontSize: 11, color: 'text.secondary', mt: .5 }}>{t('overview.reposOf', { a: s.repos_scanned, b: s.repos_total })}</Box>
                </Box>
                <Box sx={{ display: { xs: 'none', md: 'block' }, fontSize: 13, textAlign: 'right' }}>{t('overview.blobs', { n: num(s.blobs_scanned) })}</Box>
                <Box sx={{ display: { xs: 'none', md: 'block' }, fontSize: 13, textAlign: 'right', fontWeight: 700 }}>{t('overview.found', { n: s.findings_count })}</Box>
              </Box>
            ))}
          </Card>
        </Grid>
      </Grid>
    </>
  )
}
