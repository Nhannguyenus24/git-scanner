import { useEffect, useMemo, useRef, useState } from 'react'
import { flushSync } from 'react-dom'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import IconButton from '@mui/material/IconButton'
import Link from '@mui/material/Link'
import Tooltip from '@mui/material/Tooltip'
import { useTheme } from '@mui/material/styles'
import PauseIcon from '@mui/icons-material/Pause'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import SkipNextIcon from '@mui/icons-material/SkipNext'
import SkipPreviousIcon from '@mui/icons-material/SkipPrevious'
import SeverityChip from '../components/SeverityChip'
import { PageHeader } from '../components/Layout'
import { Empty, LoadError, Loading } from '../components/States'
import { commitLink } from '../format'
import { useResource } from '../hooks/useResource'
import { href } from '../hooks/useRoute'
import { usePrefs } from '../store/preferences'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import type { Finding } from '../types'

// A repository's exposure, replayed commit by commit.
//
// What the data can honestly say: when each credential was committed, and
// whether it is still in the default branch today. It cannot say when a removed
// credential left HEAD — the scan reads history, it does not diff every commit
// for deletions — so a removed key is drawn as a line that fades out, never one
// that stops at an invented date. The point the view makes is the one the
// product exists for: everything committed stays clonable, removed or not.

const WINDOW = 500
const LANES = 12

interface Step {
  at:       number
  sha:      string
  author:   string
  findings: Finding[]
}

function steps(findings: Finding[]): Step[] {
  const by = new Map<string, Step>()
  for (const f of findings) {
    if (!f.committed_at) continue
    const k = `${f.committed_at}|${f.commit_sha}`
    if (!by.has(k)) by.set(k, { at: Date.parse(f.committed_at), sha: f.commit_sha, author: f.author_name, findings: [] })
    by.get(k)!.findings.push(f)
  }
  return [...by.values()].sort((a, b) => a.at - b.at)
}

function startTransition(update: () => void, enabled: boolean) {
  const doc = document as Document & { startViewTransition?: (cb: () => void) => unknown }
  if (enabled && doc.startViewTransition) doc.startViewTransition(() => flushSync(update))
  else update()
}

