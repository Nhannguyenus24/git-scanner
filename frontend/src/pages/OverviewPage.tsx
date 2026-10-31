import { useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Grid from '@mui/material/Grid'
import Link from '@mui/material/Link'
import Menu from '@mui/material/Menu'
import MenuItem from '@mui/material/MenuItem'
import ListItemText from '@mui/material/ListItemText'
import Tooltip from '@mui/material/Tooltip'
import TuneIcon from '@mui/icons-material/TuneOutlined'
import { useTheme } from '@mui/material/styles'
import ActivityFeed from '../components/ActivityFeed'
import BarList from '../components/BarList'
import DashboardWidget from '../components/DashboardWidget'
import PostureScore from '../components/PostureScore'
import { PageHeader } from '../components/Layout'
import StatCard from '../components/StatCard'
import { LiveDot, ScanProgress } from '../components/Instrument'
import StateBadge from '../components/StateBadge'
import { Empty, LoadError, Loading } from '../components/States'
import { num, timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate } from '../hooks/useRoute'
import { ALL_WIDGETS, LAYOUT_PRESETS, usePrefs, type WidgetId } from '../store/preferences'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import { TERMINAL, type FindingStatus, type Severity } from '../types'

export default function OverviewPage({ onNewScan }: { onNewScan: () => void }) {
  const { api } = useSettings()
  const { t } = useT()
  const ch = useTheme().chart
  const { prefs, set } = usePrefs()
  const [editing, setEditing] = useState(false)
  const [layoutAnchor, setLayoutAnchor] = useState<null | HTMLElement>(null)

  const ov    = useResource(() => api.overview(), [api], 5000)
  const scans = useResource(() => api.listScans({ limit: 8 }), [api], 5000)
  // The activity feed is assembled from these three lists; see ActivityFeed for
  // why nothing is generated in the browser.
  const recent  = useResource(() => api.findings({ limit: 8 }), [api], 15000)
  const notices = useResource(() => api.notifications({ limit: 8 }), [api], 30000)

  if (ov.loading) return <Loading />
  if (ov.error && !ov.data) return <LoadError error={ov.error} retry={ov.reload} />
  const o = ov.data!

  if (o.repositories === 0 && !scans.data?.scans.length) {
    return (
      <Empty art="welcome" size={280} title={t('overview.emptyTitle')} body={t('overview.emptyBody')}
        action={<Button variant="contained" onClick={onNewScan}>{t('common.startScan')}</Button>} />
    )
  }

  const critical = o.by_severity.find(c => c.key === 'critical')?.count ?? 0
  const findings = (p: Record<string, string>) => navigate(href('findings', undefined, { status: 'open', ...p }))

  const visible = prefs.widgets
  const move = (id: WidgetId, by: number) => {
    const i = visible.indexOf(id)
    const j = i + by
    if (i < 0 || j < 0 || j >= visible.length) return
    const next = [...visible]
    ;[next[i], next[j]] = [next[j], next[i]]
    set('widgets', next)
  }
  const hide = (id: WidgetId) => set('widgets', visible.filter(w => w !== id))
  const show = (id: WidgetId) => set('widgets', [...visible, id])
  const hidden = ALL_WIDGETS.filter(w => !visible.includes(w))

  const frame = (id: WidgetId, node: React.ReactNode, title: string, bare = false) => (
    <DashboardWidget key={id} title={title} editing={editing} bare={bare}
      onUp={visible.indexOf(id) > 0 ? () => move(id, -1) : undefined}
      onDown={visible.indexOf(id) < visible.length - 1 ? () => move(id, 1) : undefined}
      onHide={() => hide(id)}>
      {node}
    </DashboardWidget>
  )

  const widget = (id: WidgetId): { span: number; node: React.ReactNode } | null => {
    switch (id) {
      case 'posture':
        return { span: 12, node: frame(id, <PostureScore overview={o} />, t('posture.title'), true) }
      case 'stats':
        return { span: 12, node: frame(id, (
          <Grid container spacing={2}>
            <Grid item xs={6} md={2}><StatCard label={t('overview.openFindings')} value={o.open_findings} hint={t('overview.totalHint', { n: num(o.findings) })} accent={ch.categorical[0]} onClick={() => findings({})} /></Grid>
            <Grid item xs={6} md={2}><StatCard label={t('overview.critical')} value={critical} accent={ch.severity.critical} onClick={() => findings({ severity: 'critical' })} /></Grid>
            <Grid item xs={6} md={2}><StatCard label={t('overview.liveAtHead')} value={o.reachable_from_head} hint={t('overview.liveHint')} accent={ch.severity.critical} onClick={() => findings({ exposure: 'head' })} /></Grid>
            <Grid item xs={6} md={2}><StatCard label={t('overview.exposedRepos')} value={`${o.exposed_repos}/${o.repositories}`} accent={ch.categorical[2]} onClick={() => navigate(href('repositories'))} /></Grid>
            <Grid item xs={6} md={2}><StatCard label={t('overview.activeScans')} value={o.active_scans} accent={ch.categorical[0]} onClick={() => navigate(href('scans'))} /></Grid>
            <Grid item xs={6} md={2}><StatCard label={t('overview.unmapped')} value={o.candidates} hint={t('overview.unmappedHint')} accent={ch.other} onClick={() => navigate(href('candidates'))} /></Grid>
          </Grid>
        ), t('overview.stats'), true) }
      case 'severity':
        return { span: 4, node: frame(id, (
          <BarList items={o.by_severity} label={k => t(`sev.${k}`)} color={k => ch.severity[k as Severity] ?? ch.other}
            onClick={k => findings({ severity: k })} empty={t('activity.empty')} />
        ), t('overview.bySeverity')) }
      case 'provider':
        return { span: 4, node: frame(id, (
          <BarList items={o.by_provider.slice(0, 10)} label={k => (k === 'unmapped' ? t('findings.unmapped') : k)}
            color={k => k === 'unmapped' ? ch.other : ch.categorical[0]} onClick={k => findings({ provider: k })}
            empty={t('activity.empty')} />
        ), t('overview.byProvider')) }
      case 'triage':
        return { span: 4, node: frame(id, (
          <BarList items={o.by_status} label={k => t(`status.${k as FindingStatus}`)}
            color={k => k === 'open' ? ch.severity.critical : k === 'rotated' ? ch.categorical[0] : ch.other}
            onClick={k => navigate(href('findings', undefined, { status: k }))} empty={t('activity.empty')} />
        ), t('overview.triage')) }
      case 'activity':
        return { span: 6, node: frame(id, (
          <ActivityFeed
            scans={scans.data?.scans ?? []}
            findings={recent.data?.findings ?? []}
            notifications={notices.data?.notifications ?? []} />
        ), t('activity.title')) }
      case 'scans':
        return { span: 6, node: frame(id, (
          <Box>
            {(scans.data?.scans ?? []).slice(0, 6).map(s => (
              <Box key={s.id} component="a" href={href('scans', s.id)} sx={{
                display: 'grid', gridTemplateColumns: { xs: '1fr auto', md: '104px 1fr 150px' }, gap: 2, alignItems: 'center',
                py: 1.1, px: 1, borderRadius: 1, color: 'inherit', textDecoration: 'none', '&:hover': { bgcolor: 'action.hover' },
              }}>
                <StateBadge state={s.state} />
                <Box sx={{ minWidth: 0 }}>
                  <Box sx={{ fontWeight: 600, fontSize: 14, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{s.target.value}</Box>
                  <Box sx={{ fontSize: 12, color: 'text.secondary' }}>
                    {s.target.kind} · {timeAgo(s.created_at)} · {t('overview.found', { n: s.findings_count })}
                  </Box>
                </Box>
                <Box sx={{ display: { xs: 'none', md: 'block' } }}>
                  <ScanProgress variant="determinate" value={s.progress * 100} active={s.state === 'running'}
                    color={s.state === 'failed' ? 'error' : TERMINAL.has(s.state) ? 'success' : 'primary'} />
                  <Box sx={{ fontSize: 11, color: 'text.secondary', mt: .5 }}>{t('overview.reposOf', { a: s.repos_scanned, b: s.repos_total })}</Box>
                </Box>
              </Box>
            ))}
            <Link href={href('scans')} underline="hover" sx={{ fontSize: 13, display: 'inline-block', mt: 1, px: 1 }}>{t('overview.allScans')}</Link>
          </Box>
        ), t('overview.recentScans')) }
      default:
        return null
    }
  }

  return (
    <>
      <PageHeader title={t('overview.title')} subtitle={t('overview.subtitle')}
        actions={<>
          {/* The page polls; saying so tells the reader the numbers are current
              without a refresh button that would only duplicate the timer. */}
          <Tooltip title={t('overview.liveHint', { s: 5 })}>
            <Box sx={{
              display: 'flex', alignItems: 'center', gap: .75, px: 1, py: .4, mr: .5,
              border: 1, borderColor: 'divider', borderRadius: 1,
              fontFamily: '"IBM Plex Mono", monospace', fontSize: 11, letterSpacing: '.08em', textTransform: 'uppercase',
              color: ov.error ? 'error.main' : 'text.secondary',
            }}>
              <LiveDot color={ov.error ? 'error.main' : 'success.main'} live={!ov.error} size={7} />
              {t('overview.live', { s: 5 })}
            </Box>
          </Tooltip>
          <Button size="small" startIcon={<TuneIcon />} onClick={e => setLayoutAnchor(e.currentTarget)}>
            {t('dash.layout')}
          </Button>
          <Button size="small" variant={editing ? 'contained' : 'outlined'} onClick={() => setEditing(e => !e)}>
            {t(editing ? 'dash.doneEditing' : 'dash.customise')}
          </Button>
        </>} />

      <Menu anchorEl={layoutAnchor} open={!!layoutAnchor} onClose={() => setLayoutAnchor(null)}>
        <MenuItem disabled sx={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '.07em' }}>{t('dash.layouts')}</MenuItem>
        {Object.keys(LAYOUT_PRESETS).map(k => (
          <MenuItem key={k} selected={visible.join() === LAYOUT_PRESETS[k].join()}
            onClick={() => { set('widgets', [...LAYOUT_PRESETS[k]]); setLayoutAnchor(null) }}>
            <ListItemText primary={t(`dash.layout.${k}`)} secondary={t(`dash.layout.${k}.hint`)} />
          </MenuItem>
        ))}
      </Menu>

      {editing && (
        <Card sx={{ p: 2, mb: 2 }}>
          <Box sx={{ fontSize: 13, color: 'text.secondary', mb: hidden.length ? 1.5 : 0 }}>{t('dash.editHint')}</Box>
          {!!hidden.length && (
            <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
              {hidden.map(w => (
                <Button key={w} size="small" variant="outlined" onClick={() => show(w)}>+ {t(`dash.w.${w}`)}</Button>
              ))}
            </Box>
          )}
        </Card>
      )}

      <Grid container spacing={2}>
        {visible.map(id => {
          const w = widget(id)
          if (!w) return null
          return <Grid item xs={12} md={w.span} key={id}>{w.node}</Grid>
        })}
      </Grid>
    </>
  )
}
