import { useMemo, useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Grid from '@mui/material/Grid'
import MenuItem from '@mui/material/MenuItem'
import TextField from '@mui/material/TextField'
import DownloadIcon from '@mui/icons-material/FileDownloadOutlined'
import { useTheme } from '@mui/material/styles'
import BarList from '../components/BarList'
import Figure from '../components/charts/Figure'
import LeakCalendar from '../components/LeakCalendar'
import Columns from '../components/charts/Columns'
import Ring, { countsToSlices } from '../components/charts/Ring'
import { PageHeader } from '../components/Layout'
import StatCard from '../components/StatCard'
import { EmptyInline, LoadError, Loading } from '../components/States'
import { download, exportName, toCsv } from '../export'
import { monthLabel, num, paletteFor, timeAgo } from '../format'
import { repoRisk } from '../posture'
import { useResource } from '../hooks/useResource'
import { href, navigate } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import type { Count, FindingStatus, Severity } from '../types'

const SEV_ORDER: Severity[] = ['critical', 'high', 'medium', 'low', 'info']

/** A download button for the numbers behind one figure. Every chart on this page
 *  gets one: a reader who wants to check a bar should not have to retype it. */
function FigureExport({ name, headers, rows }: { name: string; headers: string[]; rows: unknown[][] }) {
  const { t } = useT()
  return (
    <Button size="small" startIcon={<DownloadIcon sx={{ fontSize: 15 }} />} disabled={!rows.length}
      onClick={() => download(exportName(name, 'csv'), toCsv(headers, rows), 'text/csv')}
      sx={{ fontSize: 11.5, minWidth: 0, px: .75 }}>
      {t('export.csvShort')}
    </Button>
  )
}

export default function AnalyticsPage() {
  const { api } = useSettings()
  const { t } = useT()
  const ch = useTheme().chart
  const ov = useResource(() => api.overview(), [api], 10000)
  const an = useResource(() => api.analytics(), [api], 10000)
  const repos = useResource(() => api.repositories(), [api], 30000)
  // The oldest unresolved findings need the findings themselves; one window of
  // the server's maximum is enough to rank the worst, and the caption says so.
  const old = useResource(() => api.findings({ status: 'open', limit: 500 }), [api], 60000)

  // Severity and provider are applied to the links this page emits rather than
  // to the aggregates themselves: /v1/analytics takes no filter, and recomputing
  // a filtered timeline from a page of findings would be a different number
  // wearing the same label. A date range needs the same endpoint change.
  const [severity, setSeverity] = useState<Severity | ''>('')
  const [provider, setProvider] = useState('')

  const oldest = useMemo(() => {
    const rows = old.data?.findings ?? []
    return [...rows]
      .filter(f => (!severity || f.severity === severity) && (!provider || (f.provider || 'unmapped') === provider))
      .sort((a, b) => +new Date(a.committed_at ?? a.detected_at) - +new Date(b.committed_at ?? b.detected_at))
      .slice(0, 8)
  }, [old.data, severity, provider])

  const topRepos = useMemo(() => {
    const rows = repos.data?.repositories ?? []
    return [...rows].filter(r => r.open > 0)
      .sort((a, b) => repoRisk(b).score - repoRisk(a).score)
      .slice(0, 8)
  }, [repos.data])

  if (ov.loading || an.loading) return <Loading />
  if (ov.error && !ov.data) return <LoadError error={ov.error} retry={ov.reload} />
  if (an.error && !an.data) return <LoadError error={an.error} retry={an.reload} />
  const o = ov.data!, a = an.data!

  const sevColor = (k: string) => ch.severity[k as Severity] ?? ch.other
  const sevLabel = (k: string) => t(`sev.${k}`)
  // A stable colour per provider by rank; "unmapped" is always the neutral.
  const named = o.by_provider.map(c => c.key).filter(k => k !== 'unmapped')
  const provMap = paletteFor(named, ch.categorical, ch.other)
  const provColor = (k: string) => (k === 'unmapped' ? ch.other : provMap[k] ?? ch.other)
  const appMap = paletteFor(a.by_app_type.map(c => c.key), ch.categorical, ch.other)
  const sortSev = (cs: Count[]) => [...cs].sort((x, y) => SEV_ORDER.indexOf(x.key as Severity) - SEV_ORDER.indexOf(y.key as Severity))
  const statusColor = (k: string) => (k === 'open' ? ch.severity.critical : k === 'rotated' ? ch.categorical[0] : ch.other)
  const noticeColor: Record<string, string> = { sent: ch.categorical[0], pending: ch.categorical[2], failed: ch.severity.critical, suppressed: ch.other }
  const probeColor: Record<string, string> = { live: ch.severity.critical, dead: ch.categorical[0], unverified: ch.other, error: ch.severity.medium }

  // Every click-through carries the page's current filters, so a chart and the
  // findings list it opens always agree.
  const scope = { status: 'open', ...(severity ? { severity } : {}), ...(provider ? { provider } : {}) }
  const toFindings = (p: Record<string, string>) => navigate(href('findings', undefined, { ...scope, ...p }))
  const filtered = !!severity || !!provider

  return (
    <>
      <PageHeader title={t('analytics.title')} subtitle={t('analytics.subtitle')}
        actions={filtered ? <Button color="inherit" onClick={() => { setSeverity(''); setProvider('') }}>{t('common.clearFilters')}</Button> : undefined} />

      <Card sx={{ p: 1.5, mb: 2, display: 'flex', gap: 1.5, flexWrap: 'wrap', alignItems: 'center' }}>
        <TextField select label={t('findings.severity')} value={severity} onChange={e => setSeverity(e.target.value as Severity | '')} sx={{ minWidth: 150 }}>
          <MenuItem value="">{t('common.any')}</MenuItem>
          {SEV_ORDER.map(s => <MenuItem key={s} value={s}>{t(`sev.${s}`)}</MenuItem>)}
        </TextField>
        <TextField select label={t('findings.provider')} value={provider} onChange={e => setProvider(e.target.value)} sx={{ minWidth: 170 }}>
          <MenuItem value="">{t('common.any')}</MenuItem>
          {o.by_provider.map(c => <MenuItem key={c.key} value={c.key}>{c.key === 'unmapped' ? t('findings.unmapped') : c.key}</MenuItem>)}
        </TextField>
        <Box sx={{ fontSize: 12, color: 'text.secondary', flex: '1 1 240px' }}>{t('analytics.filterNote')}</Box>
        <Button size="small" variant="outlined" onClick={() => toFindings({})}>{t('analytics.openFindings')}</Button>
      </Card>

      <Alert severity="info" sx={{ mb: 2 }}>{t('analytics.rangeNote')}</Alert>

      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid item xs={6} md={3}><StatCard label={t('analytics.findings')} value={o.findings} hint={t('analytics.openOf', { n: num(o.open_findings) })} accent={ch.categorical[0]} onClick={() => toFindings({})} /></Grid>
        <Grid item xs={6} md={3}><StatCard label={t('overview.liveAtHead')} value={o.reachable_from_head} hint={t('overview.liveHint')} accent={ch.severity.critical} onClick={() => toFindings({ exposure: 'head' })} /></Grid>
        <Grid item xs={6} md={3}><StatCard label={t('analytics.repositories')} value={`${o.exposed_repos}/${o.repositories}`} hint={t('analytics.exposedOf')} accent={ch.categorical[2]} onClick={() => navigate(href('repositories'))} /></Grid>
        <Grid item xs={6} md={3}><StatCard label={t('overview.unmapped')} value={o.candidates} hint={t('overview.unmappedHint')} accent={ch.categorical[3]} onClick={() => navigate(href('candidates'))} /></Grid>
      </Grid>

      <Box sx={{ mb: 2 }}><LeakCalendar /></Box>

      <Grid container spacing={2}>
        <Grid item xs={12} md={8}>
          <Figure title={t('analytics.timeline')} caption={t('analytics.timelineCap')} minHeight={220}
            actions={<FigureExport name="timeline" headers={['period', 'findings']} rows={a.timeline.map(p => [p.period, p.findings])} />}>
            <Columns data={a.timeline.map(p => ({ label: monthLabel(p.period), value: p.findings, sub: monthLabel(p.period) }))} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={4}>
          <Figure title={t('analytics.bySeverity')} minHeight={220}
            actions={<FigureExport name="by-severity" headers={['severity', 'findings']} rows={o.by_severity.map(c => [c.key, c.count])} />}>
            <Ring slices={countsToSlices(sortSev(o.by_severity), sevColor, sevLabel)} centerLabel={t('analytics.open')} />
          </Figure>
        </Grid>

        <Grid item xs={12} md={5}>
          <Figure title={t('analytics.byProvider')} caption={t('analytics.byProviderCap')}
            actions={<FigureExport name="by-provider" headers={['provider', 'findings']} rows={o.by_provider.map(c => [c.key, c.count])} />}>
            <BarList items={o.by_provider.slice(0, 10)} color={provColor} label={k => (k === 'unmapped' ? t('findings.unmapped') : k)} onClick={k => toFindings({ provider: k })} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={3}>
          <Figure title={t('analytics.byAppType')}
            actions={<FigureExport name="by-app-type" headers={['app_type', 'findings']} rows={a.by_app_type.map(c => [c.key, c.count])} />}>
            <Ring slices={countsToSlices(a.by_app_type, k => appMap[k] ?? ch.other, k => k)} centerLabel={t('analytics.open')} size={140} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={4}>
          <Figure title={t('analytics.triage')}
            actions={<FigureExport name="triage" headers={['status', 'findings']} rows={o.by_status.map(c => [c.key, c.count])} />}>
            <BarList items={o.by_status} label={k => t(`status.${k as FindingStatus}`)} color={statusColor}
              onClick={k => navigate(href('findings', undefined, { ...scope, status: k }))} />
          </Figure>
        </Grid>

        {/* Top repositories by exposure — ranked by the same risk score the
            repositories table sorts on, so the two pages cannot disagree. */}
        <Grid item xs={12} md={6}>
          <Figure title={t('analytics.topRepos')} caption={t('analytics.topReposCap')}
            actions={<FigureExport name="top-repositories" headers={['repository', 'risk', 'open', 'critical', 'high']}
              rows={topRepos.map(r => [`${r.owner}/${r.name}`, repoRisk(r).score, r.open, r.critical, r.high])} />}>
            {topRepos.length ? (
              <BarList
                items={topRepos.map(r => ({ key: `${r.owner}/${r.name}`, count: repoRisk(r).score }))}
                color={() => ch.severity.high}
                onClick={k => {
                  const r = topRepos.find(x => `${x.owner}/${x.name}` === k)
                  if (r) navigate(href('findings', undefined, { repository_id: r.id, status: 'open' }))
                }} />
            ) : <EmptyInline title={t('analytics.topReposEmpty')} art="repositories" size={100} />}
          </Figure>
        </Grid>

        {/* Oldest unresolved — the ones that have been public longest, which is
            not the same list as the most severe. */}
        <Grid item xs={12} md={6}>
          <Figure title={t('analytics.oldest')} caption={t('analytics.oldestCap')}
            actions={<FigureExport name="oldest-open" headers={['repository', 'rule', 'severity', 'committed_at', 'detected_at']}
              rows={oldest.map(f => [`${f.repository.owner}/${f.repository.name}`, f.rule_id, f.severity, f.committed_at ?? '', f.detected_at])} />}>
            {oldest.length ? (
              <Box>
                {oldest.map(f => (
                  <Box key={f.id} component="a"
                    href={href('findings', undefined, { finding: f.id, repository_id: f.repository.id })}
                    sx={{
                      display: 'flex', justifyContent: 'space-between', gap: 2, py: .85, px: .5,
                      textDecoration: 'none', color: 'inherit', borderBottom: 1, borderColor: 'divider',
                      '&:hover': { bgcolor: 'action.hover' }, '&:last-of-type': { borderBottom: 0 },
                    }}>
                    <Box sx={{ minWidth: 0 }}>
                      <Box sx={{ fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {f.repository.owner}/{f.repository.name}
                      </Box>
                      <Box sx={{ fontSize: 11, color: 'text.secondary', fontFamily: 'monospace' }}>
                        {f.rule_id} · {t(`sev.${f.severity}`)}
                      </Box>
                    </Box>
                    <Box sx={{ fontSize: 12, color: 'text.secondary', whiteSpace: 'nowrap' }}>
                      {timeAgo(f.committed_at ?? f.detected_at)}
                    </Box>
                  </Box>
                ))}
              </Box>
            ) : <EmptyInline title={t('analytics.oldestEmpty')} art="safe" size={100} />}
          </Figure>
        </Grid>

        <Grid item xs={12} md={7}>
          <Figure title={t('analytics.rules')} caption={t('analytics.rulesCap')}
            actions={<FigureExport name="by-rule" headers={['rule', 'findings']} rows={a.by_rule.map(c => [c.key, c.count])} />}>
            <BarList items={a.by_rule} color={() => ch.categorical[0]} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={5}>
          <Figure title={t('analytics.liveTitle')} caption={a.live_probes.length ? t('analytics.liveCap') : t('analytics.liveCapOff')}
            actions={<FigureExport name="live-probes" headers={['outcome', 'findings']} rows={a.live_probes.map(c => [c.key, c.count])} />}>
            {a.live_probes.length
              ? <Ring slices={countsToSlices(a.live_probes, k => probeColor[k] ?? ch.other, k => t(`probe.${k}`))} centerLabel={t('analytics.checked')} size={140} />
              : <EmptyInline title={t('analytics.liveEmpty')} art="no-data" size={100} />}
          </Figure>
        </Grid>

        <Grid item xs={12} md={5}>
          <Figure title={t('analytics.notices')} caption={t('analytics.noticesCap', { a: num(o.notified_repos), b: num(o.awaiting_notice) })}
            actions={<FigureExport name="notifications" headers={['state', 'count']} rows={a.notifications.map(c => [c.key, c.count])} />}>
            {a.notifications.length
              ? <BarList items={a.notifications} label={k => t(`notice.${k}`)} color={k => noticeColor[k] ?? ch.other}
                  onClick={() => navigate(href('repositories'))} />
              : <EmptyInline title={t('analytics.noticesEmpty')} art="mailbox" size={100} />}
          </Figure>
        </Grid>
      </Grid>
    </>
  )
}
