import Chip from '@mui/material/Chip'
import { LiveDot } from './Instrument'
import { useT } from '../i18n'
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
  const { t } = useT()
  return (
    <Chip
      label={t(`state.${state}`)}
      color={COLOR[state]}
      size={size}
      variant={state === 'running' ? 'filled' : 'outlined'}
      icon={state === 'running' ? <LiveDot color="currentColor" size={6} /> : undefined}
      sx={{ '& .MuiChip-icon': { ml: 1, mr: -.25 } }}
    />
  )
}
