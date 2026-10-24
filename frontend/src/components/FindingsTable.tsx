import { useState } from 'react'
import Box from '@mui/material/Box'
import Chip from '@mui/material/Chip'
import IconButton from '@mui/material/IconButton'
import Link from '@mui/material/Link'
import ListItemText from '@mui/material/ListItemText'
import Menu from '@mui/material/Menu'
import MenuItem from '@mui/material/MenuItem'
import Table from '@mui/material/Table'
import TableBody from '@mui/material/TableBody'
import TableCell from '@mui/material/TableCell'
import TableContainer from '@mui/material/TableContainer'
import TableHead from '@mui/material/TableHead'
import TableRow from '@mui/material/TableRow'
import Tooltip from '@mui/material/Tooltip'
import Typography from '@mui/material/Typography'
import OpenInNewIcon from '@mui/icons-material/OpenInNew'
import MoreVertIcon from '@mui/icons-material/MoreVert'
import SeverityChip from './SeverityChip'
import { commitLink, fileLink, timeAgo } from '../format'
import { href } from '../hooks/useRoute'
import { useT } from '../i18n'
import { STATUSES, type Finding, type FindingStatus } from '../types'

interface Props {
  findings:      Finding[]
  onStatus:      (f: Finding, s: FindingStatus) => void
  showRepo?:     boolean
  emptyMessage?: string
}

const STATUS_COLOR: Record<FindingStatus, 'error' | 'success' | 'default' | 'warning'> = {
  open: 'error', rotated: 'success', false_positive: 'default', ignored: 'default',
}

// ProbeBadge turns a live-credential check into the finding's loudest signal:
// a confirmed-live key is the one to rotate first. Nothing shows when probing
// was not run for this finding.
function ProbeBadge({ f, t }: { f: Finding; t: (k: string, v?: Record<string, string | number>) => string }) {
  const status = f.metadata?.probe_status
  if (!status) return null
  const who = f.metadata.probe_owner || f.metadata.probe_account || f.metadata.probe_team
  if (status === 'live') {
    return (
      <Tooltip title={who ? t('findings.probeLiveTip', { who }) : t('findings.probeLiveTipNo')}>
        <Chip size="small" color="error" label={t('probe.live')} />
      </Tooltip>
    )
  }
  if (status === 'dead') {
    return <Tooltip title={t('findings.probeDeadTip')}><Chip size="small" variant="outlined" color="success" label={t('probe.dead')} /></Tooltip>
  }
  return <Tooltip title={t('findings.probeOtherTip')}><Chip size="small" variant="outlined" label={t('probe.unverified')} /></Tooltip>
}

