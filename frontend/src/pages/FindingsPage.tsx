import { useEffect, useMemo, useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Chip from '@mui/material/Chip'
import InputAdornment from '@mui/material/InputAdornment'
import MenuItem from '@mui/material/MenuItem'
import TablePagination from '@mui/material/TablePagination'
import TextField from '@mui/material/TextField'
import SearchIcon from '@mui/icons-material/Search'
import HistoryIcon from '@mui/icons-material/History'
import ExportMenu from '../components/ExportMenu'
import FindingsTable from '../components/FindingsTable'
import SavedFilterMenu from '../components/SavedFilterMenu'
import { PageHeader } from '../components/Layout'
import { LoadError, TableSkeleton } from '../components/States'
import { useBulkTriage, useTriage } from '../hooks/useTriage'
import { num } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate, type Route } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import { SEVERITIES, STATUSES, type Finding, type FindingFilter, type FindingStatus, type Severity } from '../types'
import BulkBar from '../components/BulkBar'

// The filter lives in the URL so a filtered view can be linked, bookmarked and
// reached from the overview's charts.
//
// Two of the fields are refinements the API does not implement: `exposure`
// (reachable from HEAD / confirmed live) and `sort`. They are applied in the
// browser, which is only honest within a bounded window — so when one of them
// is on, the page asks for the server's maximum of 500 rows, refines those, and
// says so above the table when there were more. Silently refining one page of
// 50 and printing a total of 4000 would be a lie told by arithmetic.
const KEYS = ['severity', 'provider', 'status', 'q', 'repository_id', 'scan_id', 'exposure', 'sort', 'page', 'size'] as const

const WINDOW = 500

type Exposure = '' | 'head' | 'history' | 'live'
type Sort = '' | 'recent' | 'oldest' | 'severity'

