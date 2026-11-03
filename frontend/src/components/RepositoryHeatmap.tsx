import Box from '@mui/material/Box'
import Card from '@mui/material/Card'
import Chip from '@mui/material/Chip'
import Tooltip from '@mui/material/Tooltip'
import { useTheme } from '@mui/material/styles'
import ArrowUpwardIcon from '@mui/icons-material/ArrowUpward'
import AccessTimeIcon from '@mui/icons-material/AccessTime'
import { repoRisk } from '../posture'
import { timeAgo } from '../format'
import { useT } from '../i18n'
import type { Repository } from '../types'

interface Props {
  repos: Repository[]
  onOpen: (repo: Repository) => void
}

function isStale(lastScannedAt?: string): boolean {
  if (!lastScannedAt) return true
  return Date.now() - new Date(lastScannedAt).getTime() > 7 * 24 * 60 * 60 * 1000
}

export default function RepositoryHeatmap({ repos, onOpen }: Props) {
  const { t } = useT()
  const theme = useTheme()
  const ch = theme.chart
  const colorFor = (score: number) => {
    if (score >= 75) return ch.severity.critical
    if (score >= 50) return ch.severity.high
    if (score >= 25) return ch.severity.medium
    return ch.categorical[0]
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2, flexWrap: 'wrap', mb: 2 }}>
        <Box sx={{ color: 'text.secondary', fontSize: 13 }}>{t('repos.heatmap.help')}</Box>
        <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
          {(['low', 'medium', 'high', 'critical'] as const).map(band => (
            <Box key={band} sx={{ display: 'flex', alignItems: 'center', gap: .5, fontSize: 11, color: 'text.secondary' }}>
              <Box sx={{ width: 9, height: 9, borderRadius: '50%', bgcolor: colorFor(band === 'low' ? 0 : band === 'medium' ? 25 : band === 'high' ? 50 : 75) }} />
              {t(`risk.${band}`)}
            </Box>
          ))}
          <Box sx={{ display: 'flex', alignItems: 'center', gap: .5, fontSize: 11, color: 'text.secondary' }}>
            <AccessTimeIcon sx={{ fontSize: 13 }} /> {t('repos.heatmap.stale')}
          </Box>
        </Box>
      </Box>
      <Box sx={{
        display: 'grid',
        gridTemplateColumns: { xs: '1fr', sm: 'repeat(2, minmax(0, 1fr))', lg: 'repeat(3, minmax(0, 1fr))' },
        gap: 1.5,
      }}>
        {repos.map(repo => {
          const risk = repoRisk(repo)
          const color = colorFor(risk.score)
          const stale = isStale(repo.last_scanned_at)
          return (
            <Card key={repo.id} role="button" tabIndex={0} onClick={() => onOpen(repo)}
              onKeyDown={event => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); onOpen(repo) } }}
              sx={{
              textAlign: 'left', p: 1.75, cursor: 'pointer', color: 'inherit',
              border: 1, borderColor: 'divider', borderLeft: `5px solid ${color}`,
              transition: 'transform .15s, box-shadow .15s',
              '&:hover': { transform: 'translateY(-2px)', boxShadow: 4 },
            }}>
              <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 1, alignItems: 'flex-start' }}>
                <Box sx={{ minWidth: 0 }}>
                  <Box sx={{ fontWeight: 700, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    <Box component="span" sx={{ color: 'text.secondary', fontWeight: 400 }}>{repo.owner}/</Box>{repo.name}
                  </Box>
                  <Box sx={{ color: 'text.secondary', fontSize: 11.5, mt: .35 }}>
                    {repo.app_type || t('repos.heatmap.repository')}
                  </Box>
                </Box>
                <Tooltip title={t('repos.heatmap.scoreTip')}>
                  <Box sx={{ color, fontWeight: 800, fontSize: 22, lineHeight: 1, fontVariantNumeric: 'tabular-nums' }}>{risk.score}</Box>
                </Tooltip>
              </Box>
              <Box sx={{ mt: 1.5, height: 7, bgcolor: ch.inset, borderRadius: 1, overflow: 'hidden' }}>
                <Box sx={{ width: `${risk.score}%`, height: '100%', bgcolor: color }} />
              </Box>
              <Box sx={{ display: 'flex', alignItems: 'center', gap: .75, mt: 1.25, flexWrap: 'wrap' }}>
                {repo.critical > 0 && <Chip size="small" label={`${repo.critical} ${t('sev.critical')}`} sx={{ color: ch.severity.critical, borderColor: ch.severity.critical }} variant="outlined" />}
                {repo.high > 0 && <Chip size="small" label={`${repo.high} ${t('sev.high')}`} sx={{ color: ch.severity.high, borderColor: ch.severity.high }} variant="outlined" />}
                <Box sx={{ ml: 'auto', color: 'text.secondary', fontSize: 11, display: 'flex', alignItems: 'center', gap: .35 }}>
                  {stale && <AccessTimeIcon sx={{ fontSize: 14, color: 'warning.main' }} />}
                  {repo.last_scanned_at ? timeAgo(repo.last_scanned_at) : t('repos.heatmap.never')}
                </Box>
              </Box>
              <Box sx={{ display: 'flex', gap: 1.5, mt: 1, color: 'text.secondary', fontSize: 11.5 }}>
                <span>{t('repos.heatmap.open', { n: repo.open })}</span>
                <span>{t('repos.heatmap.total', { n: repo.findings })}</span>
                {repo.is_fork && <span>fork</span>}
                {repo.open > 0 && <ArrowUpwardIcon sx={{ fontSize: 14, color }} />}
              </Box>
            </Card>
          )
        })}
      </Box>
    </Box>
  )
}
