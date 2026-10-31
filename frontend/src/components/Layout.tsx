import React, { useState } from 'react'
import Badge from '@mui/material/Badge'
import Box from '@mui/material/Box'
import BottomNavigation from '@mui/material/BottomNavigation'
import BottomNavigationAction from '@mui/material/BottomNavigationAction'
import Button from '@mui/material/Button'
import Drawer from '@mui/material/Drawer'
import IconButton from '@mui/material/IconButton'
import List from '@mui/material/List'
import ListItemButton from '@mui/material/ListItemButton'
import ListItemIcon from '@mui/material/ListItemIcon'
import ListItemText from '@mui/material/ListItemText'
import Tooltip from '@mui/material/Tooltip'
import DashboardIcon from '@mui/icons-material/SpaceDashboardOutlined'
import InsightsIcon from '@mui/icons-material/InsightsOutlined'
import RadarIcon from '@mui/icons-material/RadarOutlined'
import KeyIcon from '@mui/icons-material/VpnKeyOutlined'
import HubIcon from '@mui/icons-material/HubOutlined'
import FolderIcon from '@mui/icons-material/FolderOutlined'
import HelpIcon from '@mui/icons-material/HelpOutlineOutlined'
import ShieldIcon from '@mui/icons-material/VerifiedUserOutlined'
import BookIcon from '@mui/icons-material/MenuBookOutlined'
import MailIcon from '@mui/icons-material/MailOutline'
import SettingsIcon from '@mui/icons-material/SettingsOutlined'
import AddIcon from '@mui/icons-material/Add'
import MenuIcon from '@mui/icons-material/MenuOutlined'
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'
import TopBar from './TopBar'
import { href } from '../hooks/useRoute'
import { IS_MAC } from '../hooks/useHotkeys'
import { usePrefs } from '../store/preferences'
import { useT } from '../i18n'

const NAV = [
  { page: 'overview',     icon: <DashboardIcon fontSize="small" /> },
  { page: 'analytics',    icon: <InsightsIcon fontSize="small" /> },
  { page: 'scans',        icon: <RadarIcon fontSize="small" /> },
  { page: 'findings',     icon: <KeyIcon fontSize="small" /> },
  { page: 'repositories', icon: <FolderIcon fontSize="small" /> },
  { page: 'spread',       icon: <HubIcon fontSize="small" /> },
  { page: 'candidates',   icon: <HelpIcon fontSize="small" /> },
  { page: 'notifications', icon: <MailIcon fontSize="small" /> },
  { page: 'providers',    icon: <ShieldIcon fontSize="small" /> },
  { page: 'settings',     icon: <SettingsIcon fontSize="small" /> },
  { page: 'help',         icon: <BookIcon fontSize="small" /> },
]

// The five that earn a place on a phone's bottom bar; the rest live behind More.
const PRIMARY = ['overview', 'findings', 'scans', 'repositories']

interface Props {
  page: string
  onNewScan: () => void
  onCommands?: () => void
  badges?: Record<string, number>
  children: React.ReactNode
}

