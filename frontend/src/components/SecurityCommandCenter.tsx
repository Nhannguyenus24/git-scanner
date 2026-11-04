import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Chip from '@mui/material/Chip'
import ArrowForwardIcon from '@mui/icons-material/ArrowForward'
import ErrorOutlineIcon from '@mui/icons-material/ErrorOutline'
import HistoryToggleOffIcon from '@mui/icons-material/HistoryToggleOff'
import RotateRightIcon from '@mui/icons-material/RotateRight'
import ShieldOutlinedIcon from '@mui/icons-material/ShieldOutlined'
import { useTheme } from '@mui/material/styles'
import { href, navigate } from '../hooks/useRoute'
import { useT } from '../i18n'
import { posture, repoRisk } from '../posture'
import type { Overview, Repository } from '../types'

interface Props {
  overview: Overview
  repositories: Repository[]
}

export default function SecurityCommandCenter({ overview, repositories }: Props) {
  const { t } = useT()
  const ch = useTheme().chart
  const score = posture(overview)
  const critical = overview.by_severity.find(c => c.key === 'critical')?.count ?? 0
  const urgent = [
    critical > 0 ? {
      icon: <ErrorOutlineIcon />,
      color: ch.severity.critical,
      title: t('commandCenter.criticalTitle', { n: critical }),
      detail: t('commandCenter.criticalDetail'),
      action: t('commandCenter.reviewAction'),
      run: () => navigate(href('findings', undefined, { severity: 'critical', status: 'open' })),
    } : null,
    overview.reachable_from_head > 0 ? {
      icon: <RotateRightIcon />,
      color: ch.severity.high,
      title: t('commandCenter.headTitle', { n: overview.reachable_from_head }),
      detail: t('commandCenter.headDetail'),
      action: t('commandCenter.reviewAction'),
      run: () => navigate(href('findings', undefined, { exposure: 'head', status: 'open' })),
    } : null,
    overview.candidates > 0 ? {
      icon: <ShieldOutlinedIcon />,
      color: ch.other,
      title: t('commandCenter.candidatesTitle', { n: overview.candidates }),
      detail: t('commandCenter.candidatesDetail'),
      action: t('commandCenter.reviewAction'),
      run: () => navigate(href('candidates')),
    } : null,
  ].filter(Boolean) as { icon: React.ReactNode; color: string; title: string; detail: string; action: string; run: () => void }[]

  const stale = repositories.filter(r => !r.last_scanned_at ||
    Date.now() - new Date(r.last_scanned_at).getTime() > 7 * 24 * 60 * 60 * 1000)
  const top = repositories.slice(0, 3)

  return (
    <Card sx={{
      p: { xs: 2, md: 2.5 }, mb: 2, overflow: 'hidden',
      background: `linear-gradient(120deg, ${ch.inset}, transparent 65%)`,
      borderColor: score.band === 'critical' ? ch.severity.critical : 'divider',
    }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', gap: 2, alignItems: 'flex-start', flexWrap: 'wrap' }}>
        <Box>
          <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
            <ShieldOutlinedIcon sx={{ color: 'primary.main' }} />
            <Box sx={{ fontSize: 18, fontWeight: 800 }}>{t('commandCenter.title')}</Box>
            <Chip size="small" variant="outlined" label={t(`risk.${score.band}`)} sx={{ color: score.band === 'critical' ? ch.severity.critical : 'text.secondary' }} />
          </Box>
          <Box sx={{ color: 'text.secondary', fontSize: 13, mt: .5 }}>{t('commandCenter.subtitle')}</Box>
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
          <Box sx={{ textAlign: 'right' }}>
            <Box sx={{ color: 'text.secondary', fontSize: 11, textTransform: 'uppercase', letterSpacing: '.08em' }}>{t('commandCenter.posture')}</Box>
            <Box sx={{ color: score.score < 65 ? ch.severity.critical : ch.categorical[0], fontSize: 32, fontWeight: 800, lineHeight: 1 }}>{score.score}</Box>
          </Box>
          <Box sx={{ width: 76, height: 8, bgcolor: 'action.hover', borderRadius: 2, overflow: 'hidden' }}>
            <Box sx={{ width: `${score.score}%`, height: '100%', bgcolor: score.score < 65 ? ch.severity.critical : ch.categorical[0] }} />
          </Box>
        </Box>
      </Box>

      <Box sx={{ display: 'grid', gridTemplateColumns: { xs: '1fr', md: '1.2fr 1fr' }, gap: 2, mt: 2 }}>
        <Box>
          <Box sx={{ fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.08em', color: 'text.secondary', mb: .75 }}>
            {t('commandCenter.actions', { n: urgent.length })}
          </Box>
          {urgent.length ? urgent.slice(0, 3).map(item => (
            <Box key={item.title} sx={{ display: 'flex', gap: 1, alignItems: 'center', py: .8, borderTop: 1, borderColor: 'divider' }}>
              <Box sx={{ color: item.color, display: 'flex' }}>{item.icon}</Box>
              <Box sx={{ minWidth: 0, flex: 1 }}>
                <Box sx={{ fontSize: 13, fontWeight: 700 }}>{item.title}</Box>
                <Box sx={{ color: 'text.secondary', fontSize: 11.5 }}>{item.detail}</Box>
              </Box>
              <Button size="small" endIcon={<ArrowForwardIcon />} onClick={item.run}>{item.action}</Button>
            </Box>
          )) : (
            <Box sx={{ py: 1.5, color: 'success.main', fontSize: 13, display: 'flex', gap: 1, alignItems: 'center' }}>
              <ShieldOutlinedIcon fontSize="small" /> {t('commandCenter.noActions')}
            </Box>
          )}
        </Box>

        <Box>
          <Box sx={{ fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.08em', color: 'text.secondary', mb: .75 }}>
            {t('commandCenter.hotspots')}
          </Box>
          {top.length ? top.map(repo => {
            const risk = repoRisk(repo)
            return (
              <Box key={repo.id} component="button" onClick={() => navigate(href('findings', undefined, { repository_id: repo.id }))}
                sx={{ all: 'unset', boxSizing: 'border-box', width: '100%', display: 'flex', gap: 1, alignItems: 'center', py: .65, borderTop: 1, borderColor: 'divider', cursor: 'pointer' }}>
                <Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: risk.band === 'critical' ? ch.severity.critical : risk.band === 'high' ? ch.severity.high : ch.severity.medium }} />
                <Box sx={{ flex: 1, minWidth: 0, fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{repo.owner}/{repo.name}</Box>
                <Box sx={{ fontWeight: 800, color: risk.band === 'critical' ? ch.severity.critical : ch.severity.high }}>{risk.score}</Box>
              </Box>
            )
          }) : <Box sx={{ color: 'text.secondary', fontSize: 13 }}>{t('commandCenter.noRepositories')}</Box>}
          {stale.length > 0 && (
            <Button size="small" color="inherit" startIcon={<HistoryToggleOffIcon />} onClick={() => navigate(href('repositories'))} sx={{ mt: .75 }}>
              {t('commandCenter.stale', { n: stale.length })}
            </Button>
          )}
        </Box>
      </Box>
    </Card>
  )
}
