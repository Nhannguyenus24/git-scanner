import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Grid from '@mui/material/Grid'
import LinearProgress from '@mui/material/LinearProgress'
import Link from '@mui/material/Link'
import Typography from '@mui/material/Typography'
import BarList from '../components/BarList'
import { PageHeader } from '../components/Layout'
import StatCard from '../components/StatCard'
import StateBadge from '../components/StateBadge'
import { Empty, LoadError, Loading } from '../components/States'
import { num, SEV_COLOR, STATUS_LABEL, timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import { TERMINAL, type FindingStatus, type Severity } from '../types'

export default function OverviewPage({ onNewScan }: { onNewScan: () => void }) {
  const { api } = useSettings()
  const ov    = useResource(() => api.overview(), [api], 5000)
  const scans = useResource(() => api.listScans(6), [api], 5000)

  if (ov.loading) return <Loading />
  if (ov.error && !ov.data) return <LoadError error={ov.error} retry={ov.reload} />
  const o = ov.data!

  if (o.repositories === 0 && !scans.data?.scans.length) {
    return (
      <Empty title="No scans yet"
        body="Scan a repository, a user or an organisation. Findings, repositories and the unmapped-provider backlog appear here as the pipeline works through them."
        action={<Button variant="contained" onClick={onNewScan} sx={{ mt: 1 }}>Start a scan</Button>} />
    )
  }

  const critical = o.by_severity.find(c => c.key === 'critical')?.count ?? 0
  const findings = (p: Record<string, string>) => navigate(href('findings', undefined, { status: 'open', ...p }))

  return (
    <>
      <PageHeader title="Overview" subtitle="Open exposure across every scanned repository" />

      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid item xs={6} md={2}><StatCard label="Open findings" value={num(o.open_findings)} hint={`${num(o.findings)} total`} accent="#f5a623" onClick={() => findings({})} /></Grid>
        <Grid item xs={6} md={2}><StatCard label="Critical" value={num(critical)} accent={SEV_COLOR.critical} onClick={() => findings({ severity: 'critical' })} /></Grid>
        <Grid item xs={6} md={2}><StatCard label="Live at HEAD" value={num(o.reachable_from_head)} hint="still in the current tree" accent="#f07070" /></Grid>
        <Grid item xs={6} md={2}><StatCard label="Exposed repos" value={`${o.exposed_repos}/${o.repositories}`} accent="#56b6f7" onClick={() => navigate(href('repositories'))} /></Grid>
        <Grid item xs={6} md={2}><StatCard label="Active scans" value={o.active_scans} accent="#4ec9a3" onClick={() => navigate(href('scans'))} /></Grid>
        <Grid item xs={6} md={2}><StatCard label="Unmapped shapes" value={o.candidates} hint="awaiting a provider" accent="#6b789a" onClick={() => navigate(href('candidates'))} /></Grid>
      </Grid>

      <Grid container spacing={2}>
        <Grid item xs={12} md={4}>
          <Card sx={{ p: 2, height: '100%' }}>
            <Typography sx={{ fontWeight: 700, mb: 1.5 }}>Open by severity</Typography>
            <BarList items={o.by_severity} color={k => SEV_COLOR[k as Severity] ?? '#6b789a'}
              onClick={k => findings({ severity: k })} empty="No open findings" />
          </Card>
        </Grid>
        <Grid item xs={12} md={4}>
          <Card sx={{ p: 2, height: '100%' }}>
            <Typography sx={{ fontWeight: 700, mb: 1.5 }}>Open by provider</Typography>
            <BarList items={o.by_provider.slice(0, 10)} color={k => k === 'unmapped' ? '#6b789a' : '#f5a623'}
              onClick={k => findings({ provider: k })} empty="No open findings" />
          </Card>
        </Grid>
        <Grid item xs={12} md={4}>
          <Card sx={{ p: 2, height: '100%' }}>
            <Typography sx={{ fontWeight: 700, mb: 1.5 }}>Triage</Typography>
            <BarList items={o.by_status} label={k => STATUS_LABEL[k as FindingStatus] ?? k}
              color={k => k === 'open' ? '#f07070' : k === 'rotated' ? '#4ec9a3' : '#6b789a'}
              onClick={k => navigate(href('findings', undefined, { status: k }))} />
          </Card>
        </Grid>

        <Grid item xs={12}>
          <Card sx={{ p: 2 }}>
            <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 1 }}>
              <Typography sx={{ fontWeight: 700 }}>Recent scans</Typography>
              <Link href={href('scans')} underline="hover" sx={{ fontSize: 13 }}>All scans →</Link>
            </Box>
            {(scans.data?.scans ?? []).map(s => (
              <Box key={s.id} component="a" href={href('scans', s.id)} sx={{
                display: 'grid', gridTemplateColumns: { xs: '1fr auto', md: '90px 1fr 200px 90px 90px' }, gap: 2, alignItems: 'center',
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
                  <Box sx={{ fontSize: 11, color: 'text.secondary', mt: .5 }}>{s.repos_scanned}/{s.repos_total} repos</Box>
                </Box>
                <Box sx={{ display: { xs: 'none', md: 'block' }, fontSize: 13, textAlign: 'right' }}>{num(s.blobs_scanned)} blobs</Box>
                <Box sx={{ display: { xs: 'none', md: 'block' }, fontSize: 13, textAlign: 'right', fontWeight: 700 }}>{s.findings_count} found</Box>
              </Box>
            ))}
          </Card>
        </Grid>
      </Grid>
    </>
  )
}
