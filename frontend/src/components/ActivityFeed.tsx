import Box from '@mui/material/Box'
import Tooltip from '@mui/material/Tooltip'
import RadarIcon from '@mui/icons-material/RadarOutlined'
import KeyIcon from '@mui/icons-material/VpnKeyOutlined'
import MailIcon from '@mui/icons-material/MailOutline'
import ErrorIcon from '@mui/icons-material/ErrorOutline'
import CheckIcon from '@mui/icons-material/CheckCircleOutline'
import { useTheme } from '@mui/material/styles'
import { EmptyInline } from './States'
import { timeAgo } from '../format'
import { href } from '../hooks/useRoute'
import { useT } from '../i18n'
import type { Finding, NotificationRow, ScanSummary } from '../types'

// Everything here is reconstructed from timestamps the server already returns —
// a scan's created_at and finished_at, a notification's sent_at, a finding's
// detected_at. Nothing is generated in the browser: an invented event would
// survive a reload only in this tab, and a security log that disagrees between
// two tabs is worse than no log. When the API grows a real event stream this
// component changes its source and not its shape.
//
// Labels carry the provider and the repository, never the credential: the
// masked preview is already more than a feed needs.

type Kind = 'scan-done' | 'scan-failed' | 'scan-started' | 'finding' | 'notice' | 'notice-failed'

interface Item {
  id:    string
  kind:  Kind
  at:    string
  text:  string
  sub?:  string
  link:  string
}

const ICON: Record<Kind, React.ReactNode> = {
  'scan-done':     <CheckIcon sx={{ fontSize: 16 }} />,
  'scan-failed':   <ErrorIcon sx={{ fontSize: 16 }} />,
  'scan-started':  <RadarIcon sx={{ fontSize: 16 }} />,
  'finding':       <KeyIcon sx={{ fontSize: 16 }} />,
  'notice':        <MailIcon sx={{ fontSize: 16 }} />,
  'notice-failed': <ErrorIcon sx={{ fontSize: 16 }} />,
}

interface Props {
  scans:         ScanSummary[]
  findings:      Finding[]
  notifications: NotificationRow[]
  limit?:        number
}

export default function ActivityFeed({ scans, findings, notifications, limit = 12 }: Props) {
  const { t } = useT()
  const ch = useTheme().chart

  const color: Record<Kind, string> = {
    'scan-done': ch.categorical[0], 'scan-failed': ch.severity.critical, 'scan-started': ch.textSoft,
    'finding': ch.severity.high, 'notice': ch.categorical[2], 'notice-failed': ch.severity.critical,
  }

  const items: Item[] = []

  for (const s of scans) {
    const target = s.target.value
    if (s.finished_at) {
      items.push({
        id: `s-${s.id}-end`, at: s.finished_at,
        kind: s.state === 'failed' ? 'scan-failed' : 'scan-done',
        text: t(s.state === 'failed' ? 'activity.scanFailed' : 'activity.scanDone', { target }),
        sub: s.state === 'failed' ? (s.error || t('activity.noReason'))
          : t('activity.scanDoneSub', { n: s.findings_count, repos: s.repos_scanned }),
        link: href('scans', s.id),
      })
    } else {
      items.push({
        id: `s-${s.id}-start`, at: s.started_at ?? s.created_at, kind: 'scan-started',
        text: t('activity.scanStarted', { target }),
        sub: t('activity.scanProgress', { a: s.repos_scanned, b: s.repos_total || '?' }),
        link: href('scans', s.id),
      })
    }
  }

  for (const f of findings) {
    items.push({
      id: `f-${f.id}`, at: f.detected_at, kind: 'finding',
      text: t('activity.finding', {
        sev: t(`sev.${f.severity}`),
        provider: f.provider || t('findings.unmapped'),
        repo: `${f.repository.owner}/${f.repository.name}`,
      }),
      sub: f.reachable_from_head ? t('findings.atHeadTip') : t('findings.historyTip'),
      link: href('findings', undefined, { finding: f.id, repository_id: f.repository.id }),
    })
  }

  for (const n of notifications) {
    const when = n.sent_at ?? n.created_at
    if (n.state !== 'sent' && n.state !== 'failed') continue
    items.push({
      id: `n-${n.id}`, at: when,
      kind: n.state === 'failed' ? 'notice-failed' : 'notice',
      text: t(n.state === 'failed' ? 'activity.noticeFailed' : 'activity.notice', {
        repo: `${n.repository.owner}/${n.repository.name}`,
      }),
      sub: n.state === 'failed' ? (n.error || t('activity.noReason')) : t('activity.noticeSub', { n: n.findings }),
      link: href('notifications'),
    })
  }

  const sorted = items
    .filter(i => i.at)
    .sort((a, b) => new Date(b.at).getTime() - new Date(a.at).getTime())
    .slice(0, limit)

  if (!sorted.length) return <EmptyInline title={t('activity.empty')} art="no-data" size={110} />

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column' }}>
      {sorted.map((i, idx) => (
        <Box key={i.id} component="a" href={i.link} sx={{
          display: 'flex', gap: 1.5, py: 1, px: 1, borderRadius: 1, textDecoration: 'none', color: 'inherit',
          borderTop: idx ? 1 : 0, borderColor: 'divider',
          '&:hover': { bgcolor: 'action.hover' },
        }}>
          <Box sx={{ color: color[i.kind], pt: .25, flexShrink: 0 }}>{ICON[i.kind]}</Box>
          <Box sx={{ minWidth: 0, flex: 1 }}>
            <Box sx={{ fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{i.text}</Box>
            {i.sub && <Box sx={{ fontSize: 11.5, color: 'text.secondary', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{i.sub}</Box>}
          </Box>
          <Tooltip title={new Date(i.at).toLocaleString()}>
            <Box sx={{ fontSize: 11, color: 'text.disabled', whiteSpace: 'nowrap', pt: .25 }}>{timeAgo(i.at)}</Box>
          </Tooltip>
        </Box>
      ))}
    </Box>
  )
}