export default function FindingsTable({ findings, onStatus, showRepo = true, emptyMessage }: Props) {
  const { t } = useT()
  const [menu, setMenu] = useState<{ el: HTMLElement; f: Finding } | null>(null)

  if (!findings.length) {
    return (
      <Box sx={{ p: 5, textAlign: 'center', color: 'text.secondary' }}>
        <Typography variant="body2">{emptyMessage ?? t('findings.empty')}</Typography>
      </Box>
    )
  }

  return (
    <>
      <TableContainer>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell>{t('findings.col.severity')}</TableCell>
              <TableCell>{t('findings.col.credential')}</TableCell>
              {showRepo && <TableCell>{t('findings.col.repository')}</TableCell>}
              <TableCell>{t('findings.col.location')}</TableCell>
              <TableCell>{t('findings.col.exposure')}</TableCell>
              <TableCell>{t('findings.col.status')}</TableCell>
              <TableCell>{t('findings.col.detected')}</TableCell>
              <TableCell padding="checkbox" />
            </TableRow>
          </TableHead>
          <TableBody>
            {findings.map(f => {
              const link = fileLink(f.repository.url, f.file_path, f.line_number, f.commit_sha, f.reachable_from_head)
              return (
                <TableRow key={f.id} hover sx={{ opacity: f.status === 'open' ? 1 : .55 }}>
                  <TableCell><SeverityChip severity={f.severity} /></TableCell>
                  <TableCell>
                    <Box sx={{ fontWeight: 600, fontSize: 13 }}>{f.provider || <Box component="span" sx={{ color: 'text.secondary' }}>{t('findings.unmapped')}</Box>}</Box>
                    <Box sx={{ fontFamily: 'monospace', fontSize: 11, color: 'text.secondary' }}>
                      {f.rule_id} · <Box component="span" sx={{ color: 'primary.main' }}>{f.masked_preview}</Box>
                    </Box>
                  </TableCell>
                  {showRepo && (
                    <TableCell>
                      <Link href={href('repositories', f.repository.id)} underline="hover" color="inherit" sx={{ fontSize: 13 }}>
                        <Box component="span" sx={{ color: 'text.secondary' }}>{f.repository.owner}/</Box>{f.repository.name}
                      </Link>
                    </TableCell>
                  )}
                  <TableCell sx={{ maxWidth: 360 }}>
                    <Tooltip title={`${f.file_path}:${f.line_number}`} placement="top-start">
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: .5, fontFamily: 'monospace', fontSize: 12 }}>
                        <Box component="span" sx={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', direction: 'rtl', textAlign: 'left' }}>
                          {f.file_path}:{f.line_number}
                        </Box>
                        {link && (
                          <IconButton size="small" href={link} target="_blank" rel="noreferrer" sx={{ p: .25 }}>
                            <OpenInNewIcon sx={{ fontSize: 14 }} />
                          </IconButton>
                        )}
                      </Box>
                    </Tooltip>
                    {f.commit_sha ? (
                      <Box sx={{ fontSize: 11, color: 'text.secondary', mt: .25, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
                        <Link href={commitLink(f.repository.url, f.commit_sha)} target="_blank" rel="noreferrer"
                          underline="hover" sx={{ fontFamily: 'monospace' }}>{f.commit_sha.slice(0, 7)}</Link>
                        {f.author_name && <> · {f.author_name}</>}
                        {f.committed_at && <Tooltip title={new Date(f.committed_at).toLocaleString()}><span> · {timeAgo(f.committed_at)}</span></Tooltip>}
                      </Box>
                    ) : (
                      <Box sx={{ fontSize: 11, color: 'text.disabled', mt: .25 }}>{t('findings.commitUnknown')}</Box>
                    )}
                  </TableCell>
                  <TableCell>
                    <Box sx={{ display: 'flex', gap: .5, flexWrap: 'wrap' }}>
                      <Tooltip title={f.reachable_from_head ? t('findings.atHeadTip') : t('findings.historyTip')}>
                        <Chip size="small" variant="outlined" label={f.reachable_from_head ? t('findings.atHead') : t('findings.history')}
                          color={f.reachable_from_head ? 'error' : 'default'} />
                      </Tooltip>
                      <ProbeBadge f={f} t={t} />
                    </Box>
                  </TableCell>
                  <TableCell>
                    <Chip size="small" label={t(`status.${f.status}`)} color={STATUS_COLOR[f.status]}
                      variant={f.status === 'open' ? 'filled' : 'outlined'} />
                  </TableCell>
                  <TableCell sx={{ fontSize: 12, color: 'text.secondary', whiteSpace: 'nowrap' }}>{timeAgo(f.detected_at)}</TableCell>
                  <TableCell padding="checkbox">
                    <IconButton size="small" onClick={e => setMenu({ el: e.currentTarget, f })} aria-label={t('findings.markAs')}>
                      <MoreVertIcon fontSize="small" />
                    </IconButton>
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </TableContainer>

      <Menu anchorEl={menu?.el} open={!!menu} onClose={() => setMenu(null)}>
        <MenuItem disabled sx={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '.07em' }}>{t('findings.markAs')}</MenuItem>
        {STATUSES.map(s => (
          <MenuItem key={s} selected={menu?.f.status === s}
            onClick={() => { if (menu) onStatus(menu.f, s); setMenu(null) }}>
            <ListItemText>{t(`status.${s}`)}</ListItemText>
          </MenuItem>
        ))}
      </Menu>
    </>
  )
}
