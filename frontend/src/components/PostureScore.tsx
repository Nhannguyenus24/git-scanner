import { useState } from 'react'
import Box from '@mui/material/Box'
import Card from '@mui/material/Card'
import Collapse from '@mui/material/Collapse'
import Button from '@mui/material/Button'
import LinearProgress from '@mui/material/LinearProgress'
import Tooltip from '@mui/material/Tooltip'
import ExpandMoreIcon from '@mui/icons-material/ExpandMore'
import { useTheme } from '@mui/material/styles'
import { posture, type Posture } from '../posture'
import type { ChartPalette } from '../theme'
import { useT } from '../i18n'
import type { Overview, Repository } from '../types'

// The score is shown with its arithmetic, not instead of it. Each factor prints
// what was measured, what it cost, and what the worst case would have been, so
// the number can be argued with — a security score nobody can check is a number
// people stop reading after a week.

function bandColor(p: Posture, ch: ChartPalette): string {
  return p.band === 'good' ? ch.categorical[0]
    : p.band === 'fair' ? ch.severity.medium
    : p.band === 'poor' ? ch.severity.high
    : ch.severity.critical
}

export default function PostureScore({ overview, repositories }: { overview: Overview; repositories?: Repository[] }) {
  const { t } = useT()
  const ch = useTheme().chart
  const [open, setOpen] = useState(false)
  const p = posture(overview, repositories)
  const color = bandColor(p, ch)

  return (
    <Card sx={{ p: 2.5 }}>
      <Box sx={{ display: 'flex', gap: 3, alignItems: 'center', flexWrap: 'wrap' }}>
        <Box>
          <Box sx={{ fontSize: 11, fontWeight: 700, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '.07em' }}>
            {t('posture.title')}
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1, mt: .5 }}>
            <Box sx={{ fontSize: 42, fontWeight: 800, lineHeight: 1, color, fontVariantNumeric: 'tabular-nums' }}>
              {p.scored ? p.score : '—'}
            </Box>
            <Box sx={{ fontSize: 16, color: 'text.secondary' }}>/ 100</Box>
          </Box>
          <Box sx={{ fontSize: 13, fontWeight: 700, color, mt: .25 }}>
            {p.scored ? t(`posture.band.${p.band}`) : t('posture.unscored')}
          </Box>
        </Box>

        <Box sx={{ flex: '1 1 320px', minWidth: 240 }}>
          {/* The bar is a second, redundant encoding of the same number, so the
              band is never carried by colour alone. */}
          <LinearProgress
            variant="determinate" value={p.scored ? p.score : 0}
            aria-label={t('posture.title')}
            sx={{ height: 10, borderRadius: 1, '& .MuiLinearProgress-bar': { backgroundColor: color } }} />
          <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mt: 1.25 }}>
            {p.factors.filter(f => f.penalty > 0).map(f => (
              <Tooltip key={f.key} title={t(`${f.key}.why`)}>
                <Box sx={{
                  fontSize: 11.5, fontFamily: '"IBM Plex Mono", monospace',
                  px: .75, py: .25, borderRadius: .5, border: 1, borderColor: 'divider', color: 'text.secondary',
                }}>
                  {t(f.key)} −{f.penalty}
                </Box>
              </Tooltip>
            ))}
            {p.scored && p.factors.every(f => f.penalty === 0) && (
              <Box sx={{ fontSize: 12.5, color: 'text.secondary' }}>{t('posture.perfect')}</Box>
            )}
          </Box>
        </Box>

        <Button size="small" onClick={() => setOpen(o => !o)} endIcon={
          <ExpandMoreIcon sx={{ transform: open ? 'rotate(180deg)' : 'none', transition: 'transform .18s' }} />
        } aria-expanded={open}>
          {t('posture.explain')}
        </Button>
      </Box>

      <Collapse in={open} unmountOnExit>
        <Box sx={{ mt: 2, pt: 2, borderTop: 1, borderColor: 'divider' }}>
          <Box sx={{ fontSize: 12.5, color: 'text.secondary', mb: 1.5 }}>{t('posture.method')}</Box>
          <Box sx={{ display: 'grid', gap: .75 }}>
            {p.factors.map(f => (
              <Box key={f.key} sx={{
                display: 'grid', gridTemplateColumns: { xs: '1fr auto', sm: '180px 1fr 70px' },
                gap: 1.5, alignItems: 'center', fontSize: 13,
              }}>
                <Box sx={{ fontWeight: 600 }}>{t(f.key)}</Box>
                <Box sx={{ color: 'text.secondary', display: { xs: 'none', sm: 'block' } }}>{t(`${f.key}.detail`, f.detail)}</Box>
                <Box sx={{
                  textAlign: 'right', fontFamily: '"IBM Plex Mono", monospace',
                  color: f.penalty > 0 ? 'error.main' : 'text.disabled',
                }}>
                  −{f.penalty} / {f.max}
                </Box>
              </Box>
            ))}
          </Box>
          <Box sx={{ fontSize: 11.5, color: 'text.disabled', mt: 2 }}>{t('posture.derivedNote')}</Box>
        </Box>
      </Collapse>
    </Card>
  )
}
