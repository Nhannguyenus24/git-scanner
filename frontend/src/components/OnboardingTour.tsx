import { useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import MobileStepper from '@mui/material/MobileStepper'
import TextField from '@mui/material/TextField'
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

export default function OnboardingTour({ open, onClose, onNewScan }: {
  open: boolean
  onClose: () => void
  onNewScan: () => void
}) {
  const { t } = useT()
  const { set } = usePrefs()
  const { baseUrl, setBaseUrl } = useSettings()
  const status = useApiStatus(baseUrl)
  const [i, setI] = useState(0)
  const step = STEPS[i]

  const finish = () => { set('onboarded', true); onClose() }

  return (
    <Dialog open={open} onClose={finish} fullWidth maxWidth="sm" aria-labelledby="tour-title">
      <DialogContent sx={{ pt: 4, textAlign: 'center' }}>
        <Illustration name={step.art} size={240} />
        <Box id="tour-title" component="h2" sx={{ fontSize: 21, fontWeight: 800, mt: 3, mb: 1 }}>{t(`${step.key}.title`)}</Box>
        <Box sx={{ color: 'text.secondary', fontSize: 14, lineHeight: 1.65, maxWidth: 440, mx: 'auto' }}>
          {t(`${step.key}.body`)}
        </Box>

        {step.body === 'api' && (
          <Box sx={{ mt: 3, display: 'flex', gap: 1, justifyContent: 'center', alignItems: 'center', flexWrap: 'wrap' }}>
            <TextField value={baseUrl} onChange={e => setBaseUrl(e.target.value)} label={t('settings.apiUrl')}
              sx={{ width: 280, '& input': { fontFamily: '"IBM Plex Mono", monospace', fontSize: 12.5 } }} />
            <Box sx={{
              fontSize: 12, fontWeight: 700,
              color: status === 'ok' ? 'success.main' : status === 'error' ? 'error.main' : 'text.secondary',
            }}>
              {t(`api.${status}`)}
            </Box>
          </Box>
        )}

        {step.body === 'scan' && (
          <Button variant="contained" sx={{ mt: 3 }} onClick={() => { finish(); onNewScan() }}>
            {t('common.startScan')}
          </Button>
        )}
      </DialogContent>

      <MobileStepper
        variant="dots" steps={STEPS.length} position="static" activeStep={i}
        sx={{ bgcolor: 'transparent', justifyContent: 'center' }}
        nextButton={<span />} backButton={<span />} />

      <DialogActions sx={{ px: 3, pb: 2, justifyContent: 'space-between' }}>
        <Button color="inherit" onClick={finish}>{t('tour.skip')}</Button>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button onClick={() => setI(n => n - 1)} disabled={i === 0}>{t('common.back')}</Button>
          {i < STEPS.length - 1
            ? <Button variant="contained" onClick={() => setI(n => n + 1)}>{t('common.next')}</Button>
            : <Button variant="contained" onClick={() => { finish(); navigate(href('overview')) }}>{t('tour.done')}</Button>}
        </Box>
      </DialogActions>
    </Dialog>
  )
}
