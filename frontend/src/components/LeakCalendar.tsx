import { useMemo, useState } from 'react'
import Box from '@mui/material/Box'
import Chip from '@mui/material/Chip'
import { useTheme } from '@mui/material/styles'
import Figure from './charts/Figure'
import { useResource } from '../hooks/useResource'
import { useSettings } from '../store/settings'
import { useT } from '../i18n'
import type { LeakDay } from '../types'

// When credentials were committed, one square per day, a year at a time.
//
// The days are UTC: committed_at is stored as an instant and the author's own
// offset is gone, so a day here can be the author's previous or next day. The
// caption says so rather than letting the grid pass for local habit. Shade is
// by quantile of the year's non-empty days, not by raw count — one day with a
// vendored directory of 400 keys would otherwise wash every other day out.
// A day with a critical finding is outlined in the critical colour.

const CELL = 11, GAP = 3

const iso = (d: Date) => d.toISOString().slice(0, 10)

export default function LeakCalendar() {
  const { api } = useSettings()
  const { t, lang } = useT()
  const theme = useTheme()
  const ch = theme.chart
  const res = useResource(() => api.leakCalendar(), [api])
  const days = res.data?.days ?? []

  const years = useMemo(() => [...new Set(days.map(d => Number(d.day.slice(0, 4))))].sort((a, b) => b - a), [days])
  const [picked, setPicked] = useState<number | null>(null)
  const year = picked ?? years[0] ?? new Date().getUTCFullYear()

  const byDay = useMemo(() => new Map(days.filter(d => d.day.startsWith(`${year}-`)).map(d => [d.day, d])), [days, year])
  const ys = [...byDay.values()]
  const total = ys.reduce((n, d) => n + d.findings, 0)
  const busiest = ys.reduce<LeakDay | null>((m, d) => (!m || d.findings > m.findings ? d : m), null)

  // Four shades, cut at the quartiles of this year's non-empty days.
  const cuts = useMemo(() => {
    const v = ys.map(d => d.findings).sort((a, b) => a - b)
    const q = (p: number) => v[Math.min(v.length - 1, Math.floor(p * v.length))] ?? 0
    return [q(0.25), q(0.5), q(0.75)]
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [byDay])
  const level = (n: number) => (n <= 0 ? 0 : n <= cuts[0] ? 1 : n <= cuts[1] ? 2 : n <= cuts[2] ? 3 : 4)
  const shade = (lvl: number) => (lvl === 0 ? theme.palette.action.hover : `color-mix(in srgb, ${theme.palette.primary.main} ${[0, 30, 55, 78, 100][lvl]}%, transparent)`)

  // Weeks start on Monday; the first column holds the days before Jan 1 as blanks.
  const start = new Date(Date.UTC(year, 0, 1))
  const offset = (start.getUTCDay() + 6) % 7
  const cells: { day: string; col: number; row: number; d?: LeakDay }[] = []
  for (let d = new Date(start); d.getUTCFullYear() === year; d.setUTCDate(d.getUTCDate() + 1)) {
    const i = offset + Math.round((d.getTime() - start.getTime()) / 86400000)
    const key = iso(d)
    cells.push({ day: key, col: Math.floor(i / 7), row: i % 7, d: byDay.get(key) })
  }
  const cols = Math.max(...cells.map(c => c.col)) + 1
  const today = iso(new Date())
  const left = 26, top = 16
  const W = left + cols * (CELL + GAP), H = top + 7 * (CELL + GAP)
  const fmt = (day: string) => new Date(`${day}T00:00:00Z`).toLocaleDateString(lang, { timeZone: 'UTC', month: 'short', day: 'numeric', year: 'numeric' })
  const month = (m: number) => new Date(Date.UTC(year, m, 1)).toLocaleDateString(lang, { timeZone: 'UTC', month: 'short' })
  const weekday = (r: number) => new Date(Date.UTC(2024, 0, 1 + r)).toLocaleDateString(lang, { timeZone: 'UTC', weekday: 'short' })

  return (
    <Figure title={t('calendar.title')} caption={t('calendar.caption')}
      right={years.length > 1 ? (
        <Box sx={{ display: 'flex', gap: .5, flexWrap: 'wrap', justifyContent: 'flex-end' }}>
          {years.slice(0, 8).map(y => (
            <Chip key={y} size="small" label={y} clickable color={y === year ? 'primary' : 'default'}
              variant={y === year ? 'filled' : 'outlined'} onClick={() => setPicked(y)} aria-pressed={y === year} />
          ))}
        </Box>
      ) : undefined}>
      {!days.length ? (
        <Box sx={{ color: 'text.secondary', fontSize: 13 }}>{t('calendar.empty')}</Box>
      ) : (
        <>
          <Box sx={{ overflowX: 'auto' }}>
            <svg width={W} height={H} role="img" style={{ display: 'block' }}
              aria-label={t('calendar.label', { year, n: total, days: ys.length })}>
              {Array.from({ length: 12 }, (_, m) => {
                const first = cells.find(c => c.day === iso(new Date(Date.UTC(year, m, 1))))
                return first ? <text key={m} x={left + first.col * (CELL + GAP)} y={10} fontSize={10} fill={theme.palette.text.secondary}>{month(m)}</text> : null
              })}
              {[0, 2, 4].map(r => (
                <text key={r} x={0} y={top + r * (CELL + GAP) + CELL - 1} fontSize={9.5} fill={theme.palette.text.secondary}>{weekday(r)}</text>
              ))}
              {cells.map(c => {
                // A day that has not happened yet is not a day without leaks.
                if (c.day > today) return null
                const n = c.d?.findings ?? 0
                const crit = (c.d?.critical ?? 0) > 0
                return (
                  <rect key={c.day} x={left + c.col * (CELL + GAP)} y={top + c.row * (CELL + GAP)} width={CELL} height={CELL} rx={2}
                    fill={shade(level(n))} stroke={crit ? ch.severity.critical : 'none'} strokeWidth={crit ? 1.5 : 0}>
                    <title>{n ? t('calendar.cell', { date: fmt(c.day), n, c: c.d!.critical }) : fmt(c.day)}</title>
                  </rect>
                )
              })}
            </svg>
          </Box>
          <Box sx={{ display: 'flex', gap: 3, flexWrap: 'wrap', alignItems: 'center', mt: 1.5, fontSize: 12.5, color: 'text.secondary' }}>
            <span>{t('calendar.summary', { n: total, days: ys.length, year })}</span>
            {busiest && <span>{t('calendar.busiest', { date: fmt(busiest.day), n: busiest.findings })}</span>}
            <Box sx={{ display: 'inline-flex', alignItems: 'center', gap: .5, ml: 'auto' }}>
              {t('calendar.less')}
              {[0, 1, 2, 3, 4].map(l => <Box key={l} sx={{ width: CELL, height: CELL, borderRadius: '2px', bgcolor: shade(l) }} />)}
              {t('calendar.more')}
              <Box sx={{ width: CELL, height: CELL, borderRadius: '2px', border: `1.5px solid ${ch.severity.critical}`, ml: 1.5 }} />
              {t('calendar.critical')}
            </Box>
          </Box>
        </>
      )}
    </Figure>
  )
}
