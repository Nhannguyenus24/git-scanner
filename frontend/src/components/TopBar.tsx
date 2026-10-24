import AppBar from '@mui/material/AppBar'
import Box from '@mui/material/Box'
import TextField from '@mui/material/TextField'
import Toolbar from '@mui/material/Toolbar'
import Tooltip from '@mui/material/Tooltip'
import Typography from '@mui/material/Typography'
import { useApiStatus, type ApiStatus } from '../hooks/useApiStatus'
import { href } from '../hooks/useRoute'
import { useSettings } from '../store/settings'

const STATUS_COLOR: Record<ApiStatus, string> = { checking: '#6b789a', ok: '#4ec9a3', error: '#f07070' }
const STATUS_LABEL: Record<ApiStatus, string> = { checking: 'Checking API…', ok: 'API connected', error: 'API unreachable' }

export default function TopBar() {
  const { baseUrl, setBaseUrl } = useSettings()
  const status = useApiStatus(baseUrl)
  return (
    <AppBar position="sticky" elevation={0}>
      <Toolbar sx={{ gap: 2, minHeight: '52px !important', px: 2 }}>
        <Box component="a" href={href('overview')} sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'inherit', textDecoration: 'none' }}>
          <Box sx={{
            width: 28, height: 28, bgcolor: 'primary.main', borderRadius: 1, display: 'flex', alignItems: 'center',
            justifyContent: 'center', fontSize: 10, fontWeight: 900, color: '#000',
          }}>GS</Box>
          <Typography sx={{ fontWeight: 700, fontSize: 15 }}>GitScanner</Typography>
        </Box>
        <Box sx={{ flex: 1 }} />
        <Tooltip title={STATUS_LABEL[status]}>
          <TextField
            value={baseUrl}
            onChange={e => setBaseUrl(e.target.value)}
            placeholder="http://localhost:8080"
            sx={{ width: { xs: 170, sm: 250 }, '& input': { fontFamily: 'monospace', fontSize: 12 } }}
            InputProps={{ startAdornment: <Box component="span" sx={{ mr: 1, fontSize: 10, color: STATUS_COLOR[status] }}>●</Box> }}
          />
        </Tooltip>
      </Toolbar>
    </AppBar>
  )
}
