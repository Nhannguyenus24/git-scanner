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
import ToggleButton from '@mui/material/ToggleButton'
import ToggleButtonGroup from '@mui/material/ToggleButtonGroup'
import MailOutlineIcon from '@mui/icons-material/MailOutline'
import PaletteIcon from '@mui/icons-material/PaletteOutlined'
import VisibilityOffIcon from '@mui/icons-material/VisibilityOffOutlined'
import { PageHeader } from '../components/Layout'
import { LoadError, Loading } from '../components/States'
import { useResource } from '../hooks/useResource'
import { href } from '../hooks/useRoute'
import { usePrefs } from '../store/preferences'
import { useSettings } from '../store/settings'
import { useToast } from '../store/toast'
import { useT } from '../i18n'
import { presetSwatches, THEME_PRESETS, type ThemePreset } from '../theme'
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

/** A labelled choice, for the preferences that are not a yes/no. */
function Choice({ label, help, value, options, onChange }: {
  label: string; help?: string; value: string
  options: { value: string; label: string }[]
  onChange: (v: string) => void
}) {
  return (
    <Box sx={{ display: 'flex', gap: 2, alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', px: 2.5, py: 2 }}>
      <Box sx={{ minWidth: 0 }}>
        <Box sx={{ fontWeight: 600, fontSize: 14 }}>{label}</Box>
        {help && <Box sx={{ color: 'text.secondary', fontSize: 12.5, mt: .25 }}>{help}</Box>}
      </Box>
      <ToggleButtonGroup size="small" exclusive value={value} onChange={(_, v: string | null) => v && onChange(v)} aria-label={label}>
        {options.map(o => <ToggleButton key={o.value} value={o.value}>{o.label}</ToggleButton>)}
      </ToggleButtonGroup>
    </Box>
  )
}

/** A preset shown as the four inks it actually prints in, not as its name. */
function ThemePreview({ preset, mode, selected, onPick, label }: {
  preset: ThemePreset; mode: 'light' | 'dark'; selected: boolean; onPick: () => void; label: string
}) {
  const s = presetSwatches(preset, mode)
  return (
    <Box
      component="button" type="button" onClick={onPick} aria-pressed={selected}
      sx={{
        p: 1, borderRadius: 1, cursor: 'pointer', textAlign: 'left', font: 'inherit',
        border: 2, borderColor: selected ? 'primary.main' : 'divider',
        bgcolor: 'transparent', display: 'flex', flexDirection: 'column', gap: .75, width: '100%',
        '&:hover': { borderColor: selected ? 'primary.main' : 'text.secondary' },
      }}>
      <Box sx={{ height: 44, borderRadius: .5, bgcolor: s.paper, border: 1, borderColor: 'divider', p: .75, display: 'flex', gap: .5, alignItems: 'flex-end' }}>
        <Box sx={{ flex: 1, height: 10, bgcolor: s.ink, opacity: .75, borderRadius: .25 }} />
        <Box sx={{ width: 10, height: 22, bgcolor: s.accent, borderRadius: .25 }} />
        <Box sx={{ width: 10, height: 30, bgcolor: s.alert, borderRadius: .25 }} />
      </Box>
      <Box sx={{ fontSize: 12.5, fontWeight: selected ? 700 : 500 }}>{label}</Box>
    </Box>
  )
}

export default function SettingsPage({ onRestartTour }: { onRestartTour?: () => void }) {
  const { api } = useSettings()
  const { t } = useT()
  const notify = useToast()
  const { prefs, set, reset, mode } = usePrefs()
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

      {/* Appearance is first because it is the only section on this page that
          changes nothing on the server. Everything below it does. */}
      <Card sx={{ mb: 2 }}>
        <Box sx={{ display: 'flex', gap: 2, alignItems: 'flex-start', p: 2.5, pb: 1.5 }}>
          <Box sx={{ color: 'text.secondary', pt: .25 }}><PaletteIcon /></Box>
          <Box>
            <Box sx={{ fontWeight: 700, fontSize: 15 }}>{t('appearance.title')}</Box>
            <Box sx={{ color: 'text.secondary', fontSize: 13, mt: .5, lineHeight: 1.55 }}>{t('appearance.body')}</Box>
          </Box>
        </Box>

        <Box sx={{ px: 2.5, pb: 2 }}>
          <Box sx={{ fontSize: 12, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.07em', color: 'text.secondary', mb: 1 }}>
            {t('appearance.preset')}
          </Box>
          <Box sx={{ display: 'grid', gap: 1, gridTemplateColumns: { xs: '1fr 1fr', sm: 'repeat(3, 1fr)', md: 'repeat(6, 1fr)' } }}>
            {THEME_PRESETS.map(p => (
              <ThemePreview key={p} preset={p} mode={mode} label={t(`preset.${p}`)}
                selected={prefs.themePreset === p} onPick={() => set('themePreset', p)} />
            ))}
          </Box>
          <Box sx={{ fontSize: 12, color: 'text.secondary', mt: 1 }}>{t(`preset.${prefs.themePreset}.hint`)}</Box>
        </Box>

        <Divider />
        <Choice label={t('appearance.mode')} help={t('appearance.modeHelp')} value={prefs.colorMode}
          onChange={v => set('colorMode', v as typeof prefs.colorMode)}
          options={[
            { value: 'light', label: t('appearance.light') },
            { value: 'dark', label: t('appearance.dark') },
            { value: 'system', label: t('appearance.system') },
          ]} />
        <Divider />
        <Choice label={t('appearance.density')} help={t('appearance.densityHelp')} value={prefs.density}
          onChange={v => set('density', v as typeof prefs.density)}
          options={[
            { value: 'comfortable', label: t('appearance.comfortable') },
            { value: 'compact', label: t('appearance.compact') },
          ]} />
        <Divider />
        <Choice label={t('appearance.motion')} help={t('appearance.motionHelp')} value={String(prefs.reducedMotion)}
          onChange={v => set('reducedMotion', v === 'system' ? 'system' : v === 'true')}
          options={[
            { value: 'system', label: t('appearance.system') },
            { value: 'true', label: t('appearance.motionReduced') },
            { value: 'false', label: t('appearance.motionFull') },
          ]} />
        <Divider />
        <Row icon={<Box />} title={t('appearance.sidebar')} body={t('appearance.sidebarHelp')}
          checked={prefs.sidebarCollapsed} onChange={v => set('sidebarCollapsed', v)} />
        <Divider />
        <Box sx={{ p: 2.5, display: 'flex', gap: 1.5, flexWrap: 'wrap', alignItems: 'center' }}>
          <Button variant="outlined" onClick={() => { reset(); notify(t('appearance.resetDone'), 'info') }}>
            {t('appearance.reset')}
          </Button>
          {onRestartTour && <Button onClick={onRestartTour}>{t('tour.restart')}</Button>}
          <Box sx={{ fontSize: 12, color: 'text.secondary' }}>{t('appearance.localNote')}</Box>
        </Box>
      </Card>

      <Card sx={{ mb: 2 }}>
        <Row icon={<MailOutlineIcon />} title={t('settings.mail.title')} color="warning"
          body={<>{t('settings.mail.body')} <Link href={href('notifications')}>{t('settings.mail.log')}</Link></>}
          checked={s.notifications_enabled}
          onChange={v => apply(api.setSettings({ notifications_enabled: v }), t(v ? 'repos.notify.on' : 'repos.notify.off'), v ? 'success' : 'info')} />
        <Divider />
        <Box sx={{ px: 2.5, py: 2, fontSize: 12.5, color: 'text.secondary' }}>{t('settings.mail.prefsNote')}</Box>
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