export default function Layout({ page, onNewScan, onCommands, badges = {}, children }: Props) {
  const { t } = useT()
  const { prefs, set } = usePrefs()
  const [moreOpen, setMoreOpen] = useState(false)
  const collapsed = prefs.sidebarCollapsed
  const width = collapsed ? 60 : 210

  const navItem = (n: typeof NAV[number]) => {
    const selected = page === n.page
    const count = badges[n.page]
    const item = (
      <ListItemButton
        key={n.page} component="a" href={href(n.page)} selected={selected}
        aria-current={selected ? 'page' : undefined}
        sx={{ justifyContent: collapsed ? 'center' : 'flex-start', px: collapsed ? 1 : 2 }}>
        <ListItemIcon sx={{ minWidth: collapsed ? 0 : 32, color: selected ? 'primary.main' : 'text.secondary' }}>
          {/* Collapsed, the count has nowhere to sit but the icon. */}
          {collapsed && count ? <Badge color="primary" variant="dot">{n.icon}</Badge> : n.icon}
        </ListItemIcon>
        {!collapsed && (
          <>
            <ListItemText primary={t(`nav.${n.page}`)} primaryTypographyProps={{ fontSize: 13.5, fontWeight: selected ? 700 : 500 }} />
            {!!count && (
              <Box sx={{ fontSize: 11, fontWeight: 700, px: .75, borderRadius: 1, bgcolor: 'divider', color: 'text.secondary' }}>{count}</Box>
            )}
          </>
        )}
      </ListItemButton>
    )
    return collapsed
      ? <Tooltip key={n.page} title={count ? `${t(`nav.${n.page}`)} · ${count}` : t(`nav.${n.page}`)} placement="right">{item}</Tooltip>
      : item
  }

  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <TopBar onCommands={onCommands} />
      <Box sx={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
        <Box component="nav" aria-label={t('nav.primary')} sx={{
          width, flexShrink: 0, borderRight: 1, borderColor: 'divider', p: collapsed ? .75 : 1.5,
          display: { xs: 'none', md: 'flex' }, flexDirection: 'column', gap: 1.5, bgcolor: 'background.paper',
          transition: 'width .18s',
        }}>
          {collapsed ? (
            <Tooltip title={t('nav.newScan')} placement="right">
              <IconButton color="primary" onClick={onNewScan} aria-label={t('nav.newScan')}
                sx={{ border: 1, borderColor: 'primary.main', borderRadius: 1 }}>
                <AddIcon />
              </IconButton>
            </Tooltip>
          ) : (
            <Button variant="contained" startIcon={<AddIcon />} onClick={onNewScan} fullWidth>{t('nav.newScan')}</Button>
          )}

          <List dense disablePadding sx={{ display: 'flex', flexDirection: 'column', gap: .25 }}>
            {NAV.map(navItem)}
          </List>

          <Box sx={{ flex: 1 }} />
          <Tooltip title={`${t(collapsed ? 'appearance.expandSidebar' : 'appearance.collapseSidebar')} · ${IS_MAC ? '⌘[' : 'Ctrl+['}`} placement="right">
            <IconButton size="small" onClick={() => set('sidebarCollapsed', !collapsed)}
              aria-label={t(collapsed ? 'appearance.expandSidebar' : 'appearance.collapseSidebar')}
              sx={{ alignSelf: collapsed ? 'center' : 'flex-end', color: 'text.secondary' }}>
              {collapsed ? <ChevronRightIcon fontSize="small" /> : <ChevronLeftIcon fontSize="small" />}
            </IconButton>
          </Tooltip>
        </Box>

        <Box sx={{ flex: 1, overflow: 'auto', minWidth: 0 }}>
          <Box sx={{ p: { xs: 2, md: 3 }, pb: { xs: 10, md: 3 }, maxWidth: 1400, mx: 'auto' }}>{children}</Box>
          <Footer />
        </Box>
      </Box>

      {/* Phones get a fixed bottom bar instead of the old scrolling tab strip:
          thumbs reach the bottom of a screen, and a strip that scrolls hides
          half its destinations behind a gesture nobody discovers. */}
      <Box sx={{
        display: { xs: 'block', md: 'none' }, position: 'fixed', bottom: 0, left: 0, right: 0, zIndex: 1200,
        borderTop: 1, borderColor: 'divider', bgcolor: 'background.paper',
      }}>
        <BottomNavigation showLabels value={PRIMARY.includes(page) ? page : 'more'} sx={{ bgcolor: 'transparent', height: 58 }}>
          {NAV.filter(n => PRIMARY.includes(n.page)).map(n => (
            <BottomNavigationAction key={n.page} value={n.page} component="a" href={href(n.page)}
              label={t(`nav.${n.page}`)}
              icon={badges[n.page] ? <Badge color="error" badgeContent={badges[n.page]} max={99}>{n.icon}</Badge> : n.icon}
              sx={{ minWidth: 0, fontSize: 10 }} />
          ))}
          <BottomNavigationAction value="more" label={t('nav.more')} icon={<MenuIcon fontSize="small" />}
            onClick={() => setMoreOpen(true)} sx={{ minWidth: 0 }} />
        </BottomNavigation>
      </Box>

      <Drawer anchor="bottom" open={moreOpen} onClose={() => setMoreOpen(false)}
        PaperProps={{ sx: { borderTopLeftRadius: 8, borderTopRightRadius: 8, pb: 2 } }}>
        <Box sx={{ p: 2 }}>
          <Button variant="contained" fullWidth startIcon={<AddIcon />}
            onClick={() => { setMoreOpen(false); onNewScan() }}>{t('nav.newScan')}</Button>
        </Box>
        <List dense sx={{ px: 1 }}>
          {NAV.filter(n => !PRIMARY.includes(n.page)).map(n => (
            <ListItemButton key={n.page} component="a" href={href(n.page)} selected={page === n.page}
              onClick={() => setMoreOpen(false)}>
              <ListItemIcon sx={{ minWidth: 34, color: page === n.page ? 'primary.main' : 'text.secondary' }}>{n.icon}</ListItemIcon>
              <ListItemText primary={t(`nav.${n.page}`)} />
              {!!badges[n.page] && <Box sx={{ fontSize: 11, fontWeight: 700 }}>{badges[n.page]}</Box>}
            </ListItemButton>
          ))}
        </List>
      </Drawer>
    </Box>
  )
}

// Footer is a deliberately faint line at the bottom of every page: a copyright
// and the build the bundle was stamped with (vite.config.ts), so a screenshot or
// a bug report carries the version without anyone having to ask.
function Footer() {
  const { t } = useT()
  const year = new Date().getFullYear()
  const build = [`v${__APP_VERSION__}`, __APP_COMMIT__, __BUILD_DATE__].filter(Boolean).join(' · ')
  return (
    <Box
      component="footer"
      sx={{
        px: { xs: 2, md: 3 }, py: 2, mt: 2, textAlign: 'center',
        color: 'text.disabled', fontSize: 11, opacity: 0.6,
        userSelect: 'none', borderTop: 1, borderColor: 'divider',
      }}
    >
      © {year} GitScanner · {t('footer.rights')}
      <Box component="span" sx={{ mx: 0.75 }}>·</Box>
      <Box component="span" sx={{ fontFamily: '"IBM Plex Mono", monospace' }}>{build}</Box>
    </Box>
  )
}

export function PageHeader({ title, subtitle, actions }: { title: React.ReactNode; subtitle?: React.ReactNode; actions?: React.ReactNode }) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'flex-end', justifyContent: 'space-between', gap: 2, mb: 2.5, flexWrap: 'wrap' }}>
      <Box>
        <Box component="h1" sx={{ m: 0, fontSize: 22, fontWeight: 800, letterSpacing: '-.01em' }}>{title}</Box>
        {subtitle && <Box sx={{ color: 'text.secondary', fontSize: 13, mt: .25 }}>{subtitle}</Box>}
      </Box>
      {actions && <Box sx={{ display: 'flex', gap: 1, alignItems: 'center', flexWrap: 'wrap' }}>{actions}</Box>}
    </Box>
  )
}

