import { useState } from 'react'
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
import Pager from '../components/Pager'
import { Empty, LoadError, Loading } from '../components/States'
import { timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'

export default function CandidatesPage() {
  const { api } = useSettings()
  const { t } = useT()
  const [page, setPage] = useState(0)
  const [size, setSize] = useState(50)
  const res = useResource(() => api.candidates({ limit: size, offset: page * size }), [api, page, size], 15000)

  if (res.loading && !res.data) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />
  const cs = res.data!.candidates

  return (
    <>
      <PageHeader title={t('cand.title')} subtitle={t('cand.subtitle')} />
      <Card>
        {!cs.length ? <Empty art="investigate" size={200} title={t('cand.empty')} body={t('cand.emptyBody')} /> : (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>{t('cand.col.name')}</TableCell>
                  <TableCell>{t('cand.col.sample')}</TableCell>
                  <TableCell>{t('cand.col.shape')}</TableCell>
                  <TableCell>{t('cand.col.context')}</TableCell>
                  <TableCell align="right">{t('cand.col.seen')}</TableCell>
                  <TableCell>{t('cand.col.status')}</TableCell>
                  <TableCell>{t('cand.col.lastSeen')}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {cs.map(c => (
                  <TableRow key={c.id} hover>
                    <TableCell sx={{ fontWeight: 600 }}>{c.guessed_name || <Box component="span" sx={{ color: 'text.secondary', fontWeight: 400 }}>{t('cand.unknown')}</Box>}</TableCell>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: 12, color: 'primary.main' }}>{c.sample_masked}</TableCell>
                    <TableCell sx={{ fontFamily: 'monospace', fontSize: 12 }}>
                      {c.charset_class} · {t('cand.chars', { n: c.length_min === c.length_max ? c.length_min : `${c.length_min}–${c.length_max}` })}
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
        {res.data!.total > size && (
          <Pager total={res.data!.total} page={page} size={size} onPage={setPage} onSize={n => { setSize(n); setPage(0) }} />
        )}
      </Card>
    </>
  )
}
