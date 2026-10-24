import Box from '@mui/material/Box'
import Typography from '@mui/material/Typography'
import { ink } from '../theme'
import type { Count } from '../types'

interface Props {
  items:    Count[]
  color?:   (key: string) => string
  label?:   (key: string) => string
  onClick?: (key: string) => void
  empty?:   string
}

// A ranked bar list — cheaper to read than a pie for a handful of buckets, and it
// direct-labels every row, so identity never rests on colour. Bars are hatch-free
// solid fills with a hairline frame, like a filled gauge on a printout.
export default function BarList({ items, color, label, onClick, empty = 'Nothing yet' }: Props) {
  if (!items.length) return <Typography variant="body2" color="text.secondary">{empty}</Typography>
  const max = Math.max(...items.map(i => i.count))
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1.1 }}>
      {items.map(i => (
        <Box key={i.key} onClick={onClick ? () => onClick(i.key) : undefined}
          sx={{ cursor: onClick ? 'pointer' : 'default', '&:hover .bar': onClick ? { filter: 'brightness(0.92)' } : {} }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', fontSize: 12.5, mb: .4 }}>
            <span>{label ? label(i.key) : i.key}</span>
            <Box component="span" sx={{ fontFamily: '"IBM Plex Mono", monospace', color: 'text.secondary' }}>{i.count}</Box>
          </Box>
          <Box sx={{ height: 9, bgcolor: ink.cardInset, border: `1px solid ${ink.line}`, borderRadius: .5, overflow: 'hidden' }}>
            <Box className="bar" sx={{
              height: '100%', width: `${(i.count / max) * 100}%`, minWidth: 2,
              bgcolor: color ? color(i.key) : 'primary.main', transition: 'width .3s, filter .12s',
            }} />
          </Box>
        </Box>
      ))}
    </Box>
  )
}