export default function RepoHistoryPage({ id }: { id: string }) {
  const { api } = useSettings()
  const { t, lang } = useT()
  const { motionOff } = usePrefs()
  const res = useResource(() => api.findings({ repository_id: id, limit: WINDOW }), [api, id])
  const repos = useResource(() => api.repositories(), [api])

  const all = res.data?.findings ?? []
  const timeline = useMemo(() => steps(all), [all])
  const unattributed = all.filter(f => !f.committed_at).length
  const [index, setIndex] = useState<number | null>(null)
  const [playing, setPlaying] = useState(false)
  const step = index ?? timeline.length // default: today, everything shown

  const go = (n: number) => startTransition(() => setIndex(Math.max(0, Math.min(timeline.length, n))), !motionOff)

  // Play walks forward one commit at a time, starting over when already at the end.
  const timer = useRef<number>()
  useEffect(() => {
    if (!playing) return
    if (step >= timeline.length) { setPlaying(false); return }
    timer.current = window.setTimeout(() => go(step + 1), motionOff ? 250 : 900)
    return () => window.clearTimeout(timer.current)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playing, step, timeline.length])

  if (res.loading && !res.data) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />

  const repo = repos.data?.repositories.find(r => r.id === id)
  const name = repo ? `${repo.owner}/${repo.name}` : all[0] ? `${all[0].repository.owner}/${all[0].repository.name}` : id
  const fmt = (ms: number) => new Date(ms).toLocaleDateString(lang, { year: 'numeric', month: 'short', day: 'numeric' })

  const shown = timeline.slice(0, step)
  const committed = shown.flatMap(s => s.findings)
  const atHead = committed.filter(f => f.reachable_from_head).length
  const current = step > 0 ? timeline[step - 1] : null

  return (
    <>
      <PageHeader title={t('history.title', { repo: name })} subtitle={t('history.subtitle')} actions={
        <Button href={href('findings', undefined, { repository_id: id })}>{t('history.toFindings')}</Button>
      } />

      {res.data && res.data.total > all.length && (
        <Alert severity="info" sx={{ mb: 2 }}>{t('history.window', { n: all.length, total: res.data.total })}</Alert>
      )}

      {!timeline.length ? (
        <Card><Empty art="safe" size={200} title={t('history.empty')} body={unattributed ? t('history.emptyUnattributed', { n: unattributed }) : t('history.emptyBody')} /></Card>
      ) : (
        <>
          <Card sx={{ p: { xs: 2, md: 2.5 }, mb: 2 }}>
            <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexWrap: 'wrap', mb: 1.5 }}>
              <Tooltip title={t('history.prev')}><span>
                <IconButton aria-label={t('history.prev')} disabled={step === 0} onClick={() => { setPlaying(false); go(step - 1) }}><SkipPreviousIcon /></IconButton>
              </span></Tooltip>
              <Tooltip title={t(playing ? 'history.pause' : 'history.play')}>
                <IconButton aria-label={t(playing ? 'history.pause' : 'history.play')} color="primary"
                  onClick={() => { if (!playing && step >= timeline.length) setIndex(0); setPlaying(p => !p) }}>
                  {playing ? <PauseIcon /> : <PlayArrowIcon />}
                </IconButton>
              </Tooltip>
              <Tooltip title={t('history.next')}><span>
                <IconButton aria-label={t('history.next')} disabled={step >= timeline.length} onClick={() => { setPlaying(false); go(step + 1) }}><SkipNextIcon /></IconButton>
              </span></Tooltip>
              <Box component="input" type="range" min={0} max={timeline.length} value={step}
                aria-label={t('history.scrub')} aria-valuetext={current ? fmt(current.at) : t('history.before')}
                onChange={(e: React.ChangeEvent<HTMLInputElement>) => { setPlaying(false); go(Number(e.target.value)) }}
                sx={{ flex: 1, minWidth: 180, accentColor: (th: any) => th.palette.primary.main }} />
              <Box sx={{ fontFamily: '"IBM Plex Mono", monospace', fontSize: 13, minWidth: 120, textAlign: 'right' }}>
                {current ? fmt(current.at) : t('history.before')}
              </Box>
            </Box>

            <Lanes timeline={timeline} step={step} fmt={fmt} />

            <Box aria-live="polite" sx={{ mt: 2, fontSize: 14, lineHeight: 1.6, viewTransitionName: 'history-summary' }}>
              {current ? (
                <>
                  <Box component="strong">{t('history.by', { date: fmt(current.at), n: committed.length })}</Box>{' '}
                  {t('history.today', { head: atHead, gone: committed.length - atHead })}{' '}
                  <Box component="span" sx={{ color: 'error.main', fontWeight: 600 }}>{t('history.allClonable', { n: committed.length })}</Box>
                </>
              ) : t('history.beforeBody', { n: timeline.length })}
            </Box>
          </Card>

          <Card>
            <Box sx={{ px: 2.5, py: 1.5, fontWeight: 700, borderBottom: 1, borderColor: 'divider' }}>{t('history.commits')}</Box>
            {!shown.length ? <Box sx={{ p: 2.5, color: 'text.secondary', fontSize: 13 }}>{t('history.noneYet')}</Box> : (
              <Box component="ol" reversed sx={{ listStyle: 'none', m: 0, p: 0 }}>
                {[...shown].reverse().map((s, i) => (
                  <CommitRow key={`${s.sha}-${s.at}`} s={s} fresh={i === 0 && index !== null} fmt={fmt} repoUrl={repo ? `https://github.com/${repo.owner}/${repo.name}` : all[0]?.repository.url ?? ''} />
                ))}
              </Box>
            )}
            {unattributed > 0 && (
              <Box sx={{ px: 2.5, py: 1.5, fontSize: 12.5, color: 'text.secondary', borderTop: 1, borderColor: 'divider' }}>
                {t('history.unattributed', { n: unattributed })}
              </Box>
            )}
          </Card>
        </>
      )}
    </>
  )
}

function CommitRow({ s, fresh, fmt, repoUrl }: { s: Step; fresh: boolean; fmt: (ms: number) => string; repoUrl: string }) {
  const { t } = useT()
  const link = commitLink(repoUrl, s.sha)
  return (
    <Box component="li" sx={{
      px: 2.5, py: 1.5, borderBottom: 1, borderColor: 'divider',
      bgcolor: fresh ? 'action.selected' : undefined, viewTransitionName: fresh ? 'history-fresh' : undefined,
    }}>
      <Box sx={{ display: 'flex', gap: 1.5, alignItems: 'baseline', flexWrap: 'wrap', fontSize: 13, mb: .75 }}>
        <Box sx={{ fontFamily: '"IBM Plex Mono", monospace' }}>
          {link ? <Link href={link} target="_blank" rel="noreferrer" underline="hover">{s.sha.slice(0, 8)}</Link> : s.sha.slice(0, 8)}
        </Box>
        <Box sx={{ color: 'text.secondary' }}>{fmt(s.at)} · {s.author || t('history.unknownAuthor')}</Box>
        <Box sx={{ color: 'text.secondary' }}>{t('history.introduced', { n: s.findings.length })}</Box>
      </Box>
      {s.findings.map(f => (
        <Box key={f.id} sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap', fontSize: 12.5, py: .25 }}>
          <SeverityChip severity={f.severity} />
          <Box sx={{ fontWeight: 600 }}>{f.provider || t('findings.unmapped')}</Box>
          <Link href={href('findings', undefined, { repository_id: f.repository.id, finding: f.id })} underline="hover"
            sx={{ fontFamily: '"IBM Plex Mono", monospace', wordBreak: 'break-all' }}>
            {f.file_path}:{f.line_number}
          </Link>
          <Box component="span" sx={{ color: f.reachable_from_head ? 'error.main' : 'text.secondary', fontSize: 11.5 }}>
            {f.reachable_from_head ? t('history.stillAtHead') : t('history.removed')}
          </Box>
        </Box>
      ))}
    </Box>
  )
}

