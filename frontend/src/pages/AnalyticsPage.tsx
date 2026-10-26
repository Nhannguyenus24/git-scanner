import Box from '@mui/material/Box'
import Grid from '@mui/material/Grid'
import { useTheme } from '@mui/material/styles'
import BarList from '../components/BarList'
import Figure from '../components/charts/Figure'
import Columns from '../components/charts/Columns'
import Ring, { countsToSlices } from '../components/charts/Ring'
import { PageHeader } from '../components/Layout'
import StatCard from '../components/StatCard'
import { LoadError, Loading } from '../components/States'
import { monthLabel, num, paletteFor } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import type { Count, FindingStatus, Severity } from '../types'

const SEV_ORDER: Severity[] = ['critical', 'high', 'medium', 'low', 'info']

export default function AnalyticsPage() {
  const { api } = useSettings()
  const { t } = useT()
  const ch = useTheme().chart
  const ov = useResource(() => api.overview(), [api], 10000)
  const an = useResource(() => api.analytics(), [api], 10000)

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

  const toFindings = (p: Record<string, string>) => navigate(href('findings', undefined, { status: 'open', ...p }))

  return (
    <>
      <PageHeader title={t('analytics.title')} subtitle={t('analytics.subtitle')} />

      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid item xs={6} md={3}><StatCard label={t('analytics.findings')} value={num(o.findings)} hint={t('analytics.openOf', { n: num(o.open_findings) })} accent={ch.categorical[0]} /></Grid>
        <Grid item xs={6} md={3}><StatCard label={t('overview.liveAtHead')} value={num(o.reachable_from_head)} hint={t('overview.liveHint')} accent={ch.severity.critical} /></Grid>
        <Grid item xs={6} md={3}><StatCard label={t('analytics.repositories')} value={`${o.exposed_repos}/${o.repositories}`} hint={t('analytics.exposedOf')} accent={ch.categorical[2]} /></Grid>
        <Grid item xs={6} md={3}><StatCard label={t('overview.unmapped')} value={o.candidates} hint={t('overview.unmappedHint')} accent={ch.categorical[3]} /></Grid>
      </Grid>

      <Grid container spacing={2}>
        <Grid item xs={12} md={8}>
          <Figure title={t('analytics.timeline')} caption={t('analytics.timelineCap')} minHeight={220}>
            <Columns data={a.timeline.map(p => ({ label: monthLabel(p.period), value: p.findings, sub: monthLabel(p.period) }))} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={4}>
          <Figure title={t('analytics.bySeverity')} minHeight={220}>
            <Ring slices={countsToSlices(sortSev(o.by_severity), sevColor, sevLabel)} centerLabel={t('analytics.open')} />
          </Figure>
        </Grid>

        <Grid item xs={12} md={5}>
          <Figure title={t('analytics.byProvider')} caption={t('analytics.byProviderCap')}>
            <BarList items={o.by_provider.slice(0, 10)} color={provColor} label={k => (k === 'unmapped' ? t('findings.unmapped') : k)} onClick={k => toFindings({ provider: k })} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={3}>
          <Figure title={t('analytics.byAppType')}>
            <Ring slices={countsToSlices(a.by_app_type, k => appMap[k] ?? ch.other, k => k)} centerLabel={t('analytics.open')} size={140} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={4}>
          <Figure title={t('analytics.triage')}>
            <BarList items={o.by_status} label={k => t(`status.${k as FindingStatus}`)} color={statusColor}
              onClick={k => navigate(href('findings', undefined, { status: k }))} />
          </Figure>
        </Grid>

        <Grid item xs={12} md={7}>
          <Figure title={t('analytics.rules')} caption={t('analytics.rulesCap')}>
            <BarList items={a.by_rule} color={() => ch.categorical[0]} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={5}>
          <Figure title={t('analytics.liveTitle')} caption={a.live_probes.length ? t('analytics.liveCap') : t('analytics.liveCapOff')}>
            {a.live_probes.length
              ? <Ring slices={countsToSlices(a.live_probes, k => probeColor[k] ?? ch.other, k => t(`probe.${k}`))} centerLabel={t('analytics.checked')} size={140} />
              : <Box sx={{ py: 3, color: 'text.secondary', fontSize: 13 }}>{t('analytics.liveEmpty')}</Box>}
          </Figure>
        </Grid>

        <Grid item xs={12} md={5}>
          <Figure title={t('analytics.notices')} caption={t('analytics.noticesCap', { a: num(o.notified_repos), b: num(o.awaiting_notice) })}>
            {a.notifications.length
              ? <BarList items={a.notifications} label={k => t(`notice.${k}`)} color={k => noticeColor[k] ?? ch.other}
                  onClick={() => navigate(href('repositories'))} />
              : <Box sx={{ py: 3, color: 'text.secondary', fontSize: 13 }}>{t('analytics.noticesEmpty')}</Box>}
          </Figure>
        </Grid>
      </Grid>
    </>
  )
}
