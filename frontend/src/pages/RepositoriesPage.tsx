import { useMemo, useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Chip from '@mui/material/Chip'
import InputAdornment from '@mui/material/InputAdornment'
import Table from '@mui/material/Table'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableContainer from '@mui/material/TableContainer'
import TableHead from '@mui/material/TableHead'
import TableRow from '@mui/material/TableRow'
import TextField from '@mui/material/TextField'
import FormControlLabel from '@mui/material/FormControlLabel'
import Switch from '@mui/material/Switch'
import SearchIcon from '@mui/icons-material/Search'
import { PageHeader } from '../components/Layout'
import { Empty, LoadError, Loading } from '../components/States'
import { SEV_COLOR, timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import type { Repository } from '../types'

function Count({ n, color }: { n: number; color: string }) {
  return (
    <Box component="span" sx={{ fontWeight: 700, fontVariantNumeric: 'tabular-nums', color: n ? color : 'text.disabled' }}>
      {n || '·'}
    </Box>
  )
}

export default function RepositoriesPage() {
  const { api } = useSettings()
  const res = useResource(() => api.repositories(), [api], 10000)
  const [q, setQ] = useState('')
  const [exposedOnly, setExposedOnly] = useState(true)

  const repos = useMemo(() => (res.data?.repositories ?? []).filter((r: Repository) =>
    (!exposedOnly || r.open > 0) && `${r.owner}/${r.name}`.toLowerCase().includes(q.toLowerCase())), [res.data, q, exposedOnly])

  if (res.loading) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />
  const all = res.data!.repositories

  return (
    <>
      <PageHeader title="Repositories" subtitle={`${all.filter(r => r.open).length} of ${all.length} have open findings`} />
      <Card sx={{ p: 1.5, mb: 2, display: 'flex', gap: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <TextField placeholder="Filter by owner/name" value={q} onChange={e => setQ(e.target.value)} sx={{ flex: '1 1 260px' }}
          InputProps={{ startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment> }} />
        <FormControlLabel control={<Switch checked={exposedOnly} onChange={e => setExposedOnly(e.target.checked)} />} label="Only with open findings" />
      </Card>
      <Card>
        {!repos.length ? <Empty title="No repositories match" action={exposedOnly ? <Button onClick={() => setExposedOnly(false)}>Show all</Button> : undefined} /> : (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Repository</TableCell>
                  <TableCell>Type</TableCell>
                  <TableCell align="center">Critical</TableCell>
                  <TableCell align="center">High</TableCell>
                  <TableCell align="center">Medium</TableCell>
                  <TableCell align="center">Low</TableCell>
                  <TableCell align="right">Open / total</TableCell>
                  <TableCell>Last scanned</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {repos.map(r => (
                  <TableRow key={r.id} hover sx={{ cursor: 'pointer' }}
                    onClick={() => navigate(href('findings', undefined, { repository_id: r.id }))}>
                    <TableCell>
                      <Box sx={{ fontWeight: 600 }}><Box component="span" sx={{ color: 'text.secondary', fontWeight: 400 }}>{r.owner}/</Box>{r.name}</Box>
                      <Box sx={{ display: 'flex', gap: .5, mt: .25 }}>
                        {r.is_fork && <Chip size="small" label="fork" variant="outlined" />}
                        {r.stars > 0 && <Chip size="small" label={`★ ${r.stars}`} variant="outlined" />}
                      </Box>
                    </TableCell>
                    <TableCell sx={{ color: 'text.secondary', fontSize: 12 }}>{r.app_type || '—'}</TableCell>
                    <TableCell align="center"><Count n={r.critical} color={SEV_COLOR.critical} /></TableCell>
                    <TableCell align="center"><Count n={r.high} color={SEV_COLOR.high} /></TableCell>
                    <TableCell align="center"><Count n={r.medium} color={SEV_COLOR.medium} /></TableCell>
                    <TableCell align="center"><Count n={r.low} color={SEV_COLOR.low} /></TableCell>
                    <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums' }}>{r.open} / {r.findings}</TableCell>
                    <TableCell sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}>{timeAgo(r.last_scanned_at)}</TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Card>
    </>
  )
}
