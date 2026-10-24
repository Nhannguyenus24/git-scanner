import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import CircularProgress from '@mui/material/CircularProgress'
import Typography from '@mui/material/Typography'
import { useT } from '../i18n'

export function Loading() {
  return <Box sx={{ p: 6, textAlign: 'center' }}><CircularProgress size={28} /></Box>
}

export function LoadError({ error, retry }: { error: string; retry: () => void }) {
  const { t } = useT()
  return (
    <Alert severity="error" action={<Button color="inherit" size="small" onClick={retry}>{t('common.retry')}</Button>}>
      {t('common.loadError', { error })}
    </Alert>
  )
}

export function Empty({ title, body, action }: { title: string; body?: string; action?: React.ReactNode }) {
  return (
    <Box sx={{ p: 6, textAlign: 'center', color: 'text.secondary', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1 }}>
      <Typography variant="h6" sx={{ fontWeight: 700, color: 'text.primary' }}>{title}</Typography>
      {body && <Typography variant="body2" sx={{ maxWidth: 420 }}>{body}</Typography>}
      {action}
    </Box>
  )
}
