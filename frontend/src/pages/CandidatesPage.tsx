import Box from '@mui/material/Box'
import Card from '@mui/material/Card'
import Chip from '@mui/material/Chip'
import Table from '@mui/material/Table'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableContainer from '@mui/material/TableContainer'
import TableHead from '@mui/material/TableHead'
import TableRow from '@mui/material/TableRow'
import { PageHeader } from '../components/Layout'
import { Empty, LoadError, Loading } from '../components/States'
import { timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { useSettings } from '../store/settings'

export default function CandidatesPage() {
  const { api } = useSettings()
  const res = useResource(() => api.candidates(), [api], 15000)

  if (res.loading) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />
  const cs = res.data!.candidates

  return (
    <>
      <PageHeader title="Unmapped credential shapes"
        subtitle="High-entropy secrets no named rule claims, clustered by shape. A frequent shape is a provider worth writing a detection rule for." />
      <Card>
        {!cs.length ? <Empty title="Backlog is empty" body="Every credential found so far was attributed to a known provider." /> : (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Guessed name</TableCell>
                  <TableCell>Sample</TableCell>
                  <TableCell>Shape</TableCell>
                  <TableCell>Context</TableCell>
                  <TableCell align="right">Seen</TableCell>
                  <TableCell>Status</TableCell>
                  <TableCell>Last seen</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {cs.map(c => (
                  <TableRow key={c.id} hover>
                    <TableCell sx={{ fontWeight: 600 }}>{c.guessed_name || <Box component="span" sx={{ color: 'text.secondary', fontWeight: 400 }}>unknown</Box>}</TableCell>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: 12, color: 'primary.main' }}>{c.sample_masked}</TableCell>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: 12 }}>
                      {c.charset_class} · {c.length_min === c.length_max ? c.length_min : `${c.length_min}–${c.length_max}`} chars
                    </TableCell>
                    <TableCell sx={{ maxWidth: 320 }}>
                      <Box sx={{ display: 'flex', gap: .5, flexWrap: 'wrap' }}>
                        {c.env_var_names.slice(0, 3).map(v => <Chip key={v} size="small" label={v} variant="outlined" />)}
                        {c.nearby_hosts.slice(0, 2).map(h => <Chip key={h} size="small" label={h} color="secondary" variant="outlined" />)}
                      </Box>
                    </TableCell>
                    <TableCell align="right" sx={{ fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>{c.occurrences}</TableCell>
                    <TableCell><Chip size="small" label={c.status} variant="outlined" /></TableCell>
                    <TableCell sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}>{timeAgo(c.last_seen_at)}</TableCell>
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
