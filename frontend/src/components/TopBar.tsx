import AppBar from '@mui/material/AppBar'
import Box from '@mui/material/Box'
import TextField from '@mui/material/TextField'
import Toolbar from '@mui/material/Toolbar'
import Tooltip from '@mui/material/Tooltip'
import Typography from '@mui/material/Typography'
import type { ApiStatus } from '../hooks/useApiStatus'

const STATUS_COLOR: Record<ApiStatus, string> = {
  checking: '#6b789a',
  ok:       '#4ec9a3',
  error:    '#f07070',
}
const STATUS_LABEL: Record<ApiStatus, string> = {
  checking: 'Checking API…',
  ok:       'API connected',
  error:    'API unreachable',
}

interface Props {
  baseUrl:   string
  onBaseUrl: (url: string) => void
  status:    ApiStatus
}

export default function TopBar({ baseUrl, onBaseUrl, status }: Props) {
  return (
    <AppBar position="sticky" elevation={0}>
      <Toolbar sx={{ gap: 2, minHeight: '52px !important', px: 2 }}>
        {/* logo */}
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, flexShrink: 0 }}>
          <Box sx={{
            width: 28, height: 28, bgcolor: 'primary.main', borderRadius: 1,
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontSize: 10, fontWeight: 900, color: '#000', letterSpacing: '-.03em',
          }}>
            GS
          </Box>
          <Typography sx={{ fontWeight: 700, fontSize: 15, letterSpacing: '-.01em' }}>
            GitScanner
          </Typography>
        </Box>

        {/* spacer */}
        <Box sx={{ flex: 1 }} />

        {/* base URL */}
        <Tooltip title={STATUS_LABEL[status]}>
          <TextField
            value={baseUrl}
            onChange={e => onBaseUrl(e.target.value)}
            placeholder="http://localhost:8080"
            sx={{ width: 250, '& input': { fontFamily: 'monospace', fontSize: 12 } }}
            InputProps={{
              startAdornment: (
                <Box component="span" sx={{ mr: 1, fontSize: 10, color: STATUS_COLOR[status], flexShrink: 0 }}>●</Box>
              ),
            }}
          />
        </Tooltip>
      </Toolbar>
    </AppBar>
  )
}
