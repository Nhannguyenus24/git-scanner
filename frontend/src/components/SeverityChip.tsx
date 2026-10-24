import Box from '@mui/material/Box'
import { SEV_COLOR } from '../format'
import type { Severity } from '../types'

export default function SeverityChip({ severity }: { severity: Severity }) {
  const c = SEV_COLOR[severity]
  return (
    <Box component="span" sx={{
      display: 'inline-flex', alignItems: 'center', gap: .75, px: 1, py: .25, borderRadius: 1,
      fontSize: 11, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.04em',
      color: c, bgcolor: `${c}1f`, border: `1px solid ${c}55`, whiteSpace: 'nowrap',
    }}>
      <Box component="span" sx={{ width: 6, height: 6, borderRadius: '50%', bgcolor: c }} />
      {severity}
    </Box>
  )
}
