import { useState } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import CircularProgress from '@mui/material/CircularProgress'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import DialogTitle from '@mui/material/DialogTitle'
import Paper from '@mui/material/Paper'
import Slide from '@mui/material/Slide'
import CloseIcon from '@mui/icons-material/Close'
import IconButton from '@mui/material/IconButton'
import ExportMenu from './ExportMenu'
import { useT } from '../i18n'
import { STATUSES, type Finding, type FindingStatus } from '../types'
import type { ExportScope } from '../export'

// The bar that appears once rows are selected.
//
// Two of the four decisions are confirmed first. Marking something rotated or a
// false positive is a claim about the outside world — that the key was revoked,
// that the match was wrong — and in bulk it is the claim that quietly closes a
// real leak. Open and ignored are reversible from the same bar, so they go
// straight through.
const CONFIRM: FindingStatus[] = ['rotated', 'false_positive']

interface Props {
  findings: Finding[]
  running:  boolean
  onClear:  () => void
  onStatus: (s: FindingStatus) => void
  scope:    ExportScope
}

export default function BulkBar({ findings, running, onClear, onStatus, scope }: Props) {
  const { t } = useT()
  const [confirm, setConfirm] = useState<FindingStatus | null>(null)
  const n = findings.length

  const ask = (s: FindingStatus) => (CONFIRM.includes(s) ? setConfirm(s) : onStatus(s))

  return (
    <>
      <Slide direction="up" in={n > 0} mountOnEnter unmountOnExit>
        <Paper elevation={0} sx={{
          position: 'fixed', bottom: { xs: 70, md: 20 }, left: '50%', transform: 'translateX(-50%)',
          zIndex: 1250, border: 1, borderColor: 'divider', borderRadius: 1,
          px: 2, py: 1.25, display: 'flex', gap: 1.5, alignItems: 'center', flexWrap: 'wrap',
          maxWidth: 'calc(100vw - 32px)', boxShadow: 3,
        }} role="region" aria-label={t('bulk.title')}>
          <Box sx={{ fontWeight: 700, fontSize: 13.5, whiteSpace: 'nowrap' }}>{t('bulk.selected', { n })}</Box>
          {running && <CircularProgress size={16} />}
          <Box sx={{ display: 'flex', gap: .5, flexWrap: 'wrap' }}>
            {STATUSES.map(s => (
              <Button key={s} size="small" disabled={running} onClick={() => ask(s)}
                variant={s === 'rotated' ? 'contained' : 'outlined'}>
                {t(`status.${s}`)}
              </Button>
            ))}
          </Box>
          <ExportMenu findings={findings} scope={{ ...scope, total: n }} label={t('bulk.export')} />
          <IconButton size="small" onClick={onClear} aria-label={t('bulk.clear')}><CloseIcon fontSize="small" /></IconButton>
        </Paper>
      </Slide>

      <Dialog open={!!confirm} onClose={() => setConfirm(null)} maxWidth="xs" fullWidth>
        <DialogTitle sx={{ fontSize: 17, fontWeight: 700 }}>
          {confirm && t('bulk.confirmTitle', { status: t(`status.${confirm}`), n })}
        </DialogTitle>
        <DialogContent>
          <Box sx={{ fontSize: 14, color: 'text.secondary', lineHeight: 1.6 }}>
            {confirm === 'rotated' ? t('bulk.confirmRotated') : t('bulk.confirmFalse')}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button color="inherit" onClick={() => setConfirm(null)}>{t('common.cancel')}</Button>
          <Button variant="contained" onClick={() => { if (confirm) onStatus(confirm); setConfirm(null) }}>
            {t('common.confirm')}
          </Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
