import { useState, type MouseEvent } from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import DialogTitle from '@mui/material/DialogTitle'
import Divider from '@mui/material/Divider'
import IconButton from '@mui/material/IconButton'
import ListItemText from '@mui/material/ListItemText'
import Menu from '@mui/material/Menu'
import MenuItem from '@mui/material/MenuItem'
import TextField from '@mui/material/TextField'
import Tooltip from '@mui/material/Tooltip'
import BookmarkIcon from '@mui/icons-material/BookmarkBorder'
import DeleteIcon from '@mui/icons-material/DeleteOutline'
import EditIcon from '@mui/icons-material/EditOutlined'
import { sanitiseFilter, usePrefs, type SavedFilter } from '../store/preferences'
import { useToast } from '../store/toast'
import { useT } from '../i18n'

// Saved filters are local to the browser, and they hold filter fields only —
// sanitiseFilter drops everything else, so a saved view can never become a
// place where a pasted credential is persisted. Team-wide saved filters would
// need a server endpoint and are not faked here.

/** The filters offered out of the box, as URL parameters. */
export const FILTER_PRESETS: { key: string; params: Record<string, string> }[] = [
  { key: 'filters.criticalOpen', params: { severity: 'critical', status: 'open' } },
  { key: 'filters.reachable',    params: { exposure: 'head', status: 'open' } },
  { key: 'filters.live',         params: { exposure: 'live', status: 'open' } },
  { key: 'filters.unmapped',     params: { provider: 'unmapped', status: 'open' } },
  { key: 'filters.recent',       params: { status: 'open', sort: 'recent' } },
  { key: 'filters.stale',        params: { status: 'open', sort: 'oldest' } },
]

interface Props {
  /** The filter showing right now, saved verbatim when asked. */
  current: Record<string, string>
  apply:   (params: Record<string, string>) => void
}

export default function SavedFilterMenu({ current, apply }: Props) {
  const { t } = useT()
  const { prefs, set } = usePrefs()
  const notify = useToast()
  const [anchor, setAnchor] = useState<null | HTMLElement>(null)
  const [editing, setEditing] = useState<SavedFilter | null>(null)
  const [name, setName] = useState('')
  const close = () => setAnchor(null)

  const saved = prefs.savedFilters
  const live = sanitiseFilter(current)
  const hasFilter = Object.keys(live).length > 0

  const startSave = () => {
    close()
    setEditing({ id: `f${Date.now().toString(36)}`, name: '', params: live })
    setName('')
  }

  const startRename = (f: SavedFilter) => { close(); setEditing(f); setName(f.name) }

  const commit = () => {
    if (!editing) return
    const clean = name.trim().slice(0, 60)
    if (!clean) return
    const next = saved.some(f => f.id === editing.id)
      ? saved.map(f => (f.id === editing.id ? { ...f, name: clean } : f))
      : [...saved, { ...editing, name: clean }].slice(0, 50)
    set('savedFilters', next)
    notify(t('filters.saved', { name: clean }))
    setEditing(null)
  }

  const remove = (f: SavedFilter) => {
    set('savedFilters', saved.filter(x => x.id !== f.id))
    notify(t('filters.deleted', { name: f.name }), 'info')
  }

  return (
    <>
      <Button size="small" startIcon={<BookmarkIcon />} onClick={(e: MouseEvent<HTMLElement>) => setAnchor(e.currentTarget)}
        aria-haspopup="menu">
        {t('filters.menu')}
      </Button>

      <Menu anchorEl={anchor} open={!!anchor} onClose={close} slotProps={{ paper: { sx: { minWidth: 280 } } }}>
        <MenuItem disabled sx={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '.07em' }}>{t('filters.presets')}</MenuItem>
        {FILTER_PRESETS.map(p => (
          <MenuItem key={p.key} onClick={() => { close(); apply(p.params) }}>
            <ListItemText primary={t(p.key)} secondary={t(`${p.key}.hint`)} />
          </MenuItem>
        ))}

        <Divider />
        <MenuItem disabled sx={{ fontSize: 11, textTransform: 'uppercase', letterSpacing: '.07em' }}>{t('filters.yours')}</MenuItem>
        {!saved.length && (
          <MenuItem disabled><ListItemText secondary={t('filters.noneSaved')} /></MenuItem>
        )}
        {saved.map(f => (
          <MenuItem key={f.id} onClick={() => { close(); apply(f.params) }} sx={{ pr: 1 }}>
            <ListItemText primary={f.name} secondary={Object.entries(f.params).map(([k, v]) => `${k}=${v}`).join(' · ') || t('filters.all')} />
            <Box sx={{ display: 'flex', ml: 1 }} onClick={e => e.stopPropagation()}>
              <Tooltip title={t('filters.rename')}>
                <IconButton size="small" onClick={() => startRename(f)}><EditIcon sx={{ fontSize: 16 }} /></IconButton>
              </Tooltip>
              <Tooltip title={t('common.delete')}>
                <IconButton size="small" onClick={() => remove(f)}><DeleteIcon sx={{ fontSize: 16 }} /></IconButton>
              </Tooltip>
            </Box>
          </MenuItem>
        ))}

        <Divider />
        <MenuItem onClick={startSave} disabled={!hasFilter}>
          <ListItemText primary={t('filters.saveCurrent')} secondary={hasFilter ? undefined : t('filters.saveNothing')} />
        </MenuItem>
        <MenuItem onClick={() => { close(); apply({}) }}><ListItemText primary={t('common.clearFilters')} /></MenuItem>
      </Menu>

      <Dialog open={!!editing} onClose={() => setEditing(null)} fullWidth maxWidth="xs">
        <DialogTitle sx={{ fontSize: 17, fontWeight: 700 }}>
          {saved.some(f => f.id === editing?.id) ? t('filters.rename') : t('filters.saveCurrent')}
        </DialogTitle>
        <DialogContent>
          <TextField autoFocus fullWidth label={t('filters.name')} value={name} sx={{ mt: 1 }}
            onChange={e => setName(e.target.value)}
            onKeyDown={e => { if (e.key === 'Enter') { e.preventDefault(); commit() } }} />
          <Box sx={{ mt: 1.5, fontSize: 12, color: 'text.secondary' }}>
            {Object.entries(editing?.params ?? {}).map(([k, v]) => `${k}=${v}`).join(' · ') || t('filters.all')}
          </Box>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setEditing(null)} color="inherit">{t('common.cancel')}</Button>
          <Button onClick={commit} variant="contained" disabled={!name.trim()}>{t('common.save')}</Button>
        </DialogActions>
      </Dialog>
    </>
  )
}
