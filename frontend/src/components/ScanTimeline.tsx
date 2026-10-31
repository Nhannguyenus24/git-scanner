import Box from '@mui/material/Box'
import CircularProgress from '@mui/material/CircularProgress'
import Tooltip from '@mui/material/Tooltip'
import CheckIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Cancel'
import PendingIcon from '@mui/icons-material/RadioButtonUnchecked'
import { useTheme } from '@mui/material/styles'
import { useT } from '../i18n'
import { REPO_STEPS } from '../live'
import { TERMINAL, type ProgressStage, type ScanSummary } from '../types'

// What stage a scan is in, read off the counters the API already returns.
//
// The API reports repos_total, repos_scanned, blobs_scanned and a state. It does
// not report which phase a given repository is in, so clone, history walk and
// detection are shown as one stage with three named steps rather than as three
// stages with invented boundaries — a progress bar that claims to know something
// it does not is how people stop trusting the whole page. The footnote says so.

export type StageState = 'done' | 'active' | 'pending' | 'failed' | 'skipped'

export interface Stage {
  key:     string
  state:   StageState
  detail?: string
  steps?:  string[]
}

/** Derives the stage list; exported so its behaviour can be checked directly. */
export function stagesFor(s: ScanSummary, t: (k: string, v?: Record<string, string | number>) => string): Stage[] {
  const failed = s.state === 'failed'
  const cancelled = s.state === 'cancelled'
  const done = s.state === 'completed'
  const ended = TERMINAL.has(s.state)
  const discovered = s.repos_total > 0
  const walking = discovered && s.repos_scanned < s.repos_total
  const walked = discovered && s.repos_scanned >= s.repos_total

  const after = (reached: boolean, active: boolean): StageState =>
    failed && active ? 'failed'
    : cancelled && !reached ? 'skipped'
    : reached ? 'done'
    : active ? 'active'
    : 'pending'

  return [
    {
      key: 'stage.discover',
      state: s.state === 'queued' ? 'active' : after(discovered, !discovered && !ended),
      detail: discovered ? t('stage.discoverDone', { n: s.repos_total }) : t('stage.discoverWait'),
    },
    {
      key: 'stage.scan',
      state: after(walked, walking),
      detail: discovered
        ? t('stage.scanDetail', { a: s.repos_scanned, b: s.repos_total, blobs: s.blobs_scanned.toLocaleString() })
        : t('stage.scanWait'),
      steps: ['stage.step.clone', 'stage.step.walk', 'stage.step.detect'],
    },
    {
      key: 'stage.classify',
      state: after(walked && ended, walked && !ended),
      detail: t('stage.classifyDetail', { n: s.findings_count }),
    },
    {
      key: 'stage.finalize',
      state: ended ? (failed ? 'failed' : cancelled ? 'skipped' : 'done') : 'pending',
      detail: t('stage.finalizeDetail'),
    },
    {
      key: done ? 'stage.completed' : failed ? 'stage.failed' : cancelled ? 'stage.cancelled' : 'stage.end',
      state: done ? 'done' : failed ? 'failed' : cancelled ? 'skipped' : 'pending',
      detail: s.error || undefined,
    },
  ]
}

/**
 * Remaining time, but only when the estimate is worth printing: the job must be
 * running, the repository count must be known, and at least three repositories
 * must have finished, so the rate is not extrapolated from a single sample.
 * Returns undefined otherwise, and the caller shows counters instead.
 */
export function etaSeconds(s: ScanSummary): number | undefined {
  if (TERMINAL.has(s.state) || s.state === 'paused' || !s.started_at) return undefined
  if (s.repos_total <= 0 || s.repos_scanned < 3 || s.repos_scanned >= s.repos_total) return undefined
  const elapsed = (Date.now() - new Date(s.started_at).getTime()) / 1000
  if (elapsed < 10) return undefined
  const perRepo = elapsed / s.repos_scanned
  return Math.round(perRepo * (s.repos_total - s.repos_scanned))
}

