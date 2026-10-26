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

const PLACEHOLDER: Record<'repo' | 'user' | 'org', string> = {
  repo: 'https://github.com/owner/repo',
  user: 'octocat',
  org:  'github',
}

export default function NewScanDialog({ open, onClose, onSubmit }: Props) {
  const { t } = useT()
  const [kind,    setKind]    = useState<TargetKind>('repo')
  const [value,   setValue]   = useState('')
  const [count,   setCount]   = useState('10')
  const [forever, setForever] = useState(false)
  const [forks,   setForks]   = useState(false)
  const [loading, setLoading] = useState(false)
  const [error,   setError]   = useState('')

  const reset = () => {
    setValue(''); setForks(false); setError(''); setLoading(false)
    setCount('10'); setForever(false)
  }
  const handleClose = () => { reset(); onClose() }

  const handleSubmit = async () => {
    // For the random target, value carries the count ("0" = forever); the other
    // kinds carry a URL or login.
    let submitValue: string
    if (kind === 'random') {
      if (!forever && (!/^\d+$/.test(count.trim()) || Number(count.trim()) <= 0)) {
        setError(t('dialog.countInvalid')); return
      }
      submitValue = forever ? '0' : count.trim()
    } else {
      if (!value.trim()) { setError(t('dialog.required')); return }
      submitValue = value.trim()
    }
    setLoading(true); setError('')
    try   { await onSubmit(kind, submitValue, forks); reset(); onClose() }
    catch (e) { setError(e instanceof Error ? e.message : 'Error'); setLoading(false) }
  }

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="sm" fullWidth>
      <DialogTitle sx={{ fontFamily: '"IBM Plex Mono", monospace', fontWeight: 700 }}>{t('dialog.title')}</DialogTitle>

      <DialogContent sx={{ pt: '12px !important', display: 'flex', flexDirection: 'column', gap: 2 }}>
        <Tabs value={kind} onChange={(_, v: TargetKind) => { setKind(v); setValue(''); setError('') }}
          textColor="primary" indicatorColor="primary">
          <Tab label={t('dialog.repo')}   value="repo" />
          <Tab label={t('dialog.user')}   value="user" />
          <Tab label={t('dialog.org')}    value="org" />
          <Tab label={t('dialog.random')} value="random" />
        </Tabs>

        {kind === 'random' ? (
          <>
            <FormControlLabel
              control={<Switch checked={forever} onChange={e => { setForever(e.target.checked); setError('') }} color="primary" />}
              label={t('dialog.forever')} />
            <TextField
              fullWidth autoFocus type="number"
              label={t('dialog.count')}
              value={count}
              disabled={forever}
              onChange={e => { setCount(e.target.value); setError('') }}
              error={!!error}
              helperText={error || t('dialog.help.random')}
              inputProps={{ min: 1 }}
              onKeyDown={e => { if (e.key === 'Enter') handleSubmit() }}
            />
          </>
        ) : (
          <TextField
            fullWidth autoFocus
            label={kind === 'repo' ? t('dialog.cloneUrl') : t('dialog.login')}
            placeholder={PLACEHOLDER[kind as 'repo' | 'user' | 'org']}
            value={value}
            onChange={e => { setValue(e.target.value); setError('') }}
            error={!!error}
            helperText={error || t(`dialog.help.${kind}`)}
            onKeyDown={e => { if (e.key === 'Enter') handleSubmit() }}
          />
        )}

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
