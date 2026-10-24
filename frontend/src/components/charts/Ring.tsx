import { useState } from 'react'
import Box from '@mui/material/Box'
import { ink } from '../../theme'
import type { Count } from '../../types'

export interface Slice { key: string; label: string; value: number; color: string }

// A ring (donut) for a small part-to-whole breakdown, with a centre total and a
// direct-labelled legend — identity never by colour alone. A 2px paper gap
// between arcs keeps adjacent slices legible.
export default function Ring({
  slices, total, centerLabel, size = 150,
}: {
  slices: Slice[]
  total?: number
  centerLabel?: string
  size?: number
}) {
  const [hi, setHi] = useState<string | null>(null)
  const sum = total ?? slices.reduce((a, s) => a + s.value, 0)
  if (sum === 0) return <Box sx={{ py: 4, textAlign: 'center', color: 'text.secondary', fontSize: 13 }}>Nothing yet.</Box>

  const r = size / 2 - 10, cx = size / 2, cy = size / 2, sw = 16
  const gap = 0.012 // radians of paper between arcs
  let a = -Math.PI / 2
  const arcs = slices.filter(s => s.value > 0).map(s => {
    const frac = s.value / sum
    const start = a + gap, end = a + frac * 2 * Math.PI - gap
    a += frac * 2 * Math.PI
    return { ...s, d: arcPath(cx, cy, r, start, Math.max(end, start + 0.001)) }
  })

  return (
    <Box sx={{ display: 'flex', gap: 2, alignItems: 'center', flexWrap: 'wrap' }}>
      <svg width={size} height={size} viewBox={`0 0 ${size} ${size}`} style={{ flexShrink: 0 }}
        onMouseLeave={() => setHi(null)}>
        <circle cx={cx} cy={cy} r={r} fill="none" stroke={ink.line} strokeWidth={sw} />
        {arcs.map(s => (
          <path key={s.key} d={s.d} fill="none" stroke={s.color}
            strokeWidth={hi === s.key ? sw + 3 : sw} strokeLinecap="butt"
            opacity={hi && hi !== s.key ? 0.4 : 1}
            onMouseEnter={() => setHi(s.key)} style={{ cursor: 'default', transition: 'opacity .12s' }} />
        ))}
        <text x={cx} y={cy - 2} textAnchor="middle" fontFamily="IBM Plex Mono, monospace" fontSize="22" fontWeight="700" fill={ink.text}>
          {hi ? slices.find(s => s.key === hi)!.value : sum}
        </text>
        <text x={cx} y={cy + 14} textAnchor="middle" fontFamily="IBM Plex Mono, monospace" fontSize="9" fill={ink.textFaint} letterSpacing="1">
          {(hi ? slices.find(s => s.key === hi)!.label : centerLabel ?? 'total').toUpperCase()}
        </text>
      </svg>

      <Box component="ul" sx={{ listStyle: 'none', m: 0, p: 0, display: 'flex', flexDirection: 'column', gap: .5, minWidth: 130 }}>
        {slices.filter(s => s.value > 0).map(s => (
          <Box component="li" key={s.key} onMouseEnter={() => setHi(s.key)} onMouseLeave={() => setHi(null)}
            sx={{ display: 'flex', alignItems: 'center', gap: 1, fontSize: 12.5, cursor: 'default',
              opacity: hi && hi !== s.key ? 0.5 : 1 }}>
            <Box sx={{ width: 10, height: 10, bgcolor: s.color, border: `1px solid ${ink.lineStrong}`, flexShrink: 0 }} />
            <Box sx={{ flex: 1 }}>{s.label}</Box>
            <Box sx={{ fontFamily: '"IBM Plex Mono", monospace', color: 'text.secondary' }}>{s.value}</Box>
          </Box>
        ))}
      </Box>
    </Box>
  )
}

// countsToSlices maps API counts to coloured slices via a key→colour map.
export function countsToSlices(counts: Count[], color: (key: string) => string, label?: (key: string) => string): Slice[] {
  return counts.map(c => ({ key: c.key, label: label ? label(c.key) : c.key, value: c.count, color: color(c.key) }))
}

function arcPath(cx: number, cy: number, r: number, start: number, end: number): string {
  const x1 = cx + r * Math.cos(start), y1 = cy + r * Math.sin(start)
  const x2 = cx + r * Math.cos(end), y2 = cy + r * Math.sin(end)
  const large = end - start > Math.PI ? 1 : 0
  return `M ${x1} ${y1} A ${r} ${r} 0 ${large} 1 ${x2} ${y2}`
}
