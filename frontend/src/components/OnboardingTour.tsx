import { useEffect, useRef, useState, type KeyboardEvent, type TouchEvent } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import ButtonBase from '@mui/material/ButtonBase'
import Dialog from '@mui/material/Dialog'
import Fade from '@mui/material/Fade'
import LinearProgress from '@mui/material/LinearProgress'
import TextField from '@mui/material/TextField'
import useMediaQuery from '@mui/material/useMediaQuery'
import { useTheme } from '@mui/material/styles'
import ArrowBackIcon from '@mui/icons-material/ArrowBack'
import ArrowForwardIcon from '@mui/icons-material/ArrowForward'
import Illustration, { type IllustrationName } from './Illustration'
import { useApiStatus } from '../hooks/useApiStatus'
import { href, navigate } from '../hooks/useRoute'
import { usePrefs } from '../store/preferences'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'

// A seven-step first run. Only "this person has seen it" is stored, locally —
// no progress, no answers, nothing about the scans they looked at. It can be
// skipped at any step and restarted from settings.

interface Step {
  key:  string
  art:  IllustrationName
  /** An extra control for steps that actually do something. */
  body?: 'api' | 'scan'
}

const STEPS: Step[] = [
  { key: 'tour.welcome',  art: 'welcome' },
  { key: 'tour.api',      art: 'setup', body: 'api' },
  { key: 'tour.scan',     art: 'searching', body: 'scan' },
  { key: 'tour.overview', art: 'no-data' },
  { key: 'tour.triage',   art: 'investigate' },
  { key: 'tour.masking',  art: 'security' },
  { key: 'tour.notify',   art: 'mailbox' },
]

// A swipe has to travel this far, and further sideways than down, so scrolling
// a long step on a short phone is never read as a page turn.
const SWIPE_PX = 50

