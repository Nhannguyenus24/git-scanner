import { useCallback, useMemo, useState } from 'react'
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
import Tooltip from '@mui/material/Tooltip'
import MailOutlineIcon from '@mui/icons-material/MailOutline'
import SearchIcon from '@mui/icons-material/Search'
import { PageHeader } from '../components/Layout'
import { Empty, LoadError, Loading } from '../components/States'
import { timeAgo } from '../format'
import { useTheme } from '@mui/material/styles'
import { useResource } from '../hooks/useResource'
import { useT } from '../i18n'
import { href, navigate } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import { useToast } from '../store/toast'
import type { Notice, Repository } from '../types'

function Count({ n, color }: { n: number; color: string }) {
  return (
    <Box component="span" sx={{ fontWeight: 700, fontVariantNumeric: 'tabular-nums', color: n ? color : 'text.disabled' }}>
      {n || '·'}
    </Box>
  )
}

const NOTICE_COLOR = { sent: 'success', pending: 'info', failed: 'error', suppressed: 'default' } as const

/** Whether the repository was told, and when — the tooltip carries who and why. */
function NoticeChip({ n }: { n?: Notice }) {
  const { t } = useT()
  if (!n) return <Box component="span" sx={{ color: 'text.disabled', fontSize: 12 }}>{t('notice.none')}</Box>
  const when = n.sent_at ?? n.created_at
  const tip = [n.recipient, timeAgo(when), n.error].filter(Boolean).join(' · ')
  return (
    <Tooltip title={tip}>
      <Chip size="small" color={NOTICE_COLOR[n.state]} variant={n.state === 'sent' ? 'filled' : 'outlined'}
        label={`${t(`notice.${n.state}`)}${n.state === 'sent' ? ` · ${timeAgo(when)}` : ''}`} />
    </Tooltip>
  )
}

export default function RepositoriesPage() {
  const { api } = useSettings()
  const { t } = useT()
  const sev = useTheme().chart.severity
  const notify = useToast()
  const res = useResource(() => api.repositories(), [api], 10000)
  const settings = useResource(() => api.settings(), [api])
  const mailOn = settings.data?.notifications_enabled ?? false

  const disclose = useCallback((r: Repository) => {
    api.disclose(r.id)
      .then(d => { notify(t('repos.notify.queued', { repo: `${r.owner}/${r.name}`, to: d.recipient })); setTimeout(res.reload, 1500) })
      .catch(e => notify(e instanceof Error ? e.message : 'Error', 'error'))
  }, [api, notify, t, res])
  const [q, setQ] = useState('')
  const [exposedOnly, setExposedOnly] = useState(true)

  const repos = useMemo(() => (res.data?.repositories ?? []).filter((r: Repository) =>
    (!exposedOnly || r.open > 0) && `${r.owner}/${r.name}`.toLowerCase().includes(q.toLowerCase())), [res.data, q, exposedOnly])

  if (res.loading) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />
  const all = res.data!.repositories

  return (
    <>
      <PageHeader title={t('repos.title')} subtitle={t('repos.subtitle', { a: all.filter(r => r.open).length, b: all.length })} />
      <Card sx={{ p: 1.5, mb: 2, display: 'flex', gap: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <TextField placeholder={t('repos.filter')} value={q} onChange={e => setQ(e.target.value)} sx={{ flex: '1 1 260px' }}
          InputProps={{ startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment> }} />
        <FormControlLabel control={<Switch checked={exposedOnly} onChange={e => setExposedOnly(e.target.checked)} />} label={t('repos.onlyOpen')} />
        <Chip clickable component="a" href={href('settings')} variant="outlined" color={mailOn ? 'warning' : 'default'}
          icon={<MailOutlineIcon fontSize="small" />} label={`${t('repos.notify.switch')}: ${t(mailOn ? 'settings.on' : 'settings.off')}`} />
      </Card>
      <Card>
        {!repos.length ? <Empty title={t('repos.empty')} action={exposedOnly ? <Button onClick={() => setExposedOnly(false)}>{t('common.showAll')}</Button> : undefined} /> : (
          <TableContainer>
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>{t('repos.col.repository')}</TableCell>
                  <TableCell>{t('repos.col.type')}</TableCell>
                  <TableCell align="center">{t('repos.col.critical')}</TableCell>
                  <TableCell align="center">{t('repos.col.high')}</TableCell>
                  <TableCell align="center">{t('repos.col.medium')}</TableCell>
                  <TableCell align="center">{t('repos.col.low')}</TableCell>
                  <TableCell align="right">{t('repos.col.openTotal')}</TableCell>
                  <TableCell>{t('repos.col.lastScanned')}</TableCell>
                  <TableCell>{t('repos.col.notice')}</TableCell>
                  <TableCell />
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
                    <TableCell align="center"><Count n={r.critical} color={sev.critical} /></TableCell>
                    <TableCell align="center"><Count n={r.high} color={sev.high} /></TableCell>
                    <TableCell align="center"><Count n={r.medium} color={sev.medium} /></TableCell>
                    <TableCell align="center"><Count n={r.low} color={sev.low} /></TableCell>
                    <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums' }}>{r.open} / {r.findings}</TableCell>
                    <TableCell sx={{ color: 'text.secondary', whiteSpace: 'nowrap' }}>{timeAgo(r.last_scanned_at)}</TableCell>
                    <TableCell><NoticeChip n={r.notification} /></TableCell>
                    <TableCell align="right" onClick={e => e.stopPropagation()}>
                      {r.open > 0 && r.notification?.state !== 'sent' && r.notification?.state !== 'pending' && (
                        <Tooltip title={mailOn ? '' : t('repos.notify.offTip')}>
                          <span>
                            <Button size="small" startIcon={<MailOutlineIcon />} disabled={!mailOn} onClick={() => disclose(r)}>
                              {t('repos.notify.action')}
                            </Button>
                          </span>
                        </Tooltip>
                      )}
                    </TableCell>
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
