import { useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import CircularProgress from '@mui/material/CircularProgress'
import Typography from '@mui/material/Typography'
import CheckIcon from '@mui/icons-material/CheckCircle'
import ErrorIcon from '@mui/icons-material/Cancel'
import { useTheme } from '@mui/material/styles'
import { useT } from '../i18n'
import { describe, inFlight, recentlyFinished, REPO_STEPS, type LiveState, type RepoLive } from '../live'
import type { StreamStatus } from '../hooks/useScanEvents'
import type { ProgressStage } from '../types'

const MONO = '"IBM Plex Mono", monospace'
const LOG_SHORT = 12

const clock = (iso: string) => {
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? '' : d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

/** What a repository's current step is doing, in words. */
function stepLine(r: RepoLive, t: ReturnType<typeof useT>['t']): string {
  return t(`live.step.${r.stage}`, {
    blobs: r.blobs.toLocaleString(), matches: r.matches, findings: r.findings, live: r.live,
  })
}

/** The clone → walk → … strip for one repository. Probe is shown only once a
 *  repository has reached it, since it runs only when probing is enabled. */
function Steps({ r }: { r: RepoLive }) {
  const { t } = useT()
  const ch = useTheme().chart
  const steps = REPO_STEPS.filter(s => s !== 'probe' || r.stage === 'probe' || r.done.includes('probe'))
  return (
    <Box sx={{ display: 'flex', gap: .5, flexWrap: 'wrap', mt: .5 }}>
      {steps.map((s: ProgressStage) => {
        const done = r.done.includes(s)
        const current = r.stage === s
        const failed = current && r.status === 'failed'
        return (
          <Box key={s} sx={{
            display: 'inline-flex', alignItems: 'center', gap: .5,
            fontSize: 10.5, fontFamily: MONO, px: .6, py: .1, borderRadius: .5, border: 1,
            borderColor: failed ? 'error.main' : current ? 'primary.main' : 'divider',
            color: failed ? 'error.main' : current ? 'primary.main' : done ? 'text.primary' : 'text.disabled',
            fontWeight: current ? 700 : 400,
          }}>
            {done && <CheckIcon sx={{ fontSize: 11, color: ch.categorical[0] }} />}
            {current && !failed && <CircularProgress size={9} thickness={7} />}
            {failed && <ErrorIcon sx={{ fontSize: 11 }} />}
            {t(`live.stepName.${s}`)}
          </Box>
        )
      })}
    </Box>
  )
}

function StatusDot({ status }: { status: StreamStatus }) {
  const { t } = useT()
  const ch = useTheme().chart
  const color = status === 'live' ? ch.categorical[0]
    : status === 'reconnecting' || status === 'connecting' ? ch.severity.medium
    : ch.textFaint
  return (
    <Box component="span" role="status" sx={{ display: 'inline-flex', alignItems: 'center', gap: .75, fontSize: 12, color: 'text.secondary' }}>
      <Box component="span" sx={{
        width: 8, height: 8, borderRadius: '50%', bgcolor: color,
        animation: status === 'live' ? 'gs-pulse 1.6s ease-in-out infinite' : 'none',
        '@keyframes gs-pulse': { '0%, 100%': { opacity: 1 }, '50%': { opacity: .35 } },
      }} />
      {t(`live.status.${status}`)}
    </Box>
  )
}

/**
 * The narration of a running scan: which step each repository is in, what just
 * finished, and a log of what happened. It sits beside the counters and never
 * replaces them — they are what the job row says; this is what the workers are
 * saying about it.
 */
export default function LiveActivity({ live, status }: { live: LiveState; status: StreamStatus }) {
  const { t } = useT()
  const [allLog, setAllLog] = useState(false)
  const working = inFlight(live)
  const recent = recentlyFinished(live)
  const log = allLog ? live.log : live.log.slice(0, LOG_SHORT)
  const quietYet = !working.length && !recent.length && !live.discover && !live.log.length

  return (
    <Card sx={{ p: 2, mb: 2 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1, mb: 1.5, flexWrap: 'wrap' }}>
        <Typography sx={{ fontWeight: 700 }}>{t('live.title')}</Typography>
        <StatusDot status={status} />
      </Box>

      {status === 'unavailable' && (
        <Box sx={{ fontSize: 13, color: 'text.secondary' }}>{t('live.unavailable')}</Box>
      )}
      {status !== 'unavailable' && quietYet && (
        <Box sx={{ fontSize: 13, color: 'text.secondary' }}>{t('live.waiting')}</Box>
      )}

      {live.discover && (
        <Box sx={{ fontSize: 13, mb: 1.5 }}>
          <Box component="span" sx={{ fontWeight: 600 }}>{t('live.discovery')}</Box>{' · '}
          {t(`live.discover.${live.discover.status}`, { repos: live.discover.repos, error: live.discover.error ?? '' })}
        </Box>
      )}

      {working.length > 0 && (
        <Box sx={{ mb: 1.5 }}>
          <Box sx={{ fontSize: 11, fontWeight: 700, letterSpacing: '.08em', textTransform: 'uppercase', color: 'text.secondary', mb: .75 }}>
            {t('live.inFlight', { n: working.length })}
          </Box>
          <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.25 }}>
            {working.map(r => (
              <Box key={r.id} sx={{ minWidth: 0 }}>
                <Box sx={{ display: 'flex', gap: 1, alignItems: 'baseline', flexWrap: 'wrap' }}>
                  <Box sx={{ fontFamily: MONO, fontSize: 13, fontWeight: 600 }}>{r.repo}</Box>
                  {r.attempt > 1 && <Box sx={{ fontSize: 11.5, color: 'warning.main' }}>{t('live.attempt', { n: r.attempt })}</Box>}
                </Box>
                <Steps r={r} />
                <Box sx={{ fontSize: 12.5, color: 'text.secondary', mt: .4 }}>{stepLine(r, t)}</Box>
                {r.stage === 'walk' && r.path && (
                  <Box sx={{ fontFamily: MONO, fontSize: 11.5, color: 'text.disabled', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {r.path}
                  </Box>
                )}
              </Box>
            ))}
          </Box>
        </Box>
      )}

      {recent.length > 0 && (
        <Box sx={{ mb: 1.5 }}>
          <Box sx={{ fontSize: 11, fontWeight: 700, letterSpacing: '.08em', textTransform: 'uppercase', color: 'text.secondary', mb: .75 }}>
            {t('live.finished')}
          </Box>
          {recent.map(r => (
            <Box key={r.id} sx={{ display: 'flex', gap: 1, fontSize: 12.5, py: .25, alignItems: 'baseline', flexWrap: 'wrap' }}>
              <Box sx={{ fontFamily: MONO, fontWeight: 600 }}>{r.repo}</Box>
              <Box sx={{ color: r.status === 'done' ? 'text.secondary' : 'error.main', minWidth: 0, wordBreak: 'break-word' }}>
                {t(`live.result.${r.status}`, {
                  findings: r.findings, blobs: r.blobs.toLocaleString(), secs: ((r.elapsedMs ?? 0) / 1000).toFixed(1),
                  attempt: r.attempt, error: r.error ?? '', app: r.appType || '—',
                })}
              </Box>
            </Box>
          ))}
        </Box>
      )}

      {live.log.length > 0 && (
        <Box>
          <Box sx={{ fontSize: 11, fontWeight: 700, letterSpacing: '.08em', textTransform: 'uppercase', color: 'text.secondary', mb: .75 }}>
            {t('live.log')}
          </Box>
          <Box component="ol" aria-label={t('live.log')} sx={{ listStyle: 'none', m: 0, p: 0, fontSize: 12.5 }}>
            {log.map(ev => (
              <Box component="li" key={ev.seq} sx={{ display: 'flex', gap: 1.25, py: .2 }}>
                <Box sx={{ fontFamily: MONO, color: 'text.disabled', flexShrink: 0 }}>{clock(ev.at)}</Box>
                <Box sx={{ color: ev.status === 'failed' || ev.status === 'skipped' ? 'error.main' : 'text.primary', minWidth: 0, wordBreak: 'break-word' }}>
                  {describe(ev, t, ev.repo_id ? live.repos[ev.repo_id]?.repo : undefined)}
                </Box>
              </Box>
            ))}
          </Box>
          {live.log.length > LOG_SHORT && (
            <Button size="small" sx={{ mt: .5 }} onClick={() => setAllLog(v => !v)}>
              {allLog ? t('live.less') : t('live.more', { n: live.log.length - LOG_SHORT })}
            </Button>
          )}
        </Box>
      )}
    </Card>
  )
}
