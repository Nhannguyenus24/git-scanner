import { useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Checkbox from '@mui/material/Checkbox'
import Chip from '@mui/material/Chip'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import DialogTitle from '@mui/material/DialogTitle'
import FormControlLabel from '@mui/material/FormControlLabel'
import IconButton from '@mui/material/IconButton'
import MenuItem from '@mui/material/MenuItem'
import Radio from '@mui/material/Radio'
import RadioGroup from '@mui/material/RadioGroup'
import TextField from '@mui/material/TextField'
import Tooltip from '@mui/material/Tooltip'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import { timeAgo } from '../format'
import { useResource } from '../hooks/useResource'
import { href } from '../hooks/useRoute'
import { useT } from '../i18n'
import { useSettings } from '../store/settings'
import { useToast } from '../store/toast'
import type { Repository, ShareDetail } from '../types'

const EXPIRY_DAYS = [1, 3, 7, 14, 30]
const MONO = '"IBM Plex Mono", monospace'

/** The address of a shared report in this app, for a token. Hash routing keeps
 *  the token out of every request the page makes and out of the Referer it
 *  sends when someone follows a link in the report. */
export function shareUrl(token: string): string {
  const { origin, pathname } = window.location
  return `${origin}${pathname}${href('shared', token)}`
}

/**
 * Creates, lists and revokes the share links of one repository.
 *
 * Summary is the default and full needs choosing: a full report says where each
 * credential sits in public history, which is what the owner needs and what an
 * attacker wants. The created link is shown once, because the server keeps only
 * a hash of it.
 */
export default function ShareDialog({ repo, onClose }: { repo: Repository; onClose: () => void }) {
  const { api } = useSettings()
  const { t } = useT()
  const notify = useToast()
  const [detail, setDetail] = useState<ShareDetail>('summary')
  const [days, setDays] = useState(7)
  const [resolved, setResolved] = useState(false)
  const [busy, setBusy] = useState(false)
  const [created, setCreated] = useState<string>()
  const links = useResource(() => api.shares(repo.id), [api, repo.id])

  const create = () => {
    setBusy(true)
    api.createShare(repo.id, detail, days, resolved)
      .then(l => { setCreated(shareUrl(l.token!)); links.reload() })
      .catch(e => notify(e instanceof Error ? e.message : 'Error', 'error'))
      .finally(() => setBusy(false))
  }
  const revoke = (id: string) =>
    api.revokeShare(id).then(() => { notify(t('share.revoked')); links.reload() })
      .catch(e => notify(e instanceof Error ? e.message : 'Error', 'error'))
  const copy = () => created && navigator.clipboard?.writeText(created).then(() => notify(t('share.copied')))

  return (
    <Dialog open onClose={onClose} fullWidth maxWidth="sm" aria-labelledby="share-title">
      <DialogTitle id="share-title">
        {t('share.title')}
        <Box sx={{ fontFamily: MONO, fontSize: 13, color: 'text.secondary', fontWeight: 400 }}>{repo.owner}/{repo.name}</Box>
      </DialogTitle>
      <DialogContent dividers>
        {created ? (
          <Box>
            <Alert severity="warning" sx={{ mb: 1.5 }}>{t('share.onceOnly')}</Alert>
            <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>
              <TextField value={created} fullWidth size="small" InputProps={{ readOnly: true, sx: { fontFamily: MONO, fontSize: 12.5 } }}
                inputProps={{ 'aria-label': t('share.link') }} onFocus={e => e.target.select()} />
              <Tooltip title={t('share.copy')}>
                <IconButton onClick={copy} aria-label={t('share.copy')}><ContentCopyIcon fontSize="small" /></IconButton>
              </Tooltip>
            </Box>
            <Button size="small" sx={{ mt: 1 }} onClick={() => setCreated(undefined)}>{t('share.another')}</Button>
          </Box>
        ) : (
          <Box>
            <Box sx={{ fontSize: 13, fontWeight: 700, mb: .5 }}>{t('share.detail')}</Box>
            <RadioGroup value={detail} onChange={e => setDetail(e.target.value as ShareDetail)}>
              <FormControlLabel value="summary" control={<Radio size="small" />} label={
                <Box><Box sx={{ fontSize: 14 }}>{t('share.summary')}</Box>
                  <Box sx={{ fontSize: 12.5, color: 'text.secondary' }}>{t('share.summaryHint')}</Box></Box>} sx={{ alignItems: 'flex-start', mb: 1 }} />
              <FormControlLabel value="full" control={<Radio size="small" />} label={
                <Box><Box sx={{ fontSize: 14 }}>{t('share.full')}</Box>
                  <Box sx={{ fontSize: 12.5, color: 'text.secondary' }}>{t('share.fullHint')}</Box></Box>} sx={{ alignItems: 'flex-start' }} />
            </RadioGroup>
            {detail === 'full' && <Alert severity="warning" sx={{ mt: 1 }}>{t('share.fullWarning')}</Alert>}
            <Box sx={{ display: 'flex', gap: 2, alignItems: 'center', mt: 2, flexWrap: 'wrap' }}>
              <TextField select size="small" label={t('share.expires')} value={days} onChange={e => setDays(Number(e.target.value))} sx={{ minWidth: 160 }}>
                {EXPIRY_DAYS.map(d => <MenuItem key={d} value={d}>{t('share.days', { n: d })}</MenuItem>)}
              </TextField>
              <FormControlLabel control={<Checkbox size="small" checked={resolved} onChange={e => setResolved(e.target.checked)} />}
                label={<Box sx={{ fontSize: 14 }}>{t('share.includeResolved')}</Box>} />
            </Box>
          </Box>
        )}

        <Box sx={{ mt: 3 }}>
          <Box sx={{ fontSize: 11, fontWeight: 700, letterSpacing: '.08em', textTransform: 'uppercase', color: 'text.secondary', mb: .75 }}>
            {t('share.existing')}
          </Box>
          {!links.data?.shares.length ? (
            <Box sx={{ fontSize: 13, color: 'text.secondary' }}>{t('share.none')}</Box>
          ) : links.data.shares.map(l => (
            <Box key={l.id} sx={{ display: 'flex', alignItems: 'center', gap: 1, py: .6, borderTop: 1, borderColor: 'divider', flexWrap: 'wrap' }}>
              <Chip size="small" label={t(`share.state.${l.state}`)} color={l.state === 'active' ? 'success' : 'default'} variant="outlined" />
              <Box sx={{ fontSize: 13, fontWeight: 600 }}>{t(`share.${l.detail}`)}</Box>
              <Box sx={{ fontSize: 12, color: 'text.secondary', flex: 1, minWidth: 180 }}>
                {t('share.meta', { created: timeAgo(l.created_at), expires: new Date(l.expires_at).toLocaleDateString(), views: l.views })}
              </Box>
              {l.state === 'active' && <Button size="small" color="error" onClick={() => revoke(l.id)}>{t('share.revoke')}</Button>}
            </Box>
          ))}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{t('share.close')}</Button>
        {!created && <Button variant="contained" disabled={busy} onClick={create}>{t('share.create')}</Button>}
      </DialogActions>
    </Dialog>
  )
}
