import { useCallback, useState } from 'react'
import Alert from '@mui/material/Alert'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Card from '@mui/material/Card'
import Chip from '@mui/material/Chip'
import Divider from '@mui/material/Divider'
import Link from '@mui/material/Link'
import Switch from '@mui/material/Switch'
import TextField from '@mui/material/TextField'
import MailOutlineIcon from '@mui/icons-material/MailOutline'
import VisibilityOffIcon from '@mui/icons-material/VisibilityOffOutlined'
import { PageHeader } from '../components/Layout'
import { LoadError, Loading } from '../components/States'
import { useResource } from '../hooks/useResource'
import { href } from '../hooks/useRoute'
import { useSettings } from '../store/settings'
import { useToast } from '../store/toast'
import { useT } from '../i18n'
import type { AppSettings } from '../types'

/** One labelled switch with its explanation, the unit every section is built from. */
function Row({ icon, title, body, checked, disabled, color, onChange }: {
  icon: React.ReactNode; title: string; body: React.ReactNode; checked: boolean
  disabled?: boolean; color?: 'warning' | 'error' | 'primary'; onChange: (v: boolean) => void
}) {
  return (
    <Box sx={{ display: 'flex', gap: 2, alignItems: 'flex-start', p: 2.5 }}>
      <Box sx={{ color: 'text.secondary', pt: .25 }}>{icon}</Box>
      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Box sx={{ fontWeight: 700, fontSize: 15 }}>{title}</Box>
        <Box sx={{ color: 'text.secondary', fontSize: 13, mt: .5, lineHeight: 1.55 }}>{body}</Box>
      </Box>
      <Switch color={color} checked={checked} disabled={disabled} onChange={e => onChange(e.target.checked)} />
    </Box>
  )
}

export default function SettingsPage() {
  const { api } = useSettings()
  const { t } = useT()
  const notify = useToast()
  const res = useResource(() => api.settings(), [api])
  const [owner, setOwner] = useState('')

  const apply = useCallback((p: Promise<AppSettings>, ok: string, severity: 'success' | 'info' = 'success') => {
    p.then(() => { notify(ok, severity); res.reload() })
      .catch(e => notify(e instanceof Error ? e.message : 'Error', 'error'))
  }, [notify, res])

  if (res.loading && !res.data) return <Loading />
  if (res.error && !res.data) return <LoadError error={res.error} retry={res.reload} />
  const s = res.data!

  const addOwner = () => {
    const o = owner.trim()
    if (!o) return
    apply(api.addUnmaskOwner(o), t('settings.ownerAdded', { owner: o }))
    setOwner('')
  }

  return (
    <>
      <PageHeader title={t('settings.title')} subtitle={t('settings.subtitle')} />

      <Card sx={{ mb: 2 }}>
        <Row icon={<MailOutlineIcon />} title={t('settings.mail.title')} color="warning"
          body={<>{t('settings.mail.body')} <Link href={href('notifications')}>{t('settings.mail.log')}</Link></>}
          checked={s.notifications_enabled}
          onChange={v => apply(api.setSettings({ notifications_enabled: v }), t(v ? 'repos.notify.on' : 'repos.notify.off'), v ? 'success' : 'info')} />
      </Card>

      <Card>
        <Row icon={<VisibilityOffIcon />} title={t('settings.mask.title')} color="primary"
          body={t('settings.mask.body')}
          checked={s.masking_enabled} disabled={!s.sealing_available && s.masking_enabled}
          onChange={v => apply(api.setSettings({ masking_enabled: v }), t(v ? 'settings.mask.on' : 'settings.mask.off'), v ? 'success' : 'info')} />

        {!s.sealing_available && (
          <Alert severity="info" sx={{ mx: 2.5, mb: 2 }}>{t('settings.mask.noKey')}</Alert>
        )}
        {s.sealing_available && !s.masking_enabled && (
          <Alert severity="error" sx={{ mx: 2.5, mb: 2 }}>{t('settings.mask.offWarning')}</Alert>
        )}

        <Divider />
        <Box sx={{ p: 2.5 }}>
          <Box sx={{ fontWeight: 700, fontSize: 15 }}>{t('settings.owners.title')}</Box>
          <Box sx={{ color: 'text.secondary', fontSize: 13, mt: .5, mb: 1.5, lineHeight: 1.55 }}>{t('settings.owners.body')}</Box>
          <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', mb: 1.5 }}>
            {s.unmask_owners.length
              ? s.unmask_owners.map(o => (
                <Chip key={o} label={o} onDelete={() => apply(api.removeUnmaskOwner(o), t('settings.ownerRemoved', { owner: o }), 'info')} />
              ))
              : <Box sx={{ color: 'text.disabled', fontSize: 13 }}>{t('settings.owners.empty')}</Box>}
          </Box>
          <Box component="form" sx={{ display: 'flex', gap: 1, maxWidth: 420 }}
            onSubmit={(e: React.FormEvent) => { e.preventDefault(); addOwner() }}>
            <TextField size="small" fullWidth placeholder={t('settings.owners.placeholder')} value={owner}
              onChange={e => setOwner(e.target.value)} disabled={!s.sealing_available} />
            <Button type="submit" variant="outlined" disabled={!s.sealing_available || !owner.trim()}>{t('settings.owners.add')}</Button>
          </Box>
          <Box sx={{ color: 'text.secondary', fontSize: 12, mt: 1.5 }}>{t('settings.owners.rescan')}</Box>
        </Box>
      </Card>
    </>
  )
}
