import { useState } from 'react'
import Box from '@mui/material/Box'
import { ink } from '../../theme'
import { Hatch } from './Figure'

export interface Column { label: string; value: number; sub?: string }

// A column plot over an ordered axis — built for the leak-introduction timeline
// (findings per month). One series, so no legend: the figure title names it.
// Hairline baseline and gridlines, monospace ticks, a hatch-filled bar with a
// rounded data-end, and a hover readout.
export default function Columns({
  data, color = ink.primary, height = 200, format = (n: number) => String(n),
}: {
  data: Column[]
  color?: string
  height?: number
  format?: (n: number) => string
}) {
  const [hi, setHi] = useState<number | null>(null)
  if (!data.length) return <Empty />

  const W = Math.max(data.length * 34, 240), H = height
  const padL = 34, padB = 26, padT = 10
  const plotW = W - padL, plotH = H - padB - padT
  const max = Math.max(...data.map(d => d.value), 1)
  const niceMax = ceilNice(max)
  const bw = plotW / data.length
  const barW = Math.min(bw * 0.6, 22)
  const y = (v: number) => padT + plotH * (1 - v / niceMax)
  const ticks = [0, niceMax / 2, niceMax]
  // Label every nth column so an axis with many months stays legible.
  const step = Math.ceil(data.length / 12)

  return (
    <Box sx={{ position: 'relative', width: '100%', overflowX: 'auto' }}>
      <svg viewBox={`0 0 ${W} ${H}`} width="100%" height={H} style={{ display: 'block', fontFamily: 'IBM Plex Mono, monospace' }}
        preserveAspectRatio="xMinYMid meet" onMouseLeave={() => setHi(null)}>
        <defs><Hatch id="col-hatch" color={color} /></defs>

        {ticks.map((t, i) => (
          <g key={i}>
            <line x1={padL} x2={W} y1={y(t)} y2={y(t)} stroke={ink.line} strokeWidth={1}
              strokeDasharray={i === 0 ? '0' : '2 3'} />
            <text x={padL - 6} y={y(t) + 3} textAnchor="end" fontSize="9" fill={ink.textFaint}>{format(t)}</text>
          </g>
        ))}

        {data.map((d, i) => {
          const cx = padL + i * bw + bw / 2
          const top = y(d.value)
          const on = hi === i
          return (
            <g key={i} onMouseEnter={() => setHi(i)}>
              <rect x={padL + i * bw} y={padT} width={bw} height={plotH} fill="transparent" />
              {d.value > 0 && (
                <rect x={cx - barW / 2} y={top} width={barW} height={Math.max(plotH - (top - padT), 2)}
                  rx={2} fill="url(#col-hatch)" stroke={color} strokeWidth={on ? 2 : 1.4} />
              )}
              {i % step === 0 && (
                <text x={cx} y={H - padB + 14} textAnchor="middle" fontSize="9" fill={ink.textSoft}>{d.label}</text>
              )}
            </g>
          )
        })}
      </svg>

      {hi !== null && (
        <Box sx={{
          position: 'absolute', top: 0, left: `min(${(padL + hi * bw + bw / 2) / W * 100}%, calc(100% - 140px))`,
          bgcolor: 'text.primary', color: ink.paper, px: 1, py: .5, borderRadius: .5,
          fontFamily: '"IBM Plex Mono", monospace', fontSize: 11, pointerEvents: 'none', whiteSpace: 'nowrap',
        }}>
          {data[hi].sub ?? data[hi].label}: <b>{format(data[hi].value)}</b>
        </Box>
      )}
    </Box>
  )
}

function ceilNice(n: number): number {
  if (n <= 5) return 5
  const mag = Math.pow(10, Math.floor(Math.log10(n)))
  return Math.ceil(n / mag) * mag
}

function Empty() {
  return <Box sx={{ py: 5, textAlign: 'center', color: 'text.secondary', fontSize: 13 }}>No data yet.</Box>
}
