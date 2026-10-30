import { useCallback, useEffect, useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Chip from '@mui/material/Chip'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import DialogTitle from '@mui/material/DialogTitle'
import FormControlLabel from '@mui/material/FormControlLabel'
import Grid from '@mui/material/Grid'
import InputAdornment from '@mui/material/InputAdornment'
import Link from '@mui/material/Link'
import MenuItem from '@mui/material/MenuItem'
import Switch from '@mui/material/Switch'
import Table from '@mui/material/Table'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableContainer from '@mui/material/TableContainer'
import TableHead from '@mui/material/TableHead'
import TablePagination from '@mui/material/TablePagination'
import TableRow from '@mui/material/TableRow'
import TextField from '@mui/material/TextField'
import Tooltip from '@mui/material/Tooltip'
import { useTheme } from '@mui/material/styles'
import SearchIcon from '@mui/icons-material/Search'
import MailOutlineIcon from '@mui/icons-material/MailOutline'
import { PageHeader } from '../components/Layout'
import StatCard from '../components/StatCard'
import { Empty, LoadError, Loading } from '../components/States'
import { timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { href, navigate } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import { useToast } from '../store/toast'
import { useT } from '../i18n'
import type { NoticeState, NotificationMessage, NotificationRow } from '../types'

const STATES: NoticeState[] = ['sent', 'failed', 'pending', 'suppressed']
const CHIP = { sent: 'success', pending: 'info', failed: 'error', suppressed: 'default' } as const

/** The rendered mail, sandboxed: the HTML comes from the server, but an iframe
 *  without scripts is still the right place for markup meant for a mail client. */
function MessageDialog({ row, onClose }: { row: NotificationRow | null; onClose: () => void }) {
  const { api } = useSettings()
  const { t } = useT()
  const [msg, setMsg] = useState<NotificationMessage>()
  const [err, setErr] = useState('')
  const [plain, setPlain] = useState(false)

  useEffect(() => {
    setMsg(undefined); setErr(''); setPlain(false)
    if (row) api.notificationMessage(row.id).then(setMsg).catch(e => setErr(e instanceof Error ? e.message : String(e)))
  }, [api, row])

  return (
    <Dialog open={!!row} onClose={onClose} maxWidth="md" fullWidth>
      <DialogTitle sx={{ pb: 1 }}>
        <Box sx={{ fontSize: 16, fontWeight: 700 }}>{msg?.subject ?? t('notif.view')}</Box>
        {msg && <Box sx={{ fontSize: 13, color: 'text.secondary', fontWeight: 400 }}>{t('notif.to')}: {msg.to}</Box>}
      </DialogTitle>
      <DialogContent dividers sx={{ p: 0, bgcolor: '#F4F5F7' }}>
        {err ? <Box sx={{ p: 3 }}><LoadError error={err} retry={() => row && api.notificationMessage(row.id).then(m => { setErr(''); setMsg(m) }).catch(() => {})} /></Box>
          : !msg ? <Loading />
          : plain
            ? <Box component="pre" sx={{ m: 0, p: 2, fontSize: 12.5, whiteSpace: 'pre-wrap', bgcolor: 'background.paper', color: 'text.primary' }}>{msg.text}</Box>
            : <Box component="iframe" title="mail" sandbox="allow-popups allow-popups-to-escape-sandbox" srcDoc={msg.html}
                sx={{ display: 'block', width: '100%', height: '70vh', border: 0 }} />}
      </DialogContent>
      <DialogActions sx={{ justifyContent: 'space-between', px: 2 }}>
        <Box sx={{ fontSize: 12, color: 'text.secondary' }}>{t('notif.previewNote')}</Box>
        <Box>
          <FormControlLabel control={<Switch size="small" checked={plain} onChange={e => setPlain(e.target.checked)} />} label={t('notif.plain')} />
          <Button onClick={onClose}>{t('common.close')}</Button>
        </Box>
      </DialogActions>
    </Dialog>
  )
}

export default function NotificationsPage() {
  const { api } = useSettings()
  const { t } = useT()
  const notify = useToast()
  const ch = useTheme().chart
  const [state, setState] = useState<NoticeState | ''>('')
  const [q, setQ] = useState('')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(0)
  const [size, setSize] = useState(50)
  const [viewing, setViewing] = useState<NotificationRow | null>(null)

  // Debounce the free-text box so typing does not fire a request per key.
  useEffect(() => {
    const id = setTimeout(() => { setQuery(q); setPage(0) }, 350)
    return () => clearTimeout(id)
  }, [q])

  const res = useResource(() => api.notifications({ state, q: query, limit: size, offset: page * size }),
    [api, state, query, page, size], 10000)
  const settings = useResource(() => api.settings(), [api])
  const mailOn = settings.data?.notifications_enabled ?? false

  const resend = useCallback((n: NotificationRow) => {
    api.resendNotification(n.id)
      .then(() => { notify(t('notif.resent', { to: n.recipient })); setTimeout(res.reload, 1500) })
      .catch(e => notify(e instanceof Error ? e.message : 'Error', 'error'))
  }, [api, notify, t, res])

  if (res.loading && !res.data) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />
  const data = res.data!
  const count = (k: NoticeState) => data.by_state.find(c => c.key === k)?.count ?? 0
  const accent: Record<NoticeState, string> = { sent: ch.categorical[0], failed: ch.severity.critical, pending: ch.categorical[2], suppressed: ch.other }

  return (
    <>
      <PageHeader title={t('notif.title')} subtitle={t('notif.subtitle')}
        actions={<Chip clickable component="a" href={href('settings')} variant="outlined" color={mailOn ? 'warning' : 'default'}
          icon={<MailOutlineIcon fontSize="small" />} label={`${t('repos.notify.switch')}: ${t(mailOn ? 'settings.on' : 'settings.off')}`} />} />

      <Grid container spacing={2} sx={{ mb: 2 }}>
        {STATES.map(s => (
          <Grid item xs={6} md={3} key={s}>
            <StatCard label={t(`notice.${s}`)} value={count(s)} accent={accent[s]}
              onClick={() => { setState(state === s ? '' : s); setPage(0) }} />
          </Grid>
        ))}
      </Grid>

      <Card sx={{ p: 1.5, mb: 2, display: 'flex', gap: 1.5, flexWrap: 'wrap', alignItems: 'center' }}>
        <TextField placeholder={t('notif.search')} value={q} onChange={e => setQ(e.target.value)} sx={{ flex: '1 1 280px' }}
          InputProps={{ startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment> }} />
        <TextField select label={t('notif.col.state')} value={state} onChange={e => { setState(e.target.value as NoticeState | ''); setPage(0) }} sx={{ minWidth: 160 }}>
          <MenuItem value="">{t('common.any')}</MenuItem>
          {STATES.map(s => <MenuItem key={s} value={s}>{t(`notice.${s}`)}</MenuItem>)}
        </TextField>
      </Card>

      <Card>
        {!data.notifications.length ? <Empty art="mailbox" size={200} title={t('notif.empty')} body={t('notif.emptyBody')} /> : (
          <>
            <TableContainer>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>{t('notif.col.repository')}</TableCell>
                    <TableCell>{t('notif.col.recipient')}</TableCell>
                    <TableCell>{t('notif.col.state')}</TableCell>
                    <TableCell align="right">{t('notif.col.findings')}</TableCell>
                    <TableCell>{t('notif.col.created')}</TableCell>
                    <TableCell>{t('notif.col.sent')}</TableCell>
                    <TableCell />
                  </TableRow>
                </TableHead>
                <TableBody>
                  {data.notifications.map(n => (
                    <TableRow key={n.id} hover>
                      <TableCell>
                        <Link component="button" underline="hover" sx={{ fontWeight: 600, textAlign: 'left' }}
                          onClick={() => navigate(href('findings', undefined, { repository_id: n.repository.id }))}>
                          <Box component="span" sx={{ color: 'text.secondary', fontWeight: 400 }}>{n.repository.owner}/</Box>{n.repository.name}
                        </Link>
                      </TableCell>
                      <TableCell sx={{ fontFamily: 'monospace', fontSize: 12.5 }}>{n.recipient}</TableCell>
                      <TableCell>
                        <Tooltip title={n.error ?? ''}>
                          <Chip size="small" color={CHIP[n.state]} variant={n.state === 'sent' ? 'filled' : 'outlined'} label={t(`notice.${n.state}`)} />
                        </Tooltip>
                      </TableCell>
                      <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums' }}>{n.findings}</TableCell>
                      <TableCell sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}>{timeAgo(n.created_at)}</TableCell>
                      <TableCell sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}>{n.sent_at ? timeAgo(n.sent_at) : '—'}</TableCell>
                      <TableCell align="right" sx={{ whiteSpace: 'nowrap' }}>
                        <Button size="small" onClick={() => setViewing(n)}>{t('notif.view')}</Button>
                        {n.state === 'failed' && (
                          <Tooltip title={mailOn ? '' : t('repos.notify.offTip')}>
                            <span><Button size="small" color="warning" disabled={!mailOn} onClick={() => resend(n)}>{t('notif.resend')}</Button></span>
                          </Tooltip>
                        )}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TableContainer>
            <Box sx={{ borderTop: 1, borderColor: 'divider' }}>
              <TablePagination component="div" count={data.total} page={page} rowsPerPage={size}
                rowsPerPageOptions={[25, 50, 100, 200]}
                onPageChange={(_, p) => setPage(p)}
                onRowsPerPageChange={e => { setSize(Number(e.target.value)); setPage(0) }} />
            </Box>
          </>
        )}
      </Card>

      <MessageDialog row={viewing} onClose={() => setViewing(null)} />
    </>
  )
}
