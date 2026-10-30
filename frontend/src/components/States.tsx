import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Skeleton from '@mui/material/Skeleton'
import Typography from '@mui/material/Typography'
import Illustration, { type IllustrationName } from './Illustration'
import { RadarLoader } from './Instrument'
import { useT } from '../i18n'

export function Loading() {
  const { t } = useT()
  return <RadarLoader label={t('common.loading')} />
}

/**
 * A placeholder shaped like the thing that is coming. A spinner says only "wait";
 * a table skeleton says "a table of about this size", so the page does not jump
 * when the rows arrive.
 */
export function TableSkeleton({ rows = 6, columns = 5 }: { rows?: number; columns?: number }) {
  return (
    <Box sx={{ p: 2 }} aria-busy="true">
      {Array.from({ length: rows }, (_, r) => (
        <Box key={r} sx={{ display: 'flex', gap: 2, py: .75, alignItems: 'center' }}>
          {Array.from({ length: columns }, (_, c) => (
            <Skeleton key={c} variant="text" height={18}
              sx={{ flex: c === 1 ? 3 : 1, opacity: 1 - r * 0.1 }} />
          ))}
        </Box>
      ))}
    </Box>
  )
}

export function CardSkeleton({ height = 120 }: { height?: number }) {
  return <Skeleton variant="rectangular" height={height} sx={{ borderRadius: 1 }} />
}

/**
 * A failed load, with the one action that can fix it. The error text comes from
 * the API client, which carries the server's `error` field or an HTTP status —
 * never a response body, so nothing sensitive is echoed into the page.
 */
export function LoadError({ error, retry, illustration = true }: { error: string; retry: () => void; illustration?: boolean }) {
  const { t } = useT()
  return (
    <Box>
      {illustration && (
        <Box sx={{ pt: 2, pb: 1 }}><Illustration name="offline" size={190} /></Box>
      )}
      <Alert severity="error" action={<Button color="inherit" size="small" onClick={retry}>{t('common.retry')}</Button>}>
        {t('common.loadError', { error })}
      </Alert>
    </Box>
  )
}

interface EmptyProps {
  title:  string
  body?:  string
  action?: React.ReactNode
  /** The picture for this particular nothing. Empty states that look alike get
   *  skipped alike, so each one gets art that names its situation. */
  art?:   IllustrationName
  size?:  number
}

export function Empty({ title, body, action, art, size = 220 }: EmptyProps) {
  return (
    <Box sx={{ p: { xs: 3, md: 6 }, textAlign: 'center', color: 'text.secondary', display: 'flex', flexDirection: 'column', alignItems: 'center', gap: 1.5 }}>
      {art && <Illustration name={art} size={size} />}
      <Typography variant="h6" sx={{ fontWeight: 700, color: 'text.primary', mt: art ? 1 : 0 }}>{title}</Typography>
      {body && <Typography variant="body2" sx={{ maxWidth: 440, lineHeight: 1.6 }}>{body}</Typography>}
      {action && <Box sx={{ mt: .5, display: 'flex', gap: 1, flexWrap: 'wrap', justifyContent: 'center' }}>{action}</Box>}
    </Box>
  )
}

/** The same state, inline in a card rather than filling a page. */
export function EmptyInline({ title, art, size = 120 }: { title: string; art?: IllustrationName; size?: number }) {
  return (
    <Box sx={{ py: 3, textAlign: 'center', color: 'text.secondary' }}>
      {art && <Box sx={{ mb: 1 }}><Illustration name={art} size={size} /></Box>}
      <Typography variant="body2">{title}</Typography>
    </Box>
  )
}
