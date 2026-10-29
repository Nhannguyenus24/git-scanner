import React from 'react'
import Box from '@mui/material/Box'
import Card from '@mui/material/Card'

// A framed plate: a letterpress-style header rule over the chart body. The title
// is the figure's caption — for a single-series chart it names the series, so no
// legend box is needed.
export default function Figure({
  title, caption, right, actions, children, minHeight,
}: {
  title: string
  caption?: string
  right?: React.ReactNode
  /** Controls for the figure itself, e.g. downloading the numbers behind it. */
  actions?: React.ReactNode
  children: React.ReactNode
  minHeight?: number
}) {
  return (
    <Card sx={{ p: 0, height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Box sx={{
        display: 'flex', alignItems: 'baseline', justifyContent: 'space-between', gap: 1,
        px: 2, pt: 1.5, pb: 1, borderBottom: t => `1px solid ${t.palette.divider}`,
        // a thin second rule just below — the double rule of a printed frame
        boxShadow: t => `0 2px 0 -1px ${t.palette.divider}`,
      }}>
        <Box>
          <Box sx={{ fontFamily: '"IBM Plex Mono", monospace', fontSize: 13, fontWeight: 700, letterSpacing: '.02em' }}>{title}</Box>
          {caption && <Box sx={{ fontSize: 11.5, color: 'text.secondary', mt: .25 }}>{caption}</Box>}
        </Box>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: .5, flexShrink: 0 }}>{right}{actions}</Box>
      </Box>
      <Box sx={{ p: 2, flex: 1, minHeight }}>{children}</Box>
    </Card>
  )
}

// Hatch is a 45° hand-drawn-ish fill for a mark, so identity survives in print,
// greyscale and forced-colors. Each series gets a pattern keyed by id+color.
export function Hatch({ id, color, angle = 45 }: { id: string; color: string; angle?: number }) {
  return (
    <pattern id={id} width="6" height="6" patternUnits="userSpaceOnUse" patternTransform={`rotate(${angle})`}>
      <rect width="6" height="6" fill={color} fillOpacity="0.18" />
      <line x1="0" y1="0" x2="0" y2="6" stroke={color} strokeWidth="1.4" />
    </pattern>
  )
}
