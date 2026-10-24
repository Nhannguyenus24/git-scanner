import Box from '@mui/material/Box'
import Grid from '@mui/material/Grid'
import BarList from '../components/BarList'
import Figure from '../components/charts/Figure'
import Columns from '../components/charts/Columns'
import Ring, { countsToSlices } from '../components/charts/Ring'
import { PageHeader } from '../components/Layout'
import StatCard from '../components/StatCard'
import { LoadError, Loading } from '../components/States'
import { CATEGORICAL, monthLabel, num, OTHER_COLOR, paletteFor, SEV_COLOR, STATUS_LABEL } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import type { Count, FindingStatus, Severity } from '../types'

const SEV_ORDER: Severity[] = ['critical', 'high', 'medium', 'low', 'info']
const sevColor = (k: string) => SEV_COLOR[k as Severity] ?? OTHER_COLOR

// A stable colour per provider by rank, so adding a provider never repaints the
// others; "unmapped" is always the neutral grey.
function providerColor(keys: string[]) {
  const named = keys.filter(k => k !== 'unmapped')
  const map = paletteFor(named)
  return (k: string) => (k === 'unmapped' ? OTHER_COLOR : map[k] ?? OTHER_COLOR)
}

const PROBE_COLOR: Record<string, string> = {
  live: '#9c3b2e', dead: '#3f7d57', unverified: '#8a8372', error: '#a9781a',
}
const PROBE_LABEL: Record<string, string> = {
  live: 'Live — rotate', dead: 'Revoked', unverified: 'No prober', error: 'Check failed',
}

export default function AnalyticsPage() {
  const { api } = useSettings()
  const ov = useResource(() => api.overview(), [api], 10000)
  const an = useResource(() => api.analytics(), [api], 10000)

  if (ov.loading || an.loading) return <Loading />
  if (ov.error && !ov.data) return <LoadError error={ov.error} retry={ov.reload} />
  if (an.error && !an.data) return <LoadError error={an.error} retry={an.reload} />
  const o = ov.data!, a = an.data!

  const providers = o.by_provider.map(c => c.key)
  const provColor = providerColor(providers)
  const appKeys = a.by_app_type.map(c => c.key)
  const appColor = paletteFor(appKeys)
  const sortSev = (cs: Count[]) => [...cs].sort((x, y) => SEV_ORDER.indexOf(x.key as Severity) - SEV_ORDER.indexOf(y.key as Severity))

  const toFindings = (p: Record<string, string>) => navigate(href('findings', undefined, { status: 'open', ...p }))

  return (
    <>
      <PageHeader title="Analytics"
        subtitle="A read on the scan corpus — what was found, where it lives, and when it was introduced" />

      <Grid container spacing={2} sx={{ mb: 2 }}>
        <Grid item xs={6} md={3}><StatCard label="Findings" value={num(o.findings)} hint={`${num(o.open_findings)} open`} accent="#1f6f66" /></Grid>
        <Grid item xs={6} md={3}><StatCard label="Live at HEAD" value={num(o.reachable_from_head)} hint="still in the tree" accent="#9c3b2e" /></Grid>
        <Grid item xs={6} md={3}><StatCard label="Repositories" value={`${o.exposed_repos}/${o.repositories}`} hint="exposed / scanned" accent="#2f4ba8" /></Grid>
        <Grid item xs={6} md={3}><StatCard label="Unmapped shapes" value={o.candidates} hint="awaiting a rule" accent="#a9781a" /></Grid>
      </Grid>

      <Grid container spacing={2}>
        <Grid item xs={12} md={8}>
          <Figure title="Leak introduction timeline" caption="Findings by the month their credential was committed" minHeight={220}>
            <Columns data={a.timeline.map(p => ({ label: monthLabel(p.period), value: p.findings, sub: monthLabel(p.period) }))} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={4}>
          <Figure title="Open by severity" minHeight={220}>
            <Ring slices={countsToSlices(sortSev(o.by_severity), sevColor, k => k)} centerLabel="open" />
          </Figure>
        </Grid>

        <Grid item xs={12} md={5}>
          <Figure title="Open by provider" caption="Named providers plus the unmapped catch-all">
            <BarList items={o.by_provider.slice(0, 10)} color={provColor} onClick={k => toFindings({ provider: k })} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={3}>
          <Figure title="By application type">
            <Ring slices={countsToSlices(a.by_app_type, k => appColor[k] ?? OTHER_COLOR, k => k)} centerLabel="open" size={140} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={4}>
          <Figure title="Triage status">
            <BarList items={o.by_status} label={k => STATUS_LABEL[k as FindingStatus] ?? k}
              color={k => (k === 'open' ? '#9c3b2e' : k === 'rotated' ? '#3f7d57' : OTHER_COLOR)}
              onClick={k => navigate(href('findings', undefined, { status: k }))} />
          </Figure>
        </Grid>

        <Grid item xs={12} md={7}>
          <Figure title="Detection rules" caption="Which rules fire most across the corpus">
            <BarList items={a.by_rule} color={() => CATEGORICAL[0]} onClick={() => undefined} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={5}>
          <Figure title="Live credential validation"
            caption={a.live_probes.length ? 'Results of the liveness probe' : 'Enable PROBE_SECRETS to validate found keys'}>
            {a.live_probes.length
              ? <Ring slices={countsToSlices(a.live_probes, k => PROBE_COLOR[k] ?? OTHER_COLOR, k => PROBE_LABEL[k] ?? k)} centerLabel="checked" size={140} />
              : <Box sx={{ py: 3, color: 'text.secondary', fontSize: 13 }}>
                  No credentials have been validated. With probing on, a confirmed-live key is flagged "rotate now".
                </Box>}
          </Figure>
        </Grid>
      </Grid>
    </>
  )
}
