import { useEffect, useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Chip from '@mui/material/Chip'
import InputAdornment from '@mui/material/InputAdornment'
import MenuItem from '@mui/material/MenuItem'
import TablePagination from '@mui/material/TablePagination'
import TextField from '@mui/material/TextField'
import SearchIcon from '@mui/icons-material/Search'
import FindingsTable from '../components/FindingsTable'
import { PageHeader } from '../components/Layout'
import { LoadError, Loading } from '../components/States'
import { num, STATUS_LABEL } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate, type Route } from '../hooks/useRoute'
import { useTriage } from '../hooks/useTriage'
import { useSettings } from '../store/settings'
import { SEVERITIES, STATUSES, type FindingFilter, type FindingStatus, type Severity } from '../types'

// The filter lives in the URL so a filtered view can be linked, bookmarked and
// reached from the overview's charts.
const KEYS = ['severity', 'provider', 'status', 'q', 'repository_id', 'scan_id', 'page', 'size'] as const

export default function FindingsPage({ route }: { route: Route }) {
  const { api } = useSettings()
  const p = route.params
  const page = Number(p.get('page') ?? 0), size = Number(p.get('size') ?? 50)
  const filter: FindingFilter = {
    severity: (p.get('severity') ?? '') as Severity | '', provider: p.get('provider') ?? '',
    status: (p.get('status') ?? '') as FindingStatus | '', q: p.get('q') ?? '',
    repository_id: p.get('repository_id') ?? '', scan_id: p.get('scan_id') ?? '',
    limit: size, offset: page * size,
  }

  const set = (changes: Partial<Record<typeof KEYS[number], string>>) => {
    const next: Record<string, string> = {}
    for (const k of KEYS) next[k] = p.get(k) ?? ''
    // Any change other than paging returns to the first page.
    navigate(href('findings', undefined, { ...next, page: '', ...changes }))
  }

  const key = KEYS.map(k => p.get(k) ?? '').join('|')
  const res = useResource(() => api.findings(filter), [api, key])
  const ov  = useResource(() => api.overview(), [api])
  const triage = useTriage(res.reload)

  // Debounce the free-text box so typing does not fire a request per key.
  const [q, setQ] = useState(filter.q ?? '')
  useEffect(() => setQ(filter.q ?? ''), [filter.q])
  useEffect(() => {
    if (q === (filter.q ?? '')) return
    const t = setTimeout(() => set({ q }), 350)
    return () => clearTimeout(t)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [q])

  const providers = ov.data?.by_provider.map(c => c.key) ?? []
  const scoped = filter.repository_id || filter.scan_id

  return (
    <>
      <PageHeader title="Findings"
        subtitle={res.data ? `${num(res.data.total)} matching` : 'Every credential the pipeline has recorded'}
        actions={(filter.severity || filter.provider || filter.status || filter.q || scoped)
          ? <Button color="inherit" onClick={() => navigate(href('findings'))}>Clear filters</Button> : undefined} />

      <Card sx={{ p: 1.5, mb: 2, display: 'flex', gap: 1.5, flexWrap: 'wrap', alignItems: 'center' }}>
        <TextField placeholder="Search path, rule, provider or repository" value={q} onChange={e => setQ(e.target.value)}
          sx={{ flex: '1 1 280px' }}
          InputProps={{ startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment> }} />
        <TextField select label="Severity" value={filter.severity} onChange={e => set({ severity: e.target.value })} sx={{ minWidth: 130 }}>
          <MenuItem value="">Any</MenuItem>
          {SEVERITIES.map(s => <MenuItem key={s} value={s}>{s}</MenuItem>)}
        </TextField>
        <TextField select label="Provider" value={filter.provider} onChange={e => set({ provider: e.target.value })} sx={{ minWidth: 150 }}>
          <MenuItem value="">Any</MenuItem>
          {[...new Set([...providers, ...(filter.provider ? [filter.provider] : [])])].map(s => <MenuItem key={s} value={s}>{s}</MenuItem>)}
        </TextField>
        <TextField select label="Status" value={filter.status} onChange={e => set({ status: e.target.value })} sx={{ minWidth: 150 }}>
          <MenuItem value="">Any</MenuItem>
          {STATUSES.map(s => <MenuItem key={s} value={s}>{STATUS_LABEL[s]}</MenuItem>)}
        </TextField>
        {filter.repository_id && <Chip label="One repository" onDelete={() => set({ repository_id: '' })} />}
        {filter.scan_id && <Chip label={`Scan ${filter.scan_id.slice(0, 8)}`} onDelete={() => set({ scan_id: '' })} />}
      </Card>

      {res.error && !res.data ? <LoadError error={res.error} retry={res.reload} /> : (
        <Card sx={{ position: 'relative' }}>
          {res.loading && !res.data ? <Loading /> : (
            <>
              <FindingsTable findings={res.data?.findings ?? []} onStatus={triage} />
              <Box sx={{ borderTop: 1, borderColor: 'divider' }}>
                <TablePagination component="div" count={res.data?.total ?? 0} page={page} rowsPerPage={size}
                  rowsPerPageOptions={[25, 50, 100, 200]}
                  onPageChange={(_, n) => set({ page: String(n) })}
                  onRowsPerPageChange={e => set({ size: e.target.value })} />
              </Box>
            </>
          )}
        </Card>
      )}
    </>
  )
}
