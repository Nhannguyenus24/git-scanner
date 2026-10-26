import React from 'react'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import List from '@mui/material/List'
import ListItemButton from '@mui/material/ListItemButton'
import ListItemIcon from '@mui/material/ListItemIcon'
import ListItemText from '@mui/material/ListItemText'
import DashboardIcon from '@mui/icons-material/SpaceDashboardOutlined'
import InsightsIcon from '@mui/icons-material/InsightsOutlined'
import RadarIcon from '@mui/icons-material/RadarOutlined'
import KeyIcon from '@mui/icons-material/VpnKeyOutlined'
import FolderIcon from '@mui/icons-material/FolderOutlined'
import HelpIcon from '@mui/icons-material/HelpOutlineOutlined'
import ShieldIcon from '@mui/icons-material/VerifiedUserOutlined'
import BookIcon from '@mui/icons-material/MenuBookOutlined'
import AddIcon from '@mui/icons-material/Add'
import TopBar from './TopBar'
import { href } from '../hooks/useRoute'
import { useT } from '../i18n'

const NAV = [
  { page: 'overview',     icon: <DashboardIcon fontSize="small" /> },
  { page: 'analytics',    icon: <InsightsIcon fontSize="small" /> },
  { page: 'scans',        icon: <RadarIcon fontSize="small" /> },
  { page: 'findings',     icon: <KeyIcon fontSize="small" /> },
  { page: 'repositories', icon: <FolderIcon fontSize="small" /> },
  { page: 'candidates',   icon: <HelpIcon fontSize="small" /> },
  { page: 'providers',    icon: <ShieldIcon fontSize="small" /> },
  { page: 'help',         icon: <BookIcon fontSize="small" /> },
]

interface Props { page: string; onNewScan: () => void; badges?: Record<string, number>; children: React.ReactNode }

export default function Layout({ page, onNewScan, badges = {}, children }: Props) {
  const { t } = useT()
  return (
    <Box sx={{ display: 'flex', flexDirection: 'column', height: '100vh' }}>
      <TopBar />
      <Box sx={{ display: 'flex', flex: 1, overflow: 'hidden' }}>
        <Box component="nav" sx={{
          width: 210, flexShrink: 0, borderRight: 1, borderColor: 'divider', p: 1.5,
          display: { xs: 'none', md: 'flex' }, flexDirection: 'column', gap: 1.5, bgcolor: 'background.paper',
        }}>
          <Button variant="contained" startIcon={<AddIcon />} onClick={onNewScan} fullWidth>{t('nav.newScan')}</Button>
          <List dense disablePadding sx={{ display: 'flex', flexDirection: 'column', gap: .25 }}>
            {NAV.map(n => (
              <ListItemButton key={n.page} component="a" href={href(n.page)} selected={page === n.page}>
                <ListItemIcon sx={{ minWidth: 32, color: page === n.page ? 'primary.main' : 'text.secondary' }}>{n.icon}</ListItemIcon>
                <ListItemText primary={t(`nav.${n.page}`)} primaryTypographyProps={{ fontSize: 13.5, fontWeight: page === n.page ? 700 : 500 }} />
                {!!badges[n.page] && (
                  <Box sx={{ fontSize: 11, fontWeight: 700, px: .75, borderRadius: 1, bgcolor: 'divider', color: 'text.secondary' }}>
                    {badges[n.page]}
                  </Box>
                )}
              </ListItemButton>
            ))}
          </List>
        </Box>

        {/* Narrow screens get the same pages as a scrolling tab strip. */}
        <Box sx={{ flex: 1, overflow: 'auto', minWidth: 0 }}>
          <Box sx={{ display: { xs: 'flex', md: 'none' }, gap: .5, p: 1, overflowX: 'auto', borderBottom: 1, borderColor: 'divider' }}>
            {NAV.map(n => (
              <Button key={n.page} size="small" href={href(n.page)} variant={page === n.page ? 'contained' : 'text'} color={page === n.page ? 'primary' : 'inherit'}>
                {t(`nav.${n.page}`)}
              </Button>
            ))}
            <Button size="small" onClick={onNewScan} startIcon={<AddIcon />}>{t('nav.scan')}</Button>
          </Box>
          <Box sx={{ p: { xs: 2, md: 3 }, maxWidth: 1400, mx: 'auto' }}>{children}</Box>
          <Footer />
        </Box>
      </Box>
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
      {actions && <Box sx={{ display: 'flex', gap: 1, alignItems: 'center' }}>{actions}</Box>}
    </Box>
  )
}
