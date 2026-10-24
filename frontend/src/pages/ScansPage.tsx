import { useMemo, useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import LinearProgress from '@mui/material/LinearProgress'
import Table from '@mui/material/Table'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableContainer from '@mui/material/TableContainer'
import TableHead from '@mui/material/TableHead'
import TableRow from '@mui/material/TableRow'
import ToggleButton from '@mui/material/ToggleButton'
import ToggleButtonGroup from '@mui/material/ToggleButtonGroup'
import AddIcon from '@mui/icons-material/Add'
import { PageHeader } from '../components/Layout'
import StateBadge from '../components/StateBadge'
import { Empty, LoadError, Loading } from '../components/States'
import { duration, num, timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate } from '../hooks/useRoute'
import { useT } from '../i18n'
import { useSettings } from '../store/settings'
import { TERMINAL } from '../types'

type View = 'all' | 'active' | 'done'

export default function ScansPage({ onNewScan }: { onNewScan: () => void }) {
  const { api } = useSettings()
  const { t } = useT()
  const res = useResource(() => api.listScans(200), [api], 3000)
  const [view, setView] = useState<View>('all')

  const scans = useMemo(() => (res.data?.scans ?? []).filter(s =>
    view === 'all' || (view === 'active') !== TERMINAL.has(s.state)), [res.data, view])

  if (res.loading) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />

  return (
    <>
      <PageHeader title={t('scans.title')} subtitle={t('scans.subtitle')}
        actions={<>
          <ToggleButtonGroup size="small" exclusive value={view} onChange={(_, v: View | null) => v && setView(v)}>
            <ToggleButton value="all">{t('common.all')}</ToggleButton>
            <ToggleButton value="active">{t('common.active')}</ToggleButton>
            <ToggleButton value="done">{t('common.finished')}</ToggleButton>
          </ToggleButtonGroup>
          <Button variant="contained" startIcon={<AddIcon />} onClick={onNewScan}>{t('nav.newScan')}</Button>
        </>} />

      <Card>
        {!scans.length ? (
          <Empty title={view === 'active' ? t('scans.emptyActive') : t('scans.empty')}
            action={<Button onClick={onNewScan} sx={{ mt: 1 }}>{t('common.startScan')}</Button>} />
        ) : (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>{t('scans.col.state')}</TableCell>
                  <TableCell>{t('scans.col.target')}</TableCell>
                  <TableCell sx={{ width: 220 }}>{t('scans.col.progress')}</TableCell>
                  <TableCell align="right">{t('scans.col.blobs')}</TableCell>
                  <TableCell align="right">{t('scans.col.newFindings')}</TableCell>
                  <TableCell>{t('scans.col.started')}</TableCell>
                  <TableCell>{t('scans.col.duration')}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {scans.map(s => (
                  <TableRow key={s.id} hover onClick={() => navigate(href('scans', s.id))} sx={{ cursor: 'pointer' }}>
                    <TableCell><StateBadge state={s.state} /></TableCell>
                    <TableCell>
                      <Box sx={{ fontWeight: 600 }}>{s.target.value}</Box>
                      <Box sx={{ fontSize: 11, color: 'text.secondary', fontFamily: 'monospace' }}>{s.target.kind} · {s.id.slice(0, 8)}</Box>
                    </TableCell>
                    <TableCell>
                      <LinearProgress variant="determinate" value={s.progress * 100} sx={{ height: 6, borderRadius: 3 }}
                        color={s.state === 'failed' ? 'error' : TERMINAL.has(s.state) ? 'success' : 'primary'} />
                      <Box sx={{ fontSize: 11, color: 'text.secondary', mt: .5 }}>{t('scans.reposOf', { a: s.repos_scanned, b: s.repos_total })}</Box>
                    </TableCell>
                    <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums' }}>{num(s.blobs_scanned)}</TableCell>
                    <TableCell align="right" sx={{ fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>{s.findings_count}</TableCell>
                    <TableCell sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}>{timeAgo(s.created_at)}</TableCell>
                    <TableCell sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}>{duration(s.started_at, s.finished_at)}</TableCell>
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
