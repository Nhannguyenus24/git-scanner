import { useEffect, useRef, useState } from 'react'
import Box from '@mui/material/Box'
import Tooltip from '@mui/material/Tooltip'
import { useTheme } from '@mui/material/styles'
import { REPO_STEPS, type LiveState, type RepoLive } from '../live'
import { usePrefs } from '../store/preferences'
import { useT } from '../i18n'

// The scan pipeline as a picture: one dot per repository, sitting in the step
// the workers last reported for it, and moving to the next step when they
// report again. Finished repositories drop into the bin on the right.
//
// It draws exactly the live state the activity panel below lists — nothing is
// interpolated between events, so a dot moves only when a worker said it did.
// Positions are CSS transforms with a transition; with reduced motion the
// transition is off and the dots simply appear where they are.

const STEP_W = 120, DOT = 10, GAP = 4, ROWS = 8, BIN_COLS = 10, BIN_MAX = 120

type Slot = { x: number; y: number }

function column(r: RepoLive): number | 'bin' {
  if (r.stage === 'repo') return r.status === 'start' ? 0 : 'bin'
  if (r.stage === 'queue') return 0
  const i = REPO_STEPS.indexOf(r.stage as typeof REPO_STEPS[number])
  return i < 0 ? 0 : i
}

export default function PipelineFlow({ live }: { live: LiveState }) {
  const { t } = useT()
  const theme = useTheme()
  const ch = theme.chart
  const { motionOff } = usePrefs()
  const repos = Object.values(live.repos)

  // Bin order is the order repositories finished in, kept across renders so a
  // dot never jumps to a different bin cell when another one lands.
  const order = useRef<string[]>([])
  const [, force] = useState(0)
  useEffect(() => {
    const binned = repos.filter(r => column(r) === 'bin').map(r => r.id)
    const known = new Set(order.current)
    const fresh = binned.filter(id => !known.has(id))
    const still = new Set(binned)
    const next = order.current.filter(id => still.has(id)).concat(fresh)
    if (next.length !== order.current.length || fresh.length) { order.current = next; force(n => n + 1) }
  })

  const cols: RepoLive[][] = REPO_STEPS.map(() => [])
  for (const r of repos) { const c = column(r); if (c !== 'bin') cols[c].push(r) }
  for (const c of cols) c.sort((a, b) => a.at.localeCompare(b.at))

  const binX = REPO_STEPS.length * STEP_W + 24
  const slot = new Map<string, Slot>()
  cols.forEach((c, i) => c.slice(0, ROWS * 2).forEach((r, k) => {
    slot.set(r.id, { x: i * STEP_W + 24 + Math.floor(k / ROWS) * (DOT + GAP), y: 34 + (k % ROWS) * (DOT + GAP) })
  }))
  const binned = order.current.slice(-BIN_MAX)
  binned.forEach((id, k) => {
    slot.set(id, { x: binX + (k % BIN_COLS) * (DOT + GAP), y: 34 + Math.floor(k / BIN_COLS) * (DOT + GAP) })
  })

  const height = 34 + Math.max(ROWS, Math.ceil(binned.length / BIN_COLS)) * (DOT + GAP) + 6
  const width = binX + BIN_COLS * (DOT + GAP)
  const done = repos.filter(r => column(r) === 'bin')
  const found = done.reduce((n, r) => n + r.findings, 0)

  const colour = (r: RepoLive) =>
    r.status === 'failed' ? ch.severity.critical
    : r.status === 'skipped' ? theme.palette.text.disabled
    : r.findings > 0 || r.matches > 0 ? ch.severity.high
    : theme.palette.primary.main

  return (
    <Box sx={{ overflowX: 'auto' }}>
      <Box role="img" aria-label={t('flow.label', { active: repos.length - done.length, done: done.length, found })}
        sx={{ position: 'relative', width, height, fontSize: 11 }}>
        {REPO_STEPS.map((s, i) => (
          <Box key={s} sx={{
            position: 'absolute', left: i * STEP_W + 12, top: 0, width: STEP_W - 12, height: height - 4,
            borderLeft: 1, borderColor: 'divider', pl: 1.5,
          }}>
            <Box sx={{ fontFamily: '"IBM Plex Mono", monospace', fontWeight: 700, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '.06em', fontSize: 10.5 }}>
              {t(`live.stepName.${s}`)}
            </Box>
            <Box sx={{ color: 'text.secondary', fontSize: 10.5 }}>
              {cols[i].length > ROWS * 2 ? t('flow.more', { n: cols[i].length }) : cols[i].length || ''}
            </Box>
          </Box>
        ))}
        <Box sx={{ position: 'absolute', left: binX - 12, top: 0, pl: 1.5, borderLeft: 2, borderColor: 'divider', height: height - 4 }}>
          <Box sx={{ fontFamily: '"IBM Plex Mono", monospace', fontWeight: 700, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '.06em', fontSize: 10.5 }}>
            {t('flow.done')}
          </Box>
          <Box sx={{ color: 'text.secondary', fontSize: 10.5 }}>{done.length ? t('flow.doneCount', { n: done.length, f: found }) : ''}</Box>
        </Box>

        {repos.map(r => {
          const p = slot.get(r.id)
          if (!p) return null
          const c = colour(r)
          const moving = column(r) !== 'bin'
          return (
            <Tooltip key={r.id} title={`${r.repo} · ${!moving ? t(`flow.status.${r.status}`)
              : (REPO_STEPS as string[]).includes(r.stage) ? t(`live.stepName.${r.stage}`) : t('live.step.queue')}`}>
              <Box sx={{
                position: 'absolute', left: 0, top: 0, width: DOT, height: DOT, borderRadius: '50%',
                transform: `translate(${p.x}px, ${p.y}px)`,
                transition: motionOff ? 'none' : 'transform .7s cubic-bezier(.3,.7,.2,1), background-color .3s',
                bgcolor: c, boxShadow: moving && !motionOff ? `0 0 0 3px ${c}33` : 'none',
                animation: moving && !motionOff ? 'flowPulse 1.6s ease-in-out infinite' : 'none',
                '@keyframes flowPulse': { '50%': { boxShadow: `0 0 0 5px ${c}22` } },
              }} />
            </Tooltip>
          )
        })}
      </Box>
    </Box>
  )
}
