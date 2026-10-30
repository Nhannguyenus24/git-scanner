import Box from '@mui/material/Box'
import Card from '@mui/material/Card'
import IconButton from '@mui/material/IconButton'
import Tooltip from '@mui/material/Tooltip'
import ArrowUpIcon from '@mui/icons-material/ArrowUpward'
import ArrowDownIcon from '@mui/icons-material/ArrowDownward'
import HideIcon from '@mui/icons-material/VisibilityOffOutlined'
import { CornerTicks } from './Instrument'
import { useT } from '../i18n'

// A dashboard panel that can be moved and hidden with buttons.
//
// Buttons, not drag-and-drop, on purpose: move-up / move-down works from the
// keyboard, reads correctly to a screen reader, and needs no pointer. The widget
// model has to be stable before a drag layer is worth adding on top of it, and
// a drag layer that is the *only* way to reorder would lock out exactly the
// users the accessibility baseline is for.

interface Props {
  title:     string
  /** Shown only while the dashboard is in edit mode. */
  editing?:  boolean
  onUp?:     () => void
  onDown?:   () => void
  onHide?:   () => void
  actions?:  React.ReactNode
  /** Renders the children bare, for a widget that is its own card (or a row of them). */
  bare?:     boolean
  children:  React.ReactNode
}

export default function DashboardWidget({ title, editing, onUp, onDown, onHide, actions, bare, children }: Props) {
  const { t } = useT()

  const controls = editing && (
    <Box sx={{ display: 'flex', gap: .25 }}>
      <Tooltip title={t('dash.moveUp')}>
        <span><IconButton size="small" onClick={onUp} disabled={!onUp} aria-label={t('dash.moveUp')}><ArrowUpIcon sx={{ fontSize: 16 }} /></IconButton></span>
      </Tooltip>
      <Tooltip title={t('dash.moveDown')}>
        <span><IconButton size="small" onClick={onDown} disabled={!onDown} aria-label={t('dash.moveDown')}><ArrowDownIcon sx={{ fontSize: 16 }} /></IconButton></span>
      </Tooltip>
      <Tooltip title={t('dash.hide')}>
        <span><IconButton size="small" onClick={onHide} disabled={!onHide} aria-label={t('dash.hide')}><HideIcon sx={{ fontSize: 16 }} /></IconButton></span>
      </Tooltip>
    </Box>
  )

  if (bare) {
    return (
      <Box sx={{ position: 'relative' }}>
        {editing && (
          <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: .75 }}>
            <Box sx={{ fontSize: 11, fontWeight: 700, textTransform: 'uppercase', letterSpacing: '.07em', color: 'text.secondary' }}>{title}</Box>
            {controls}
          </Box>
        )}
        {children}
      </Box>
    )
  }

  return (
    <Card sx={{ p: 2, height: '100%', display: 'flex', flexDirection: 'column', position: 'relative' }}>
      <CornerTicks />
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 1, mb: 1.5 }}>
        <Box sx={{ fontWeight: 700 }}>{title}</Box>
        {controls ?? actions}
      </Box>
      <Box sx={{ flex: 1, minHeight: 0 }}>{children}</Box>
    </Card>
  )
}
