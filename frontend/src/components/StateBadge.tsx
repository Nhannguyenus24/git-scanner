import Chip from '@mui/material/Chip'
import type { ScanState } from '../types'

const COLOR: Record<ScanState, 'default' | 'primary' | 'warning' | 'success' | 'error'> = {
  queued:    'default',
  running:   'primary',
  paused:    'warning',
  completed: 'success',
  failed:    'error',
  cancelled: 'default',
}

interface Props { state: ScanState; size?: 'small' | 'medium' }

export default function StateBadge({ state, size = 'small' }: Props) {
  return (
    <Chip
      label={state}
      color={COLOR[state]}
      size={size}
      variant={state === 'running' ? 'filled' : 'outlined'}
    />
  )
}