export default function OnboardingTour({ open, onClose, onNewScan }: {
  open: boolean
  onClose: () => void
  onNewScan: () => void
}) {
  const { t } = useT()
  const { set } = usePrefs()
  const { baseUrl, setBaseUrl } = useSettings()
  const status = useApiStatus(baseUrl)
  const phone = useMediaQuery(useTheme().breakpoints.down('sm'))
  const [i, setI] = useState(0)
  const touch = useRef<{ x: number, y: number } | null>(null)
  const step = STEPS[i]

  // The dialog stays mounted between runs, so a replay from settings would
  // otherwise reopen on whichever step the last run was closed at.
  useEffect(() => { if (open) setI(0) }, [open])
  const last = i === STEPS.length - 1

  const finish = () => { set('onboarded', true); onClose() }
  const go = (n: number) => setI(Math.max(0, Math.min(STEPS.length - 1, n)))
  const done = () => { finish(); navigate(href('overview')) }

  const onTouchStart = (e: TouchEvent) => {
    touch.current = { x: e.touches[0].clientX, y: e.touches[0].clientY }
  }
  const onTouchEnd = (e: TouchEvent) => {
    const from = touch.current
    touch.current = null
    if (!from) return
    const dx = e.changedTouches[0].clientX - from.x
    const dy = e.changedTouches[0].clientY - from.y
    if (Math.abs(dx) < SWIPE_PX || Math.abs(dx) < Math.abs(dy)) return
    go(dx < 0 ? i + 1 : i - 1)
  }

  // Arrow keys turn the page, except inside the API field where they move the
  // caret.
  const onKeyDown = (e: KeyboardEvent) => {
    if ((e.target as HTMLElement).closest('input, textarea')) return
    if (e.key === 'ArrowRight') go(i + 1)
    else if (e.key === 'ArrowLeft') go(i - 1)
  }

  const statusColor = status === 'ok' ? 'success.main' : status === 'error' ? 'error.main' : 'text.disabled'

  return (
    <Dialog open={open} onClose={finish} fullScreen={phone} fullWidth maxWidth="sm"
      aria-labelledby="tour-title" onKeyDown={onKeyDown}
      PaperProps={{ sx: { display: 'flex', flexDirection: 'column', ...(phone ? {} : { minHeight: 560 }) } }}>

      <LinearProgress variant="determinate" value={((i + 1) / STEPS.length) * 100}
        aria-label={t('tour.progress', { n: i + 1, total: STEPS.length })} sx={{ height: 3 }} />

      <Box sx={{
        display: 'flex', alignItems: 'center', justifyContent: 'space-between',
        px: 2, pt: phone ? 'max(8px, env(safe-area-inset-top))' : 1,
      }}>
        <Box sx={{ fontSize: 12.5, fontWeight: 700, color: 'text.secondary', fontVariantNumeric: 'tabular-nums' }}>
          {t('tour.progress', { n: i + 1, total: STEPS.length })}
        </Box>
        <Button color="inherit" size="small" onClick={finish} sx={{ visibility: last ? 'hidden' : 'visible' }}>
          {t('tour.skip')}
        </Button>
      </Box>

      <Box onTouchStart={onTouchStart} onTouchEnd={onTouchEnd}
        sx={{ flex: 1, overflowY: 'auto', display: 'flex', flexDirection: 'column', justifyContent: 'center', px: 3, py: 2 }}>
        <Fade in key={i} timeout={250}>
          <Box sx={{ textAlign: 'center' }}>
            <Illustration name={step.art} size={phone ? 200 : 240} />
            <Box id="tour-title" component="h2" sx={{ fontSize: phone ? 22 : 21, fontWeight: 800, mt: 3, mb: 1 }}>
              {t(`${step.key}.title`)}
            </Box>
            <Box sx={{ color: 'text.secondary', fontSize: phone ? 15 : 14, lineHeight: 1.65, maxWidth: 440, mx: 'auto' }}>
              {t(`${step.key}.body`)}
            </Box>

            {step.body === 'api' && (
              <Box sx={{ mt: 3, mx: 'auto', maxWidth: 360, display: 'flex', flexDirection: 'column', gap: 1 }}>
                <TextField value={baseUrl} onChange={e => setBaseUrl(e.target.value)} label={t('settings.apiUrl')}
                  fullWidth inputProps={{ inputMode: 'url', autoCapitalize: 'off', autoCorrect: 'off', spellCheck: false }}
                  sx={{ '& input': { fontFamily: '"IBM Plex Mono", monospace', fontSize: 16 } }} />
                <Box role="status" sx={{
                  display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 0.75,
                  fontSize: 12.5, fontWeight: 700, color: statusColor,
                }}>
                  {/* The step's text promises a dot that turns green. */}
                  <Box component="span" sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: statusColor }} />
                  {t(`api.${status}`)}
                </Box>
              </Box>
            )}

            {step.body === 'scan' && (
              <Button variant="contained" size="large" fullWidth={phone} sx={{ mt: 3 }}
                onClick={() => { finish(); onNewScan() }}>
                {t('common.startScan')}
              </Button>
            )}
          </Box>
        </Fade>
      </Box>

      <Box sx={{ display: 'flex', justifyContent: 'center', gap: 0.5, py: 1 }}>
        {STEPS.map((s, n) => (
          // Each dot is a 24px target around an 8px mark, so it can be tapped.
          <ButtonBase key={s.key} onClick={() => go(n)} aria-current={n === i ? 'step' : undefined}
            aria-label={t('tour.goto', { n: n + 1, title: t(`${s.key}.title`) })}
            sx={{ width: 24, height: 24, borderRadius: '50%' }}>
            <Box sx={{
              height: 8, borderRadius: 4, transition: 'width .2s, background-color .2s',
              width: n === i ? 20 : 8, bgcolor: n === i ? 'primary.main' : 'action.disabled',
            }} />
          </ButtonBase>
        ))}
      </Box>

      <Box sx={{
        display: 'flex', gap: 1, px: phone ? 2 : 3,
        pb: phone ? 'max(16px, env(safe-area-inset-bottom))' : 2.5, pt: 1,
      }}>
        <Button variant="outlined" color="inherit" onClick={() => go(i - 1)} disabled={i === 0}
          startIcon={<ArrowBackIcon />} size={phone ? 'large' : 'medium'} sx={{ flex: phone ? 1 : 'none' }}>
          {t('common.back')}
        </Button>
        <Box sx={{ flex: phone ? 0 : 1 }} />
        {last
          ? <Button variant="contained" onClick={done} size={phone ? 'large' : 'medium'} sx={{ flex: phone ? 2 : 'none' }}>
              {t('tour.done')}
            </Button>
          : <Button variant="contained" onClick={() => go(i + 1)} endIcon={<ArrowForwardIcon />}
              size={phone ? 'large' : 'medium'} sx={{ flex: phone ? 2 : 'none' }} autoFocus>
              {t('common.next')}
            </Button>}
      </Box>
    </Dialog>
  )
}