export default function FindingsPage({ route }: { route: Route }) {
  const { api } = useSettings()
  const { t } = useT()
  const p = route.params
  const page = Number(p.get('page') ?? 0), size = Number(p.get('size') ?? 50)
  const exposure = (p.get('exposure') ?? '') as Exposure
  const sort = (p.get('sort') ?? '') as Sort
  const refining = !!exposure || !!sort

  const serverFilter: FindingFilter = {
    severity: (p.get('severity') ?? '') as Severity | '', provider: p.get('provider') ?? '',
    status: (p.get('status') ?? '') as FindingStatus | '', q: p.get('q') ?? '',
    repository_id: p.get('repository_id') ?? '', scan_id: p.get('scan_id') ?? '',
    limit: refining ? WINDOW : size,
    offset: refining ? 0 : page * size,
  }

  const set = (changes: Partial<Record<typeof KEYS[number], string>>) => {
    const next: Record<string, string> = {}
    for (const k of KEYS) next[k] = p.get(k) ?? ''
    // Any change other than paging returns to the first page.
    navigate(href('findings', undefined, { ...next, page: '', ...changes }))
  }

  const key = KEYS.map(k => p.get(k) ?? '').join('|')
  const res = useResource(() => api.findings(serverFilter), [api, key])
  const ov  = useResource(() => api.overview(), [api])
  const triage = useTriage(res.reload)
  const bulk = useBulkTriage(res.reload)

  // Debounce the free-text box so typing does not fire a request per key.
  const [q, setQ] = useState(serverFilter.q ?? '')
  useEffect(() => setQ(serverFilter.q ?? ''), [serverFilter.q])
  useEffect(() => {
    if (q === (serverFilter.q ?? '')) return
    const timer = setTimeout(() => set({ q }), 350)
    return () => clearTimeout(timer)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q])

  const [selected, setSelected] = useState<Set<string>>(new Set())
  // A filter change invalidates the selection: acting on rows that are no longer
  // on screen is the classic way a bulk action surprises someone.
  useEffect(() => setSelected(new Set()), [key])

  const rows = res.data?.findings ?? []

  const refined = useMemo(() => {
    let out = rows
    if (exposure === 'head')    out = out.filter(f => f.reachable_from_head)
    if (exposure === 'history') out = out.filter(f => !f.reachable_from_head)
    if (exposure === 'live')    out = out.filter(f => f.metadata?.probe_status === 'live')
    if (sort === 'recent') out = [...out].sort((a, b) => +new Date(b.detected_at) - +new Date(a.detected_at))
    if (sort === 'oldest') out = [...out].sort((a, b) =>
      +new Date(a.committed_at ?? a.detected_at) - +new Date(b.committed_at ?? b.detected_at))
    if (sort === 'severity') out = [...out].sort((a, b) => SEVERITIES.indexOf(a.severity) - SEVERITIES.indexOf(b.severity))
    return out
  }, [rows, exposure, sort])

  const shown = refining ? refined.slice(page * size, page * size + size) : refined
  const total = refining ? refined.length : (res.data?.total ?? 0)
  const truncated = refining && (res.data?.total ?? 0) > WINDOW

  const selectedFindings = useMemo<Finding[]>(
    () => refined.filter(f => selected.has(f.id)), [refined, selected])

  const toggle = (ids: string[], checked: boolean) => setSelected(prev => {
    const next = new Set(prev)
    for (const id of ids) checked ? next.add(id) : next.delete(id)
    return next
  })

  const providers = ov.data?.by_provider.map(c => c.key) ?? []
  const active: Record<string, string> = {}
  for (const k of KEYS) { const v = p.get(k); if (v && k !== 'page' && k !== 'size') active[k] = v }
  const anyFilter = Object.keys(active).length > 0

  return (
    <>
      <PageHeader title={t('findings.title')}
        subtitle={res.data ? t('findings.matching', { n: num(total) }) : t('findings.subtitle')}
        actions={<>
          <SavedFilterMenu current={active} apply={params => navigate(href('findings', undefined, params))} />
          <ExportMenu findings={refining ? refined : rows} scope={{ filters: active, total }} />
          {anyFilter && <Button color="inherit" onClick={() => navigate(href('findings'))}>{t('common.clearFilters')}</Button>}
        </>} />

      <Card sx={{ p: 1.5, mb: 2, display: 'flex', gap: 1.5, flexWrap: 'wrap', alignItems: 'center' }}>
        <TextField placeholder={t('findings.search')} value={q} onChange={e => setQ(e.target.value)}
          sx={{ flex: '1 1 240px' }} inputProps={{ 'aria-label': t('findings.search') }}
          InputProps={{ startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment> }} />
        <TextField select label={t('findings.severity')} value={serverFilter.severity} onChange={e => set({ severity: e.target.value })} sx={{ minWidth: 130 }}>
          <MenuItem value="">{t('common.any')}</MenuItem>
          {SEVERITIES.map(s => <MenuItem key={s} value={s}>{t(`sev.${s}`)}</MenuItem>)}
        </TextField>
        <TextField select label={t('findings.provider')} value={serverFilter.provider} onChange={e => set({ provider: e.target.value })} sx={{ minWidth: 150 }}>
          <MenuItem value="">{t('common.any')}</MenuItem>
          {[...new Set([...providers, ...(serverFilter.provider ? [serverFilter.provider] : [])])].map(s => <MenuItem key={s} value={s}>{s}</MenuItem>)}
        </TextField>
        <TextField select label={t('findings.status')} value={serverFilter.status} onChange={e => set({ status: e.target.value })} sx={{ minWidth: 140 }}>
          <MenuItem value="">{t('common.any')}</MenuItem>
          {STATUSES.map(s => <MenuItem key={s} value={s}>{t(`status.${s}`)}</MenuItem>)}
        </TextField>
        <TextField select label={t('findings.exposure')} value={exposure} onChange={e => set({ exposure: e.target.value })} sx={{ minWidth: 150 }}>
          <MenuItem value="">{t('common.any')}</MenuItem>
          <MenuItem value="head">{t('findings.atHead')}</MenuItem>
          <MenuItem value="history">{t('findings.history')}</MenuItem>
          <MenuItem value="live">{t('probe.live')}</MenuItem>
        </TextField>
        <TextField select label={t('findings.sort')} value={sort} onChange={e => set({ sort: e.target.value })} sx={{ minWidth: 150 }}>
          <MenuItem value="">{t('findings.sortDefault')}</MenuItem>
          <MenuItem value="recent">{t('findings.sortRecent')}</MenuItem>
          <MenuItem value="oldest">{t('findings.sortOldest')}</MenuItem>
          <MenuItem value="severity">{t('findings.sortSeverity')}</MenuItem>
        </TextField>
        {serverFilter.repository_id && <Chip label={t('findings.oneRepo')} onDelete={() => set({ repository_id: '' })} />}
        {serverFilter.repository_id && (
          <Chip clickable component="a" href={href('history', serverFilter.repository_id)} variant="outlined"
            icon={<HistoryIcon fontSize="small" />} label={t('history.action')} />
        )}
        {serverFilter.scan_id && <Chip label={`Scan ${serverFilter.scan_id.slice(0, 8)}`} onDelete={() => set({ scan_id: '' })} />}
      </Card>

      {truncated && (
        <Alert severity="info" sx={{ mb: 2 }}>{t('findings.windowNote', { n: WINDOW, total: num(res.data?.total ?? 0) })}</Alert>
      )}

      {res.error && !res.data ? <LoadError error={res.error} retry={res.reload} /> : (
        <Card sx={{ position: 'relative' }}>
          {res.loading && !res.data ? <TableSkeleton rows={8} columns={6} /> : (
            <>
              <FindingsTable findings={shown} onStatus={triage}
                selected={selected} onSelect={toggle}
                openId={p.get('finding')} onOpen={id => {
                  const next: Record<string, string> = {}
                  for (const k of KEYS) next[k] = p.get(k) ?? ''
                  navigate(href('findings', undefined, { ...next, finding: id ?? '' }))
                }} />
              <Box sx={{ borderTop: 1, borderColor: 'divider' }}>
                <TablePagination component="div" count={total} page={page} rowsPerPage={size}
                  rowsPerPageOptions={[25, 50, 100, 200]}
                  onPageChange={(_, n) => set({ page: String(n) })}
                  onRowsPerPageChange={e => set({ size: e.target.value })} />
              </Box>
            </>
          )}
        </Card>
      )}

      <BulkBar
        findings={selectedFindings}
        running={bulk.running}
        onClear={() => setSelected(new Set())}
        onStatus={status => bulk.run(selectedFindings, status)}
        scope={{ filters: active, total }} />
    </>
  )
}