/**
 * `steps`, when the live stream is connected, is how many repositories are in
 * each step right now; the scan stage then shows those instead of the generic
 * step names, because for once the boundaries are known rather than guessed.
 */
export default function ScanTimeline({ scan, steps }: { scan: ScanSummary; steps?: Partial<Record<ProgressStage, number>> }) {
  const { t } = useT()
  const ch = useTheme().chart
  const stages = stagesFor(scan, t)

  const mark = (st: StageState) => {
    if (st === 'done') return <CheckIcon sx={{ fontSize: 18, color: ch.categorical[0] }} />
    if (st === 'failed') return <ErrorIcon sx={{ fontSize: 18, color: ch.severity.critical }} />
    if (st === 'active') return <CircularProgress size={14} thickness={6} />
    return <PendingIcon sx={{ fontSize: 18, color: ch.textFaint, opacity: st === 'skipped' ? .4 : 1 }} />
  }

  return (
    <Box>
      <Box component="ol" sx={{ listStyle: 'none', m: 0, p: 0 }}>
        {stages.map((s, i) => (
          <Box component="li" key={s.key} sx={{ display: 'flex', gap: 1.5, position: 'relative', pb: i === stages.length - 1 ? 0 : 1.5 }}>
            {/* The rule joining the markers; it stops at the last one. */}
            {i < stages.length - 1 && (
              <Box sx={{
                position: 'absolute', left: 8.5, top: 20, bottom: 0, width: '1px',
                bgcolor: s.state === 'done' ? ch.categorical[0] : 'divider',
                opacity: s.state === 'done' ? .5 : 1,
              }} />
            )}
            <Box sx={{ width: 18, display: 'flex', justifyContent: 'center', flexShrink: 0, zIndex: 1, bgcolor: 'background.paper' }}>
              {mark(s.state)}
            </Box>
            <Box sx={{ minWidth: 0, flex: 1, opacity: s.state === 'pending' || s.state === 'skipped' ? .55 : 1 }}>
              <Box sx={{
                fontSize: 13.5, fontWeight: s.state === 'active' ? 700 : 600,
                color: s.state === 'failed' ? 'error.main' : 'text.primary',
              }}>
                {t(s.key)}
              </Box>
              {s.detail && <Box sx={{ fontSize: 12, color: 'text.secondary', mt: .15 }}>{s.detail}</Box>}
              {s.steps && steps && s.state === 'active' && (
                <Box sx={{ display: 'flex', gap: .75, mt: .5, flexWrap: 'wrap' }} aria-label={t('live.stepsNow')}>
                  {REPO_STEPS.filter(k => k !== 'probe' || steps.probe).map(k => (
                    <Box key={k} sx={{
                      fontSize: 10.5, fontFamily: '"IBM Plex Mono", monospace', px: .6, py: .1,
                      border: 1, borderRadius: .5,
                      borderColor: steps[k] ? 'primary.main' : 'divider',
                      color: steps[k] ? 'primary.main' : 'text.secondary',
                      fontWeight: steps[k] ? 700 : 400,
                    }}>{t(`live.stepName.${k}`)}{steps[k] ? ` · ${steps[k]}` : ''}</Box>
                  ))}
                </Box>
              )}
              {s.steps && !(steps && s.state === 'active') && s.state !== 'pending' && (
                <Box sx={{ display: 'flex', gap: .75, mt: .5, flexWrap: 'wrap' }}>
                  {s.steps.map(k => (
                    <Box key={k} sx={{
                      fontSize: 10.5, fontFamily: '"IBM Plex Mono", monospace', px: .6, py: .1,
                      border: 1, borderColor: 'divider', borderRadius: .5, color: 'text.secondary',
                    }}>{t(k)}</Box>
                  ))}
                </Box>
              )}
            </Box>
          </Box>
        ))}
      </Box>
      <Tooltip title={t('stage.noteTip')}>
        <Box sx={{ fontSize: 11, color: 'text.disabled', mt: 2, cursor: 'help' }}>{t(steps ? 'stage.noteLive' : 'stage.note')}</Box>
      </Tooltip>
    </Box>
  )
}
