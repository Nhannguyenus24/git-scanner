import { useEffect, useMemo, useRef, useState } from 'react'
import Box from '@mui/material/Box'
import Dialog from '@mui/material/Dialog'
import InputBase from '@mui/material/InputBase'
import SearchIcon from '@mui/icons-material/Search'
import ShortcutHint from './ShortcutHint'
import { href, navigate } from '../hooks/useRoute'
import { IS_MAC } from '../hooks/useHotkeys'
import { usePrefs } from '../store/preferences'
import { useColorMode } from '../store/mode'
import { LANGUAGES, useT } from '../i18n'
import { THEME_PRESETS } from '../theme'

// One box for everything the dashboard can do from the keyboard.
//
// It navigates and flips local preferences; it never performs a privileged
// action the page itself would not offer. Starting a scan goes through the same
// dialog and the same API call as the button, so the server authorises it the
// same way — the palette is a shortcut to the UI, not a second route into it.
//
// The search command carries the typed text into the findings filter as `q`,
// which is a filter field. Nothing typed here is stored: there is no history,
// deliberately, because a search box that remembers is a place a pasted
// credential could come to rest.

export interface Command {
  id:      string
  label:   string
  group:   string
  hint?:   string
  keys?:   string
  run:     () => void
}

interface Props {
  open:       boolean
  onClose:    () => void
  onNewScan:  () => void
  /** Offered as "export the current view" when the open page has an exporter. */
  onExport?:  () => void
}

const NAV_PAGES = ['overview', 'analytics', 'scans', 'findings', 'repositories', 'spread', 'candidates', 'notifications', 'providers', 'settings', 'help']

