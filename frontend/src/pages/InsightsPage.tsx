import React from 'react'
import Box from '@mui/material/Box'
import Grid from '@mui/material/Grid'
import Table from '@mui/material/Table'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableHead from '@mui/material/TableHead'
import TableRow from '@mui/material/TableRow'
import { useTheme } from '@mui/material/styles'
import BarList from '../components/BarList'
import Columns from '../components/charts/Columns'
import Figure from '../components/charts/Figure'
import Ring, { countsToSlices } from '../components/charts/Ring'
import { PageHeader } from '../components/Layout'
import StatCard from '../components/StatCard'
import { LoadError, Loading } from '../components/States'
import { num, paletteFor } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate } from '../hooks/useRoute'
import { providerMeta } from '../providers'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import type { Insights } from '../types'

// Short, language-neutral axis labels for the exposure buckets.
const EXPOSURE_LABEL: Record<string, string> = {
  '1d': '<1d', '7d': '<7d', '30d': '<30d', '90d': '<90d', '1y': '<1y', over_1y: '≥1y', unknown: '?',
}

export default function InsightsPage() {
  const { api } = useSettings()
  const { t } = useT()
  const ch = useTheme().chart
  const res = useResource(() => api.insights(), [api], 30000)

  if (res.loading) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />
  const d = res.data!

  const days = (n: number | null) => (n === null ? t('ins.noData') : t('ins.days', { n: n.toFixed(n < 10 ? 1 : 0) }))
  const kindMap = paletteFor(d.by_file_kind.map(c => c.key), ch.categorical, ch.other)
  const ownerMap = paletteFor(d.by_owner_type.map(c => c.key), ch.categorical, ch.other)
  const modeColor = (k: string) => (k === 'live' ? ch.severity.critical : ch.categorical[0])
  const activityColor: Record<string, string> = {
    active: ch.categorical[0], stale: ch.severity.high, archived: ch.severity.critical, unknown: ch.other,
  }
  const disc = d.disclosure

  return (
    <>
      <PageHeader title={t('ins.title')} subtitle={t('ins.subtitle')} />

      <Grid container spacing={2} sx={{ mb: 1 }}>
        <Grid item xs={6} md={3}>
          <StatCard label={t('ins.reused')} value={num(d.reuse.secrets)} hint={t('ins.reusedHint', { n: num(d.reuse.repos) })} accent={ch.categorical[2]} />
        </Grid>
        <Grid item xs={6} md={3}>
          <StatCard label={t('ins.removedOpen')} value={num(d.removed_but_open)} hint={t('ins.removedOpenHint')} accent={ch.severity.high}
            onClick={() => navigate(href('findings', undefined, { status: 'open' }))} />
        </Grid>
        <Grid item xs={6} md={3}>
          <StatCard label={t('ins.medianRemoval')} value={days(d.median_days_to_removal)} hint={t('ins.medianRemovalHint')} accent={ch.categorical[1]} />
        </Grid>
        <Grid item xs={6} md={3}>
          <StatCard label={t('ins.disclosureRate')}
            value={disc.live_at_disclosure ? `${Math.round(100 * disc.went_dead / disc.live_at_disclosure)}%` : t('ins.noData')}
            hint={t('ins.disclosureHint', { a: disc.went_dead, b: disc.live_at_disclosure })} accent={ch.categorical[0]} />
        </Grid>
      </Grid>

      <Section title={t('ins.secWhere')} note={t('ins.coverage', { a: num(d.attributes.known), b: num(d.attributes.repositories) })} />
      <Grid container spacing={2}>
        <Grid item xs={12} md={5}>
          <Figure title={t('ins.fileKind')} caption={t('ins.fileKindCap')}>
            <BarList items={d.by_file_kind} color={k => kindMap[k] ?? ch.other} empty={t('ins.empty')} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={4}>
          <Figure title={t('ins.language')}>
            <BarList items={d.by_language} color={() => ch.categorical[1]} empty={t('ins.empty')} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={3}>
          <Figure title={t('ins.ownerType')}>
            <Ring slices={countsToSlices(d.by_owner_type, k => ownerMap[k] ?? ch.other, k => k)} centerLabel={t('analytics.open')} size={140} />
          </Figure>
        </Grid>
        <Grid item xs={12}>
          <Figure title={t('ins.activity')} caption={t('ins.activityCap')}>
            <BarList items={d.repo_activity.map(b => ({ key: b.key, count: b.findings }))} color={k => activityColor[k] ?? ch.other}
              label={k => {
                const b = d.repo_activity.find(x => x.key === k)!
                return `${k} · ${t('ins.activityRow', { repos: num(b.repos), head: num(b.at_head) })}`
              }} empty={t('ins.empty')} />
          </Figure>
        </Grid>
      </Grid>

      <Section title={t('ins.secExposure')} />
      <Grid container spacing={2}>
        <Grid item xs={12} md={6}>
          <Figure title={t('ins.exposure')} caption={t('ins.exposureCap')} minHeight={220}>
            <Columns data={d.exposure.map(b => ({
              label: EXPOSURE_LABEL[b.key] ?? b.key, value: b.at_head + b.removed,
              sub: `${t('ins.atHead')} ${num(b.at_head)} · ${t('ins.removed')} ${num(b.removed)}`,
            }))} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={6}>
          <Figure title={t('ins.disclosure')} caption={t('ins.disclosureCap')}>
            <KeyValues rows={[
              [t('ins.d.repos'), num(disc.repos)],
              [t('ins.d.findings'), num(disc.findings)],
              [t('ins.d.live'), num(disc.live_at_disclosure)],
              [t('ins.d.dead'), num(disc.went_dead)],
              [t('ins.d.stillLive'), num(disc.still_live)],
              [t('ins.d.rotated'), num(disc.rotated)],
              [t('ins.d.median'), days(disc.median_days_to_dead)],
            ]} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={5}>
          <Figure title={t('ins.keyTypes')} caption={t('ins.keyTypesCap')}>
            <BarList items={d.key_types.map(k => ({ key: `${k.provider}\u0000${k.key_type}`, count: k.count }))}
              color={() => ch.categorical[3]}
              label={k => { const [p, kt] = k.split('\u0000'); return `${providerMeta(p).label} · ${kt}` }}
              empty={t('ins.empty')} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={3}>
          <Figure title={t('ins.modes')}>
            <Ring slices={countsToSlices(d.modes, modeColor, k => k)} centerLabel={t('analytics.open')} size={140} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={4}>
          <Figure title={t('ins.commitContext')} caption={t('ins.commitContextCap')}>
            <BarList items={d.commit_context} color={k => (k === 'removal_cleanup' ? ch.severity.high : ch.categorical[1])}
              label={k => t(`ins.cc.${k}`)} empty={t('ins.empty')} />
          </Figure>
        </Grid>
      </Grid>

      <Section title={t('ins.secQuality')} />
      <Grid container spacing={2}>
        <Grid item xs={12} md={7}>
          <Figure title={t('ins.precision')} caption={t('ins.precisionCap')}>
            <PrecisionTable rows={d.rule_precision} />
          </Figure>
        </Grid>
        <Grid item xs={12} md={5}>
          <Figure title={t('ins.reuseTitle')} caption={t('ins.reuseCap')}>
            <ReuseTable rows={d.reuse.top} />
          </Figure>
        </Grid>
        <Grid item xs={12}>
          <Figure title={t('ins.detection')} caption={t('ins.detectionCap', { n: num(d.detection.repositories) })}>
            <DetectionFunnel counters={d.detection.counters} />
          </Figure>
        </Grid>
      </Grid>
    </>
  )
}

