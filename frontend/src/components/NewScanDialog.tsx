import { useState } from 'react'
import Button from '@mui/material/Button'
import CircularProgress from '@mui/material/CircularProgress'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import DialogTitle from '@mui/material/DialogTitle'
import FormControlLabel from '@mui/material/FormControlLabel'
import Switch from '@mui/material/Switch'
import Tab from '@mui/material/Tab'
import Tabs from '@mui/material/Tabs'
import TextField from '@mui/material/TextField'
import { useT } from '../i18n'
import type { TargetKind } from '../types'

interface Props {
  open:      boolean
  onClose:   () => void
  onSubmit:  (kind: TargetKind, value: string, forks: boolean) => Promise<void>
}

const PLACEHOLDER: Record<TargetKind, string> = {
  repo: 'https://github.com/owner/repo',
  user: 'octocat',
  org:  'github',
}

export default function NewScanDialog({ open, onClose, onSubmit }: Props) {
  const { t } = useT()
  const [kind,    setKind]    = useState<TargetKind>('repo')
  const [value,   setValue]   = useState('')
  const [forks,   setForks]   = useState(false)
  const [loading, setLoading] = useState(false)
  const [error,   setError]   = useState('')

  const reset = () => { setValue(''); setForks(false); setError(''); setLoading(false) }
  const handleClose = () => { reset(); onClose() }

  const handleSubmit = async () => {
    if (!value.trim()) { setError(t('dialog.required')); return }
    setLoading(true); setError('')
    try   { await onSubmit(kind, value.trim(), forks); reset(); onClose() }
    catch (e) { setError(e instanceof Error ? e.message : 'Error'); setLoading(false) }
  }

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ fontFamily: '"IBM Plex Mono", monospace', fontWeight: 700 }}>{t('dialog.title')}</DialogTitle>

      <DialogContent sx={{ pt: '12px !important', display: 'flex', flexDirection: 'column', gap: 2 }}>
        <Tabs value={kind} onChange={(_, v: TargetKind) => { setKind(v); setValue(''); setError('') }}
          textColor="primary" indicatorColor="primary">
          <Tab label={t('dialog.repo')} value="repo" />
          <Tab label={t('dialog.user')} value="user" />
          <Tab label={t('dialog.org')}  value="org" />
        </Tabs>

        <TextField
          fullWidth autoFocus
          label={kind === 'repo' ? t('dialog.cloneUrl') : t('dialog.login')}
          placeholder={PLACEHOLDER[kind]}
          value={value}
          onChange={e => { setValue(e.target.value); setError('') }}
          error={!!error}
          helperText={error || t(`dialog.help.${kind}`)}
          onKeyDown={e => { if (e.key === 'Enter') handleSubmit() }}
        />

        {kind !== 'repo' && (
          <FormControlLabel
            control={<Switch checked={forks} onChange={e => setForks(e.target.checked)} color="primary" />}
            label={t('dialog.includeForks')} />
        )}
      </DialogContent>

      <DialogActions sx={{ px: 3, pb: 2, gap: 1 }}>
        <Button onClick={handleClose} color="inherit">{t('common.cancel')}</Button>
        <Button onClick={handleSubmit} variant="contained" disabled={loading}>
          {loading && <CircularProgress size={14} sx={{ mr: 1 }} />}
          {t('dialog.start')}
        </Button>
      </DialogActions>
    </Dialog>
  )
}
