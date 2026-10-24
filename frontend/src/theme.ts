import { createTheme, type Theme } from '@mui/material/styles'
import type { Severity } from './types'

// Vintage paper as a technical instrument, in two stocks: a light aged sheet and
// a dark one (old terminal / carbon paper). Both keep petrol-teal and oxblood
// ink, monospaced labels, hairline and double rules, and no drop shadows.
//
// Chart colours live on the theme (theme.chart) so they switch with the mode and
// are re-stepped for each surface — a dark palette is selected, not an inverted
// light one. Both palettes pass the dataviz validator on their surface.

export type Mode = 'light' | 'dark'

export interface ChartPalette {
  categorical: string[]
  other:       string
  severity:    Record<Severity, string>
  surface:     string
  inset:       string
  line:        string
  lineStrong:  string
  text:        string
  textSoft:    string
  textFaint:   string
  paper:       string
}

// Augment the MUI theme so theme.chart is typed everywhere useTheme() is used.
declare module '@mui/material/styles' {
  interface Theme { chart: ChartPalette }
  interface ThemeOptions { chart?: ChartPalette }
}

const light = {
  paper: '#f3efe3', card: '#faf7ee', cardInset: '#efeadb', line: '#ddd5c0', lineStrong: '#c6bda2',
  primary: '#1f6f66', oxblood: '#9c3b2e', amber: '#a9781a', indigo: '#2f4ba8', success: '#3f7d57',
  text: '#2c2823', textSoft: '#6b6353', textFaint: '#938a76',
  categorical: ['#0a8a70', '#c0392b', '#2f4ba8', '#a9781a', '#8e3b6b', '#5f7a1f'],
  severity: { critical: '#9c3b2e', high: '#c2622a', medium: '#a9781a', low: '#2f4ba8', info: '#8a8372' } as Record<Severity, string>,
}

const dark = {
  paper: '#17150f', card: '#211d15', cardInset: '#14120c', line: '#39342699', lineStrong: '#4d4634',
  primary: '#3fb0a0', oxblood: '#e2583f', amber: '#cf9a2e', indigo: '#8ea0e8', success: '#5fb07f',
  text: '#e9e2cf', textSoft: '#a79d84', textFaint: '#7d745f',
  categorical: ['#1f9e8b', '#e2583f', '#6277d8', '#ad8324', '#c05f94', '#7f9a35'],
  severity: { critical: '#e2583f', high: '#e8893f', medium: '#cf9a2e', low: '#6277d8', info: '#9a9078' } as Record<Severity, string>,
}

// ink is the current light token set, kept for any non-themed reference.
export const ink = light

export function makeTheme(mode: Mode): Theme {
  const c = mode === 'dark' ? dark : light
  const mono = '"IBM Plex Mono", ui-monospace, "SFMono-Regular", Menlo, monospace'
  const sans = '"IBM Plex Sans", system-ui, -apple-system, "Helvetica Neue", sans-serif'

  return createTheme({
    palette: {
      mode,
      primary:    { main: c.primary },
      secondary:  { main: c.indigo },
      success:    { main: c.success },
      warning:    { main: c.amber },
      error:      { main: c.oxblood },
      background: { default: c.paper, paper: c.card },
      text:       { primary: c.text, secondary: c.textSoft },
      divider:    c.line,
      action:     {
        hover: mode === 'dark' ? 'rgba(63,176,160,.10)' : 'rgba(31,111,102,.06)',
        selected: mode === 'dark' ? 'rgba(63,176,160,.16)' : 'rgba(31,111,102,.10)',
      },
    },
    chart: {
      categorical: c.categorical, other: c.textFaint, severity: c.severity,
      surface: c.card, inset: c.cardInset, line: c.line, lineStrong: c.lineStrong,
      text: c.text, textSoft: c.textSoft, textFaint: c.textFaint, paper: c.paper,
    },
    shape: { borderRadius: 3 },
    typography: {
      fontFamily: sans,
      h1: { fontFamily: mono }, h2: { fontFamily: mono }, h3: { fontFamily: mono },
      h4: { fontFamily: mono }, h5: { fontFamily: mono }, h6: { fontFamily: mono },
      button: { textTransform: 'none', fontWeight: 600, letterSpacing: '.01em' },
      overline: { fontFamily: mono, letterSpacing: '.14em' },
    },
    components: {
      MuiPaper: { styleOverrides: { root: { backgroundImage: 'none' } } },
      MuiCard: {
        defaultProps: { elevation: 0 },
        styleOverrides: { root: {
          backgroundImage: 'none', backgroundColor: c.card,
          border: `1px solid ${c.line}`, borderRadius: 3, boxShadow: `0 1px 0 ${c.line}`,
        } },
      },
      MuiAppBar: {
        styleOverrides: { root: {
          backgroundImage: 'none', color: c.text, backgroundColor: c.card,
          borderBottom: `1px solid ${c.lineStrong}`, boxShadow: 'none',
        } },
      },
      MuiTableCell: {
        styleOverrides: {
          root: { borderColor: c.line },
          head: {
            backgroundColor: c.cardInset, fontFamily: mono, fontWeight: 600,
            fontSize: 10.5, color: c.textSoft, textTransform: 'uppercase', letterSpacing: '.1em',
            borderBottom: `1.5px solid ${c.lineStrong}`,
          },
        },
      },
      MuiChip: { styleOverrides: { root: { fontFamily: mono, fontSize: 11, fontWeight: 500, borderRadius: 2, height: 22 } } },
      MuiButton: {
        defaultProps: { disableElevation: true, disableRipple: true },
        styleOverrides: {
          root: { borderRadius: 2 },
          contained: { boxShadow: 'none', '&:hover': { boxShadow: 'none' } },
          outlined: { borderColor: c.lineStrong },
        },
      },
      MuiTextField: { defaultProps: { size: 'small' } },
      MuiOutlinedInput: {
        styleOverrides: { root: { backgroundColor: c.card, borderRadius: 2 }, notchedOutline: { borderColor: c.lineStrong } },
      },
      MuiToggleButton: {
        styleOverrides: { root: {
          fontFamily: mono, fontSize: 12, textTransform: 'none', borderColor: c.lineStrong, color: c.textSoft,
          '&.Mui-selected': { color: c.primary, backgroundColor: mode === 'dark' ? 'rgba(63,176,160,.16)' : 'rgba(31,111,102,.10)' },
        } },
      },
      MuiListItemButton: {
        styleOverrides: { root: {
          borderRadius: 2,
          '&.Mui-selected': {
            backgroundColor: mode === 'dark' ? 'rgba(63,176,160,.16)' : 'rgba(31,111,102,.10)',
            '&:hover': { backgroundColor: mode === 'dark' ? 'rgba(63,176,160,.22)' : 'rgba(31,111,102,.14)' },
          },
        } },
      },
      MuiTooltip: {
        styleOverrides: {
          tooltip: { backgroundColor: mode === 'dark' ? '#0d0b07' : c.text, color: c.paper, fontFamily: mono, fontSize: 11, borderRadius: 2, border: `1px solid ${c.lineStrong}` },
          arrow: { color: mode === 'dark' ? '#0d0b07' : c.text },
        },
      },
      MuiLinearProgress: {
        styleOverrides: { root: { backgroundColor: c.cardInset, borderRadius: 2, border: `1px solid ${c.line}` } },
      },
    },
  })
}

export default makeTheme('light')
