import Box from '@mui/material/Box'
import Typography from '@mui/material/Typography'
import type { Count } from '../types'

interface Props {
  items:    Count[]
  color?:   (key: string) => string
  label?:   (key: string) => string
  onClick?: (key: string) => void
  empty?:   string
}

/** A ranked horizontal bar list: cheaper to read than a pie for a handful of buckets. */
export default function BarList({ items, color, label, onClick, empty = 'Nothing yet' }: Props) {
  if (!items.length) return <Typography variant="body2" color="text.secondary">{empty}</Typography>
  const max = Math.max(...items.map(i => i.count))
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', gap: 1 }}>
      {items.map(i => (
        <Box key={i.key} onClick={onClick ? () => onClick(i.key) : undefined}
          sx={{ cursor: onClick ? 'pointer' : 'default', '&:hover .bar': onClick ? { opacity: 1 } : {} }}>
          <Box sx={{ display: 'flex', justifyContent: 'space-between', fontSize: 13, mb: .25 }}>
            <span>{label ? label(i.key) : i.key}</span>
            <Box component="span" sx={{ fontVariantNumeric: 'tabular-nums', color: 'text.secondary' }}>{i.count}</Box>
          </Box>
          <Box sx={{ height: 6, bgcolor: 'divider', borderRadius: 3, overflow: 'hidden' }}>
            <Box className="bar" sx={{
              height: '100%', width: `${(i.count / max) * 100}%`, borderRadius: 3, opacity: .85,
              bgcolor: color ? color(i.key) : 'primary.main', transition: 'width .3s',
            }} />
          </Box>
        </Box>
      ))}
    </Box>
  )
}