function Section({ title, note }: { title: string; note?: string }) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1.5, flexWrap: 'wrap', mt: 3.5, mb: 1.5 }}>
      <Box component="h2" sx={{ m: 0, fontSize: 15, fontWeight: 800, letterSpacing: '.01em' }}>{title}</Box>
      {note && <Box sx={{ color: 'text.secondary', fontSize: 12 }}>{note}</Box>}
    </Box>
  )
}

function KeyValues({ rows }: { rows: [string, React.ReactNode][] }) {
  return (
    <Box component="dl" sx={{ m: 0, display: 'grid', gridTemplateColumns: '1fr auto', rowGap: .75, columnGap: 2, fontSize: 13 }}>
      {rows.map(([k, v]) => (
        <React.Fragment key={k}>
          <Box component="dt" sx={{ color: 'text.secondary' }}>{k}</Box>
          <Box component="dd" sx={{ m: 0, fontWeight: 700, textAlign: 'right', fontVariantNumeric: 'tabular-nums' }}>{v}</Box>
        </React.Fragment>
      ))}
    </Box>
  )
}

const mono = { fontFamily: '"IBM Plex Mono", monospace', fontSize: 12 }
const numCell = { fontVariantNumeric: 'tabular-nums' as const }

function PrecisionTable({ rows }: { rows: Insights['rule_precision'] }) {
  const { t } = useT()
  if (!rows.length) return <Box sx={{ color: 'text.secondary', fontSize: 13 }}>{t('ins.empty')}</Box>
  return (
    <Table size="small">
      <TableHead>
        <TableRow>
          <TableCell>{t('ins.col.rule')}</TableCell>
          <TableCell align="right">{t('ins.col.total')}</TableCell>
          <TableCell align="right">{t('ins.col.triaged')}</TableCell>
          <TableCell align="right">{t('ins.col.fp')}</TableCell>
          <TableCell align="right">{t('ins.col.precision')}</TableCell>
        </TableRow>
      </TableHead>
      <TableBody>
        {rows.map(r => {
          const real = r.rotated + r.ignored
          const judged = real + r.false_positive
          return (
            <TableRow key={r.rule} hover sx={{ cursor: 'pointer' }}
              onClick={() => navigate(href('findings', undefined, { q: r.rule }))}>
              <TableCell sx={mono}>{r.rule}</TableCell>
              <TableCell align="right" sx={numCell}>{num(r.total)}</TableCell>
              <TableCell align="right" sx={numCell}>{num(judged)}</TableCell>
              <TableCell align="right" sx={numCell}>{num(r.false_positive)}</TableCell>
              <TableCell align="right" sx={{ ...numCell, fontWeight: 700 }}>
                {/* Unjudged rules show a dash: zero triage is "unknown", not 0% or 100%. */}
                {judged ? `${Math.round(100 * real / judged)}%` : '—'}
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}

function ReuseTable({ rows }: { rows: Insights['reuse']['top'] }) {
  const { t } = useT()
  if (!rows.length) return <Box sx={{ color: 'text.secondary', fontSize: 13 }}>{t('ins.empty')}</Box>
  return (
    <Table size="small">
      <TableHead>
        <TableRow>
          <TableCell>{t('ins.col.preview')}</TableCell>
          <TableCell>{t('ins.col.provider')}</TableCell>
          <TableCell align="right">{t('ins.col.repos')}</TableCell>
          <TableCell align="right">{t('ins.col.forks')}</TableCell>
        </TableRow>
      </TableHead>
      <TableBody>
        {rows.map((r, i) => (
          <TableRow key={`${r.preview}-${i}`}>
            <TableCell sx={{ ...mono, color: 'primary.main' }}>{r.preview}</TableCell>
            <TableCell>{r.provider ? providerMeta(r.provider).label : t('findings.unmapped')}</TableCell>
            <TableCell align="right" sx={numCell}>{num(r.repos)}</TableCell>
            <TableCell align="right" sx={numCell}>{num(r.forks)}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

// The funnel shows each drop as a share of the candidates that pass entered,
// so a gate's weight reads directly: which filters do the work, and which never
// fire.
function DetectionFunnel({ counters }: { counters: Record<string, number> }) {
  const { t } = useT()
  const keys = Object.keys(counters)
  if (!keys.length) return <Box sx={{ color: 'text.secondary', fontSize: 13 }}>{t('ins.empty')}</Box>

  const group = (prefix: string, base: string) => ({
    base: counters[base] ?? 0,
    rows: keys.filter(k => k.startsWith(prefix) && k !== base).sort((a, b) => counters[b] - counters[a]),
  })
  const groups: [string, { base: number; rows: string[] }][] = [
    [t('ins.f.blobs'), group('blobs_', 'blobs_scanned')],
    [t('ins.f.named'), group('named_', 'named_candidates')],
    [t('ins.f.generic'), group('generic_', 'generic_candidates')],
  ]
  return (
    <Grid container spacing={3}>
      {groups.map(([title, g]) => (
        <Grid item xs={12} md={4} key={title}>
          <Box sx={{ fontSize: 12, fontWeight: 700, color: 'text.secondary', mb: 1, textTransform: 'uppercase', letterSpacing: '.05em' }}>
            {title} · {num(g.base)}
          </Box>
          <Table size="small">
            <TableBody>
              {g.rows.map(k => (
                <TableRow key={k}>
                  <TableCell sx={{ ...mono, pl: 0 }}>{k}</TableCell>
                  <TableCell align="right" sx={numCell}>{num(counters[k])}</TableCell>
                  <TableCell align="right" sx={{ ...numCell, color: 'text.secondary', pr: 0, width: 56 }}>
                    {g.base ? `${(100 * counters[k] / g.base).toFixed(1)}%` : ''}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Grid>
      ))}
    </Grid>
  )
}
