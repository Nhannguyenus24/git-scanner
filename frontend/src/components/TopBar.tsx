import { useState, type MouseEvent } from 'react'
import AppBar from '@mui/material/AppBar'
import Box from '@mui/material/Box'
import IconButton from '@mui/material/IconButton'
import Menu from '@mui/material/Menu'
import MenuItem from '@mui/material/MenuItem'
import TextField from '@mui/material/TextField'
import Toolbar from '@mui/material/Toolbar'
import Tooltip from '@mui/material/Tooltip'
import Typography from '@mui/material/Typography'
import DarkIcon from '@mui/icons-material/DarkModeOutlined'
import HelpIcon from '@mui/icons-material/HelpOutlineOutlined'
import LightIcon from '@mui/icons-material/LightModeOutlined'
import SearchIcon from '@mui/icons-material/Search'
import ShortcutHint from './ShortcutHint'
import { useApiStatus, type ApiStatus } from '../hooks/useApiStatus'
import { href } from '../hooks/useRoute'
import { IS_MAC } from '../hooks/useHotkeys'
import { useSettings } from '../store/settings'
import { useColorMode } from '../store/mode'
import { useT, LANGUAGES } from '../i18n'

const STATUS_COLOR: Record<ApiStatus, 'text.disabled' | 'success.main' | 'error.main'> =
  { checking: 'text.disabled', ok: 'success.main', error: 'error.main' }

export default function TopBar({ onCommands }: { onCommands?: () => void }) {
  const { baseUrl, setBaseUrl } = useSettings()
  const { mode, toggle } = useColorMode()
  const { t, lang, setLang } = useT()
  const status = useApiStatus(baseUrl)
  const statusLabel = { checking: t('api.checking'), ok: t('api.ok'), error: t('api.error') }[status]
  const [langAnchor, setLangAnchor] = useState<null | HTMLElement>(null)

  return (
    <AppBar position="sticky" elevation={0}>
      <Toolbar sx={{ gap: 1.5, minHeight: '52px !important', px: 2 }}>
        <Box component="a" href={href('overview')} sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'inherit', textDecoration: 'none' }}>
          <Box sx={{
            width: 28, height: 28, color: '#fff', bgcolor: 'primary.main', borderRadius: '2px',
            display: 'flex', alignItems: 'center', justifyContent: 'center',
            fontFamily: '"IBM Plex Mono", monospace', fontSize: 11, fontWeight: 700,
          }}>GS</Box>
          <Box sx={{ lineHeight: 1 }}>
            <Typography sx={{ fontFamily: '"IBM Plex Mono", monospace', fontWeight: 700, fontSize: 15 }}>GitScanner</Typography>
            <Typography sx={{ fontSize: 9.5, color: 'text.secondary', letterSpacing: '.14em', textTransform: 'uppercase' }}>{t('brand.tagline')}</Typography>
          </Box>
        </Box>
        <Box sx={{ flex: 1 }} />

        {/* The palette gets a visible target as well as a shortcut: a feature
            reachable only by a key combination is a feature most people never
            learn exists. */}
        {onCommands && (
          <Box
            role="button" tabIndex={0} onClick={onCommands}
            onKeyDown={e => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); onCommands() } }}
            aria-label={t('cmd.title')}
            sx={{
              display: { xs: 'none', sm: 'flex' }, alignItems: 'center', gap: 1, cursor: 'pointer',
              px: 1.25, py: .6, borderRadius: 1, border: 1, borderColor: 'divider', color: 'text.secondary',
              '&:hover': { borderColor: 'text.secondary' },
            }}>
            <SearchIcon sx={{ fontSize: 16 }} />
            <Box sx={{ fontSize: 12.5 }}>{t('cmd.title')}</Box>
            <ShortcutHint keys={IS_MAC ? '⌘K' : 'Ctrl+K'} />
          </Box>
        )}

        <Tooltip title={statusLabel}>
          <TextField
            value={baseUrl}
            onChange={e => setBaseUrl(e.target.value)}
            placeholder="http://localhost:8080"
            inputProps={{ 'aria-label': t('settings.apiUrl') }}
            sx={{ width: { xs: 140, sm: 230 }, '& input': { fontFamily: '"IBM Plex Mono", monospace', fontSize: 12 } }}
            InputProps={{ startAdornment: <Box component="span" sx={{ mr: 1, fontSize: 10, color: STATUS_COLOR[status] }}>●</Box> }}
          />
        </Tooltip>

        {onCommands && (
          <Tooltip title={t('cmd.title')}>
            <IconButton size="small" onClick={onCommands} aria-label={t('cmd.title')}
              sx={{ display: { xs: 'inline-flex', sm: 'none' }, color: 'text.secondary' }}>
              <SearchIcon fontSize="small" />
            </IconButton>
          </Tooltip>
        )}

        <Tooltip title={t('nav.help')}>
          <IconButton size="small" component="a" href={href('help')} aria-label={t('nav.help')} sx={{ color: 'text.secondary' }}>
            <HelpIcon fontSize="small" />
          </IconButton>
        </Tooltip>
        <Tooltip title={t('lang.switch')}>
          <IconButton
            size="small"
            onClick={(e: MouseEvent<HTMLElement>) => setLangAnchor(e.currentTarget)}
            aria-label={t('lang.switch')}
            sx={{
              fontFamily: '"IBM Plex Mono", monospace',
              fontSize: 12,
              fontWeight: 700,
              width: 34,
              height: 34,
              borderRadius: 1,
              color: 'text.secondary',
            }}
          >
            {lang.toUpperCase()}
          </IconButton>
        </Tooltip>
        <Menu
          anchorEl={langAnchor}
          open={Boolean(langAnchor)}
          onClose={() => setLangAnchor(null)}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
          transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        >
          {LANGUAGES.map(l => (
            <MenuItem
              key={l.code}
              selected={lang === l.code}
              onClick={() => {
                setLang(l.code)
                setLangAnchor(null)
              }}
              sx={{ fontSize: 13, display: 'flex', gap: 1.5, minWidth: 140 }}
            >
              <Box
                component="span"
                sx={{
                  fontFamily: '"IBM Plex Mono", monospace',
                  fontSize: 11,
                  fontWeight: 700,
                  width: 22,
                  color: 'text.secondary',
                }}
              >
                {l.code.toUpperCase()}
              </Box>
              <Box component="span">{l.label}</Box>
            </MenuItem>
          ))}
        </Menu>
        <Tooltip title={mode === 'dark' ? t('mode.toLight') : t('mode.toDark')}>
          <IconButton size="small" onClick={toggle} aria-label={mode === 'dark' ? t('mode.toLight') : t('mode.toDark')}
            sx={{ color: 'text.secondary' }}>
            {mode === 'dark' ? <LightIcon fontSize="small" /> : <DarkIcon fontSize="small" />}
          </IconButton>
        </Tooltip>
      </Toolbar>
    </AppBar>
  )
}