// One lane per file (the busiest first), a mark where each credential was
// committed, and a line from there to today: solid while it is still at HEAD,
// fading out when it was removed at some point we do not know.
function Lanes({ timeline, step, fmt }: { timeline: Step[]; step: number; fmt: (ms: number) => string }) {
  const theme = useTheme()
  const { t } = useT()
  const ch = theme.chart
  const all = timeline.flatMap(s => s.findings.map(f => ({ f, at: s.at })))
  const counts = new Map<string, number>()
  for (const { f } of all) counts.set(f.file_path, (counts.get(f.file_path) ?? 0) + 1)
  const files = [...counts.entries()].sort((a, b) => b[1] - a[1]).map(([p]) => p)
  const lanes = files.slice(0, LANES)
  const other = files.length > LANES
  const laneOf = (p: string) => { const i = lanes.indexOf(p); return i >= 0 ? i : LANES }

  const t0 = timeline[0].at, now = Date.now()
  const span = Math.max(1, now - t0)
  const W = 1000, labelW = 230, rowH = 22, pad = 8
  const rows = lanes.length + (other ? 1 : 0)
  const H = rows * rowH + 26
  const x = (ms: number) => labelW + ((ms - t0) / span) * (W - labelW - pad)
  const cut = step > 0 ? timeline[step - 1].at : t0 - 1
  const gradId = `fade-${theme.palette.mode}`

  return (
    <Box sx={{ overflowX: 'auto' }}>
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" style={{ minWidth: 640, display: 'block' }} role="img"
        aria-label={t('history.lanesLabel', { n: all.length, files: files.length })}>
        <defs>
          <linearGradient id={gradId} x1="0" x2="1" y1="0" y2="0">
            <stop offset="0%" stopColor={theme.palette.text.secondary} stopOpacity={0.7} />
            <stop offset="100%" stopColor={theme.palette.text.secondary} stopOpacity={0} />
          </linearGradient>
        </defs>
        {[...lanes, ...(other ? [t('history.otherFiles', { n: files.length - LANES })] : [])].map((p, i) => (
          <g key={p}>
            <line x1={labelW} x2={W - pad} y1={i * rowH + rowH / 2} y2={i * rowH + rowH / 2} stroke={theme.palette.divider} />
            <text x={labelW - 8} y={i * rowH + rowH / 2 + 4} textAnchor="end" fontSize={11}
              fill={theme.palette.text.secondary} style={{ fontFamily: '"IBM Plex Mono", monospace' }}>
              {p.length > 34 ? '…' + p.slice(-33) : p}
            </text>
          </g>
        ))}
        {all.map(({ f, at }) => {
          const y = laneOf(f.file_path) * rowH + rowH / 2
          const on = at <= cut
          const c = ch.severity[f.severity] ?? ch.severity.medium
          const x1 = x(at), x2 = x(now)
          return (
            <g key={f.id} opacity={on ? 1 : 0.12}>
              {f.reachable_from_head
                ? <line x1={x1} x2={x2} y1={y} y2={y} stroke={c} strokeWidth={2.5} opacity={0.55} />
                : <rect x={x1} y={y - 1.25} width={Math.max(0, Math.min(140, x2 - x1))} height={2.5} fill={`url(#${gradId})`} />}
              <polygon points={`${x1},${y - 6} ${x1 + 6},${y} ${x1},${y + 6} ${x1 - 6},${y}`} fill={c}
                stroke={theme.palette.background.paper} strokeWidth={1.2}>
                <title>{`${f.provider || 'unmapped'} · ${f.file_path}:${f.line_number} · ${fmt(at)}`}</title>
              </polygon>
            </g>
          )
        })}
        {step > 0 && (
          <line x1={x(cut)} x2={x(cut)} y1={0} y2={rows * rowH} stroke={theme.palette.primary.main} strokeWidth={1.5} strokeDasharray="4 3" />
        )}
        <text x={labelW} y={H - 6} fontSize={11} fill={theme.palette.text.secondary}>{fmt(t0)}</text>
        <text x={W - pad} y={H - 6} fontSize={11} textAnchor="end" fill={theme.palette.text.secondary}>{t('history.now')}</text>
      </svg>
    </Box>
  )
}