export default function CommandPalette({ open, onClose, onNewScan, onExport }: Props) {
  const { t, lang, setLang } = useT()
  const { prefs, set, reset } = usePrefs()
  const { mode, toggle } = useColorMode()
  const [q, setQ] = useState('')
  const [cursor, setCursor] = useState(0)
  const listRef = useRef<HTMLDivElement>(null)

  useEffect(() => { if (open) { setQ(''); setCursor(0) } }, [open])

  const commands = useMemo<Command[]>(() => {
    const go = (page: string, params?: Record<string, string>) => () => navigate(href(page, undefined, params))
    const cmds: Command[] = [
      { id: 'scan', group: t('cmd.g.actions'), label: t('cmd.newScan'), keys: 'N', run: onNewScan },
      ...NAV_PAGES.map(p => ({ id: `go-${p}`, group: t('cmd.g.go'), label: t('cmd.goTo', { page: t(`nav.${p}`) }), run: go(p) })),
      { id: 'f-critical', group: t('cmd.g.filters'), label: t('filters.criticalOpen'), run: go('findings', { severity: 'critical', status: 'open' }) },
      { id: 'f-reachable', group: t('cmd.g.filters'), label: t('filters.reachable'), run: go('findings', { exposure: 'head', status: 'open' }) },
      { id: 'f-unmapped', group: t('cmd.g.filters'), label: t('filters.unmapped'), run: go('findings', { provider: 'unmapped', status: 'open' }) },
      ...prefs.savedFilters.map(f => ({
        id: `sf-${f.id}`, group: t('cmd.g.filters'), label: f.name, hint: t('cmd.savedFilter'),
        run: go('findings', f.params),
      })),
      { id: 'mode', group: t('cmd.g.appearance'), label: t(mode === 'dark' ? 'mode.toLight' : 'mode.toDark'), run: toggle },
      { id: 'mode-system', group: t('cmd.g.appearance'), label: t('appearance.useSystem'), run: () => set('colorMode', 'system') },
      ...THEME_PRESETS.map(p => ({
        id: `theme-${p}`, group: t('cmd.g.appearance'), label: t('cmd.theme', { name: t(`preset.${p}`) }),
        hint: prefs.themePreset === p ? t('common.current') : undefined,
        run: () => set('themePreset', p),
      })),
      {
        id: 'density', group: t('cmd.g.appearance'),
        label: t(prefs.density === 'compact' ? 'appearance.toComfortable' : 'appearance.toCompact'),
        run: () => set('density', prefs.density === 'compact' ? 'comfortable' : 'compact'),
      },
      {
        id: 'sidebar', group: t('cmd.g.appearance'),
        label: t(prefs.sidebarCollapsed ? 'appearance.expandSidebar' : 'appearance.collapseSidebar'),
        keys: '[', run: () => set('sidebarCollapsed', !prefs.sidebarCollapsed),
      },
      {
        id: 'motion', group: t('cmd.g.appearance'),
        label: t(prefs.reducedMotion === true ? 'appearance.motionOn' : 'appearance.motionOff'),
        run: () => set('reducedMotion', prefs.reducedMotion === true ? 'system' : true),
      },
      ...LANGUAGES.map(l => ({
        id: `lang-${l.code}`, group: t('cmd.g.language'), label: l.label,
        hint: lang === l.code ? t('common.current') : undefined,
        run: () => setLang(l.code),
      })),
      { id: 'reset', group: t('cmd.g.appearance'), label: t('appearance.reset'), run: reset },
    ]
    if (onExport) cmds.push({ id: 'export', group: t('cmd.g.actions'), label: t('cmd.exportView'), run: onExport })
    return cmds
  }, [t, lang, setLang, prefs, set, reset, mode, toggle, onNewScan, onExport])

  // A free-text query that matches no command is still useful: it becomes a
  // findings search, which is what people mean when they type a repository name.
  const matches = useMemo(() => {
    const needle = q.trim().toLowerCase()
    const hits = needle
      ? commands.filter(c => `${c.label} ${c.group}`.toLowerCase().includes(needle))
      : commands
    if (!needle) return hits
    return [
      {
        id: 'search', group: t('cmd.g.actions'), label: t('cmd.search', { q: q.trim() }),
        run: () => navigate(href('findings', undefined, { q: q.trim() })),
      } as Command,
      ...hits,
    ]
  }, [q, commands, t])

  useEffect(() => { setCursor(c => Math.min(c, Math.max(0, matches.length - 1))) }, [matches.length])

  const choose = (c: Command) => { onClose(); c.run() }

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown' || (e.key === 'n' && e.ctrlKey)) { e.preventDefault(); setCursor(c => (c + 1) % Math.max(1, matches.length)) }
    else if (e.key === 'ArrowUp' || (e.key === 'p' && e.ctrlKey)) { e.preventDefault(); setCursor(c => (c - 1 + matches.length) % Math.max(1, matches.length)) }
    else if (e.key === 'Enter') { e.preventDefault(); const c = matches[cursor]; if (c) choose(c) }
  }

  useEffect(() => {
    listRef.current?.querySelector('[data-active="true"]')?.scrollIntoView({ block: 'nearest' })
  }, [cursor, matches])

  let lastGroup = ''

  return (
    <Dialog open={open} onClose={onClose} fullWidth maxWidth="sm"
      PaperProps={{ sx: { position: 'fixed', top: { xs: 16, sm: 72 }, m: 0, mx: 'auto', left: 0, right: 0, maxHeight: '70vh' } }}
      aria-label={t('cmd.title')}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, px: 2, py: 1.5, borderBottom: 1, borderColor: 'divider' }}>
        <SearchIcon fontSize="small" sx={{ color: 'text.secondary' }} />
        <InputBase
          autoFocus fullWidth value={q} onChange={e => setQ(e.target.value)} onKeyDown={onKeyDown}
          placeholder={t('cmd.placeholder')} inputProps={{ 'aria-label': t('cmd.title') }}
          sx={{ fontSize: 15 }} />
        <ShortcutHint keys="Esc" />
      </Box>

      <Box ref={listRef} role="listbox" aria-label={t('cmd.title')} sx={{ overflowY: 'auto', py: .5 }}>
        {!matches.length && (
          <Box sx={{ px: 2, py: 3, color: 'text.secondary', fontSize: 13 }}>{t('cmd.noMatch')}</Box>
        )}
        {matches.map((c, i) => {
          const header = c.group !== lastGroup ? (lastGroup = c.group) : null
          return (
            <Box key={c.id}>
              {header && (
                <Box sx={{
                  px: 2, pt: 1.25, pb: .5, fontSize: 10, fontWeight: 700, letterSpacing: '.1em',
                  textTransform: 'uppercase', color: 'text.disabled',
                }}>{header}</Box>
              )}
              <Box
                role="option" aria-selected={i === cursor} data-active={i === cursor}
                onMouseMove={() => setCursor(i)} onClick={() => choose(c)}
                sx={{
                  display: 'flex', alignItems: 'center', gap: 1, px: 2, py: 1, cursor: 'pointer', fontSize: 13.5,
                  bgcolor: i === cursor ? 'action.selected' : 'transparent',
                }}>
                <Box sx={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{c.label}</Box>
                {c.hint && <Box sx={{ fontSize: 11, color: 'text.disabled' }}>{c.hint}</Box>}
                {c.keys && <ShortcutHint keys={c.keys} />}
              </Box>
            </Box>
          )
        })}
      </Box>

      <Box sx={{
        px: 2, py: 1, borderTop: 1, borderColor: 'divider', display: 'flex', gap: 1.5,
        fontSize: 11, color: 'text.disabled', alignItems: 'center', flexWrap: 'wrap',
      }}>
        <ShortcutHint keys="↑↓" /> {t('cmd.move')}
        <ShortcutHint keys="↵" /> {t('cmd.select')}
        <ShortcutHint keys={IS_MAC ? '⌘K' : 'Ctrl+K'} /> {t('cmd.title')}
      </Box>
    </Dialog>
  )
}
