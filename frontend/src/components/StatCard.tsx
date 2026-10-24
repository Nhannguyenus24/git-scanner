import Box from '@mui/material/Box'
import Card from '@mui/material/Card'
import Typography from '@mui/material/Typography'

interface Props {
  label:    string
  value:    string | number
  hint?:    string
  accent?:  string
  onClick?: () => void
}

export default function StatCard({ label, value, hint, accent, onClick }: Props) {
  return (
    <Card onClick={onClick} sx={{
      p: 2, position: 'relative', overflow: 'hidden', cursor: onClick ? 'pointer' : 'default',
      transition: 'border-color .15s', '&:hover': onClick ? { borderColor: accent ?? 'primary.main' } : {},
    }}>
      {accent && <Box sx={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: 3, bgcolor: accent }} />}
      <Typography sx={{ fontSize: 11, fontWeight: 600, color: 'text.secondary', textTransform: 'uppercase', letterSpacing: '.07em' }}>
        {label}
      </Typography>
      <Typography sx={{ fontSize: 28, fontWeight: 800, lineHeight: 1.2, mt: .5, fontVariantNumeric: 'tabular-nums' }}>
        {value}
      </Typography>
      {hint && <Typography variant="caption" color="text.secondary">{hint}</Typography>}
    </Card>
  )
}
