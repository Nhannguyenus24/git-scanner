import { useCallback, useMemo, useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Chip from '@mui/material/Chip'
import InputAdornment from '@mui/material/InputAdornment'
import MenuItem from '@mui/material/MenuItem'
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
import IconButton from '@mui/material/IconButton'
import MailOutlineIcon from '@mui/icons-material/MailOutline'
import ShareIcon from '@mui/icons-material/IosShare'
import ShareDialog from '../components/ShareDialog'
import SearchIcon from '@mui/icons-material/Search'
import { PageHeader } from '../components/Layout'
import { Empty, LoadError, TableSkeleton } from '../components/States'
import { timeAgo } from '../format'
import { useTheme } from '@mui/material/styles'
import { repoRisk } from '../posture'
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

/** The risk number with a bar behind it, so the ranking reads without arithmetic. */
function RiskCell({ repo }: { repo: Repository }) {
  const { t } = useT()
  const ch = useTheme().chart
  const r = repoRisk(repo)
  const color = r.band === 'critical' ? ch.severity.critical
    : r.band === 'high' ? ch.severity.high
    : r.band === 'medium' ? ch.severity.medium
    : ch.categorical[0]
  return (
    <Tooltip title={t('repos.riskTip', {
      critical: repo.critical, high: repo.high, medium: repo.medium, low: repo.low,
    })}>
      <Box sx={{ minWidth: 86 }}>
        <Box sx={{ display: 'flex', justifyContent: 'space-between', fontSize: 11.5, mb: .3 }}>
          <Box component="span" sx={{ fontWeight: 700, color }}>{t(`risk.${r.band}`)}</Box>
          <Box component="span" sx={{ fontFamily: '"IBM Plex Mono", monospace', color: 'text.secondary' }}>{r.score}</Box>
        </Box>
        <Box sx={{ height: 5, bgcolor: ch.inset, border: `1px solid ${ch.line}`, borderRadius: .5, overflow: 'hidden' }}>
          <Box sx={{ height: '100%', width: `${r.score}%`, bgcolor: color }} />
        </Box>
      </Box>
    </Tooltip>
  )
}

type Sort = 'risk' | 'open' | 'recent' | 'name'

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
  const [sort, setSort] = useState<Sort>('risk')
  const [sharing, setSharing] = useState<Repository>()

  const repos = useMemo(() => {
    const list = (res.data?.repositories ?? []).filter((r: Repository) =>
      (!exposedOnly || r.open > 0) && `${r.owner}/${r.name}`.toLowerCase().includes(q.toLowerCase()))
    const by: Record<Sort, (a: Repository, b: Repository) => number> = {
      risk:   (a, b) => repoRisk(b).score - repoRisk(a).score,
      open:   (a, b) => b.open - a.open,
      recent: (a, b) => +new Date(b.last_scanned_at ?? 0) - +new Date(a.last_scanned_at ?? 0),
      name:   (a, b) => `${a.owner}/${a.name}`.localeCompare(`${b.owner}/${b.name}`),
    }
    return [...list].sort(by[sort])
  }, [res.data, q, exposedOnly, sort])

  if (res.loading && !res.data) return <Card><TableSkeleton rows={8} columns={7} /></Card>
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />
  const all = res.data!.repositories

  return (
    <>
      <PageHeader title={t('repos.title')} subtitle={t('repos.subtitle', { a: all.filter(r => r.open).length, b: all.length })} />
      <Card sx={{ p: 1.5, mb: 2, display: 'flex', gap: 2, alignItems: 'center', flexWrap: 'wrap' }}>
        <TextField placeholder={t('repos.filter')} value={q} onChange={e => setQ(e.target.value)} sx={{ flex: '1 1 220px' }}
          inputProps={{ 'aria-label': t('repos.filter') }}
          InputProps={{ startAdornment: <InputAdornment position="start"><SearchIcon fontSize="small" /></InputAdornment> }} />
        <TextField select label={t('repos.sort')} value={sort} onChange={e => setSort(e.target.value as Sort)} sx={{ minWidth: 160 }}>
          <MenuItem value="risk">{t('repos.sortRisk')}</MenuItem>
          <MenuItem value="open">{t('repos.sortOpen')}</MenuItem>
          <MenuItem value="recent">{t('repos.sortRecent')}</MenuItem>
          <MenuItem value="name">{t('repos.sortName')}</MenuItem>
        </TextField>
        <FormControlLabel control={<Switch checked={exposedOnly} onChange={e => setExposedOnly(e.target.checked)} />} label={t('repos.onlyOpen')} />
        <Chip clickable component="a" href={href('settings')} variant="outlined" color={mailOn ? 'warning' : 'default'}
          icon={<MailOutlineIcon fontSize="small" />} label={`${t('repos.notify.switch')}: ${t(mailOn ? 'settings.on' : 'settings.off')}`} />
      </Card>
      <Card>
        {!repos.length ? (
          <Empty art="repositories" size={200} title={t('repos.empty')} body={t('repos.emptyBody')}
            action={exposedOnly ? <Button onClick={() => setExposedOnly(false)}>{t('common.showAll')}</Button> : undefined} />
        ) : (
          <TableContainer>
            <Table size="small" stickyHeader>
              <TableHead>
                <TableRow>
                  <TableCell>{t('repos.col.repository')}</TableCell>
                  <TableCell>{t('repos.col.risk')}</TableCell>
                  <TableCell align="center" sx={{ display: { xs: 'none', md: 'table-cell' } }}>{t('repos.col.critical')}</TableCell>
                  <TableCell align="center" sx={{ display: { xs: 'none', md: 'table-cell' } }}>{t('repos.col.high')}</TableCell>
                  <TableCell align="center" sx={{ display: { xs: 'none', lg: 'table-cell' } }}>{t('repos.col.medium')}</TableCell>
                  <TableCell align="center" sx={{ display: { xs: 'none', lg: 'table-cell' } }}>{t('repos.col.low')}</TableCell>
                  <TableCell align="right">{t('repos.col.openTotal')}</TableCell>
                  <TableCell sx={{ display: { xs: 'none', md: 'table-cell' } }}>{t('repos.col.lastScanned')}</TableCell>
                  <TableCell sx={{ display: { xs: 'none', sm: 'table-cell' } }}>{t('repos.col.notice')}</TableCell>
                  <TableCell>{t('repos.col.next')}</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {repos.map(r => {
                  const risk = repoRisk(r)
                  return (
                    <TableRow key={r.id} hover sx={{ cursor: 'pointer' }}
                      onClick={() => navigate(href('findings', undefined, { repository_id: r.id }))}>
                      <TableCell>
                        <Box sx={{ fontWeight: 600 }}><Box component="span" sx={{ color: 'text.secondary', fontWeight: 400 }}>{r.owner}/</Box>{r.name}</Box>
                        <Box sx={{ display: 'flex', gap: .5, mt: .25, alignItems: 'center' }}>
                          {r.app_type && <Box component="span" sx={{ fontSize: 11, color: 'text.secondary' }}>{r.app_type}</Box>}
                          {r.is_fork && <Chip size="small" label="fork" variant="outlined" />}
                          {r.stars > 0 && <Chip size="small" label={`★ ${r.stars}`} variant="outlined" />}
                        </Box>
                      </TableCell>
                      <TableCell><RiskCell repo={r} /></TableCell>
                      <TableCell align="center" sx={{ display: { xs: 'none', md: 'table-cell' } }}><Count n={r.critical} color={sev.critical} /></TableCell>
                      <TableCell align="center" sx={{ display: { xs: 'none', md: 'table-cell' } }}><Count n={r.high} color={sev.high} /></TableCell>
                      <TableCell align="center" sx={{ display: { xs: 'none', lg: 'table-cell' } }}><Count n={r.medium} color={sev.medium} /></TableCell>
                      <TableCell align="center" sx={{ display: { xs: 'none', lg: 'table-cell' } }}><Count n={r.low} color={sev.low} /></TableCell>
                      <TableCell align="right" sx={{ fontVariantNumeric: 'tabular-nums' }}>{r.open} / {r.findings}</TableCell>
                      <TableCell sx={{ color: 'text.secondary', whiteSpace: 'nowrap', display: { xs: 'none', md: 'table-cell' } }}>{timeAgo(r.last_scanned_at)}</TableCell>
                      <TableCell sx={{ display: { xs: 'none', sm: 'table-cell' } }}><NoticeChip n={r.notification} /></TableCell>
                      <TableCell onClick={e => e.stopPropagation()} sx={{ whiteSpace: 'nowrap' }}>
                        {r.open > 0 && r.notification?.state !== 'sent' && r.notification?.state !== 'pending' ? (
                          <Tooltip title={mailOn ? t(risk.action) : t('repos.notify.offTip')}>
                            <span>
                              <Button size="small" startIcon={<MailOutlineIcon />} disabled={!mailOn} onClick={() => disclose(r)}>
                                {t('repos.notify.action')}
                              </Button>
                            </span>
                          </Tooltip>
                        ) : (
                          <Box component="span" sx={{ fontSize: 12, color: 'text.secondary' }}>{t(risk.action)}</Box>
                        )}
                        {r.findings > 0 && (
                          <Tooltip title={t('share.action')}>
                            <IconButton size="small" aria-label={t('share.actionFor', { repo: `${r.owner}/${r.name}` })} onClick={() => setSharing(r)}>
                              <ShareIcon fontSize="small" />
                            </IconButton>
                          </Tooltip>
                        )}
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </TableContainer>
        )}
      </Card>
      {sharing && <ShareDialog repo={sharing} onClose={() => setSharing(undefined)} />}
    </>
  )
}
