import { createTheme, type Theme } from '@mui/material/styles'
import type { Severity } from './types'

// Vintage paper as a technical instrument, in two stocks: a light aged sheet and
// a dark one (old terminal / carbon paper). Both keep petrol-teal and oxblood
// ink, monospaced labels, hairline and double rules, and no drop shadows.
//
// That stock is now one preset of six. A preset is a token set per mode, never a
// light palette inverted at runtime: a dark surface needs its own steps to keep
// contrast. Everything below the palettes — spacing, rules, type — is shared, so
// a preset changes the ink and not the instrument.
//
// Chart colours live on the theme (theme.chart) so they switch with the mode and
// are re-stepped for each surface. Severity keeps its meaning across presets:
// critical is always the most alarming hue of that palette, and severity is
// never carried by colour alone (SeverityChip prints the word and a dot).

export type Mode = 'light' | 'dark'
export type ThemePreset = 'paper' | 'midnight' | 'ocean' | 'signal' | 'minimal' | 'high-contrast'
export type Density = 'comfortable' | 'compact'

export const THEME_PRESETS: ThemePreset[] = ['paper', 'midnight', 'ocean', 'signal', 'minimal', 'high-contrast']

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

/** The ink an illustration is re-coloured with; see components/Illustration.tsx. */
export interface IllustrationPalette {
  accent:  string
  accent2: string
  ink:     string
  ink2:    string
  mid:     string
  soft:    string
  paper:   string
}

// Augment the MUI theme so theme.chart is typed everywhere useTheme() is used.
declare module '@mui/material/styles' {
  interface Theme {
    chart: ChartPalette
    illustration: IllustrationPalette
    preset: ThemePreset
    density: Density
    reducedMotion: boolean
  }
  interface ThemeOptions {
    chart?: ChartPalette
    illustration?: IllustrationPalette
    preset?: ThemePreset
    density?: Density
    reducedMotion?: boolean
  }
}

interface Tokens {
  paper: string; card: string; cardInset: string; line: string; lineStrong: string
  primary: string; oxblood: string; amber: string; indigo: string; success: string
  text: string; textSoft: string; textFaint: string
  categorical: string[]
  severity: Record<Severity, string>
  /** Serif/mono face pair; a preset may trade the typewriter look for a plain one. */
  mono?: string
  sans?: string
}

const sev = (critical: string, high: string, medium: string, low: string, info: string): Record<Severity, string> =>
  ({ critical, high, medium, low, info })

const paperLight: Tokens = {
  paper: '#f3efe3', card: '#faf7ee', cardInset: '#efeadb', line: '#ddd5c0', lineStrong: '#c6bda2',
  primary: '#1f6f66', oxblood: '#9c3b2e', amber: '#a9781a', indigo: '#2f4ba8', success: '#3f7d57',
  text: '#2c2823', textSoft: '#6b6353', textFaint: '#938a76',
  categorical: ['#0a8a70', '#c0392b', '#2f4ba8', '#a9781a', '#8e3b6b', '#5f7a1f'],
  severity: sev('#9c3b2e', '#c2622a', '#a9781a', '#2f4ba8', '#8a8372'),
}

const paperDark: Tokens = {
  paper: '#17150f', card: '#211d15', cardInset: '#14120c', line: '#39342699', lineStrong: '#4d4634',
  primary: '#3fb0a0', oxblood: '#e2583f', amber: '#cf9a2e', indigo: '#8ea0e8', success: '#5fb07f',
  text: '#e9e2cf', textSoft: '#a79d84', textFaint: '#7d745f',
  categorical: ['#1f9e8b', '#e2583f', '#6277d8', '#ad8324', '#c05f94', '#7f9a35'],
  severity: sev('#e2583f', '#e8893f', '#cf9a2e', '#6277d8', '#9a9078'),
}

// Midnight is the terminal: near-black with phosphor green and cyan. Its light
// stock is not a bright theme, it is a paler terminal, so the preset keeps its
// character when someone forces light mode.
const midnightDark: Tokens = {
  paper: '#07090b', card: '#0e1216', cardInset: '#090c10', line: '#1d262e', lineStrong: '#2c3842',
  primary: '#33d9a6', oxblood: '#ff5c5c', amber: '#e6b65c', indigo: '#56b6ff', success: '#33d9a6',
  text: '#d8e4e6', textSoft: '#8b9ba3', textFaint: '#5f6f78',
  categorical: ['#33d9a6', '#ff5c5c', '#56b6ff', '#e6b65c', '#c98bdb', '#8ac34a'],
  severity: sev('#ff5c5c', '#ff8f4d', '#e6b65c', '#56b6ff', '#7c8a93'),
}

const midnightLight: Tokens = {
  paper: '#eef2f1', card: '#ffffff', cardInset: '#e6ecea', line: '#cfd9d6', lineStrong: '#aebbb7',
  primary: '#0b7a5a', oxblood: '#b4332f', amber: '#8c6411', indigo: '#1565a8', success: '#0b7a5a',
  text: '#101a19', textSoft: '#4f5f5c', textFaint: '#7b8a87',
  categorical: ['#0b7a5a', '#b4332f', '#1565a8', '#8c6411', '#7b3f91', '#4d7016'],
  severity: sev('#b4332f', '#b35418', '#8c6411', '#1565a8', '#6f7d7a'),
}

const oceanDark: Tokens = {
  paper: '#0a1626', card: '#101f33', cardInset: '#0b1a2c', line: '#1d3350', lineStrong: '#2a4a6b',
  primary: '#3fc9e0', oxblood: '#f2635c', amber: '#e3a84a', indigo: '#6ba6ff', success: '#45c08a',
  text: '#e2ecf6', textSoft: '#93aac2', textFaint: '#6b8099',
  categorical: ['#3fc9e0', '#f2635c', '#6ba6ff', '#e3a84a', '#b183e8', '#45c08a'],
  severity: sev('#f2635c', '#f08a45', '#e3a84a', '#6ba6ff', '#7f93a8'),
}

const oceanLight: Tokens = {
  paper: '#eef4fa', card: '#ffffff', cardInset: '#e3ecf5', line: '#ccdcea', lineStrong: '#a6c0d6',
  primary: '#0f6f88', oxblood: '#b63a35', amber: '#9a6c13', indigo: '#1f4fa8', success: '#16795a',
  text: '#0e2133', textSoft: '#4a6276', textFaint: '#7890a4',
  categorical: ['#0f6f88', '#b63a35', '#1f4fa8', '#9a6c13', '#6c3f9e', '#16795a'],
  severity: sev('#b63a35', '#b35a1c', '#9a6c13', '#1f4fa8', '#6e8294'),
}

// Signal is the operations wall: graphite surfaces where only alerts carry
// saturation, so a critical row is the one thing that glows.
const signalDark: Tokens = {
  paper: '#111113', card: '#1a1a1d', cardInset: '#161618', line: '#2b2b30', lineStrong: '#3c3c43',
  primary: '#ff8a3d', oxblood: '#ff4d4d', amber: '#ffc14d', indigo: '#7aa2f7', success: '#4dd18a',
  text: '#ececed', textSoft: '#9c9ca5', textFaint: '#6e6e78',
  categorical: ['#ff8a3d', '#ff4d4d', '#7aa2f7', '#ffc14d', '#c792ea', '#4dd18a'],
  severity: sev('#ff4d4d', '#ff8a3d', '#ffc14d', '#7aa2f7', '#8a8a93'),
}

const signalLight: Tokens = {
  paper: '#f4f4f5', card: '#ffffff', cardInset: '#ebebed', line: '#dcdce0', lineStrong: '#bdbdc4',
  primary: '#b8521b', oxblood: '#c22a2a', amber: '#8f6511', indigo: '#2f4ba8', success: '#1d7a4d',
  text: '#1b1b1f', textSoft: '#55555f', textFaint: '#7f7f8a',
  categorical: ['#b8521b', '#c22a2a', '#2f4ba8', '#8f6511', '#7b3f91', '#1d7a4d'],
  severity: sev('#c22a2a', '#b8521b', '#8f6511', '#2f4ba8', '#70707a'),
}

// Minimal drops the typewriter voice: a plain enterprise dashboard for people who
// want the data and not the instrument.
const minimalLight: Tokens = {
  paper: '#f7f8fa', card: '#ffffff', cardInset: '#f0f2f5', line: '#e2e5ea', lineStrong: '#c7ccd4',
  primary: '#2563a8', oxblood: '#c0392b', amber: '#9a6c13', indigo: '#4b4fa8', success: '#1f7a52',
  text: '#1d2127', textSoft: '#5b636e', textFaint: '#868e9a',
  categorical: ['#2563a8', '#c0392b', '#1f7a52', '#9a6c13', '#7b3f91', '#0f7f93'],
  severity: sev('#c0392b', '#b35a1c', '#9a6c13', '#2563a8', '#7a828c'),
  mono: 'ui-monospace, "SFMono-Regular", Menlo, monospace',
  sans: 'system-ui, -apple-system, "Segoe UI", "Helvetica Neue", sans-serif',
}

const minimalDark: Tokens = {
  paper: '#14171c', card: '#1b1f26', cardInset: '#171a20', line: '#2a2f38', lineStrong: '#3b414c',
  primary: '#5c9fe0', oxblood: '#ef6b5c', amber: '#d9a443', indigo: '#8b8ff0', success: '#49b183',
  text: '#e6e9ee', textSoft: '#9aa3b0', textFaint: '#6f7885',
  categorical: ['#5c9fe0', '#ef6b5c', '#49b183', '#d9a443', '#b48bea', '#3fb6c7'],
  severity: sev('#ef6b5c', '#e9893f', '#d9a443', '#5c9fe0', '#838c99'),
  mono: 'ui-monospace, "SFMono-Regular", Menlo, monospace',
  sans: 'system-ui, -apple-system, "Segoe UI", "Helvetica Neue", sans-serif',
}

// High contrast is the accessibility stock: pure surfaces, black or white text,
// heavier rules. Every pair below clears WCAG AA for body text and AAA for the
// ink on its surface; the categorical set is the one place where contrast is
// traded for separability, and the bar list direct-labels every row anyway.
const contrastLight: Tokens = {
  paper: '#ffffff', card: '#ffffff', cardInset: '#f0f0f0', line: '#767676', lineStrong: '#000000',
  primary: '#005c4f', oxblood: '#a81414', amber: '#6b4a00', indigo: '#0b3ea8', success: '#005c2e',
  text: '#000000', textSoft: '#3a3a3a', textFaint: '#595959',
  categorical: ['#005c4f', '#a81414', '#0b3ea8', '#6b4a00', '#6b1a6b', '#1f4d00'],
  severity: sev('#a81414', '#8a3400', '#6b4a00', '#0b3ea8', '#3a3a3a'),
}

const contrastDark: Tokens = {
  paper: '#000000', card: '#0b0b0b', cardInset: '#161616', line: '#8c8c8c', lineStrong: '#ffffff',
  primary: '#4fe0c8', oxblood: '#ff6b6b', amber: '#ffcc4d', indigo: '#8fb8ff', success: '#5fe08f',
  text: '#ffffff', textSoft: '#d6d6d6', textFaint: '#ababab',
  categorical: ['#4fe0c8', '#ff6b6b', '#8fb8ff', '#ffcc4d', '#e392e3', '#a8e05f'],
  severity: sev('#ff6b6b', '#ffa14d', '#ffcc4d', '#8fb8ff', '#c4c4c4'),
}

const PRESETS: Record<ThemePreset, Record<Mode, Tokens>> = {
  'paper':         { light: paperLight,    dark: paperDark },
  'midnight':      { light: midnightLight, dark: midnightDark },
  'ocean':         { light: oceanLight,    dark: oceanDark },
  'signal':        { light: signalLight,   dark: signalDark },
  'minimal':       { light: minimalLight,  dark: minimalDark },
  'high-contrast': { light: contrastLight, dark: contrastDark },
}

/** The three swatches a preset is previewed with in settings, without building a theme. */
export function presetSwatches(preset: ThemePreset, mode: Mode): { paper: string; ink: string; accent: string; alert: string } {
  const c = PRESETS[preset][mode]
  return { paper: c.paper, ink: c.text, accent: c.primary, alert: c.severity.critical }
}

// ink is the current light token set, kept for any non-themed reference.
export const ink = paperLight

export interface ThemeInput {
  preset?:        ThemePreset
  mode:           Mode
  density?:       Density
  reducedMotion?: boolean
}

export function makeTheme(input: ThemeInput | Mode): Theme {
  const { preset = 'paper', mode, density = 'comfortable', reducedMotion = false } =
    typeof input === 'string' ? { mode: input } as ThemeInput : input
  const c = (PRESETS[preset] ?? PRESETS.paper)[mode]
  const mono = c.mono ?? '"IBM Plex Mono", ui-monospace, "SFMono-Regular", Menlo, monospace'
  const sans = c.sans ?? '"IBM Plex Sans", system-ui, -apple-system, "Helvetica Neue", sans-serif'
  const tight  = density === 'compact'
  const hc     = preset === 'high-contrast'
  // Compact trades padding for rows on screen; it never changes a font size below
  // 11px, because that is where the monospaced labels stop being legible.
  const cellPad = tight ? '4px 8px' : '8px 12px'
  const rule    = hc ? 2 : 1

  const selected = mode === 'dark' ? `${c.primary}29` : `${c.primary}1a`
  const hovered  = mode === 'dark' ? `${c.primary}1a` : `${c.primary}10`

  return createTheme({
    preset, density, reducedMotion,
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
      action:     { hover: hovered, selected },
    },
    chart: {
      categorical: c.categorical, other: c.textFaint, severity: c.severity,
      surface: c.card, inset: c.cardInset, line: c.line, lineStrong: c.lineStrong,
      text: c.text, textSoft: c.textSoft, textFaint: c.textFaint, paper: c.paper,
    },
    illustration: {
      accent: c.primary, accent2: c.oxblood, ink: c.text, ink2: c.textSoft,
      mid: c.lineStrong, soft: c.cardInset, paper: c.card,
    },
    shape: { borderRadius: preset === 'minimal' ? 6 : 3 },
    spacing: tight ? 7 : 8,
    typography: {
      fontFamily: sans,
      h1: { fontFamily: mono }, h2: { fontFamily: mono }, h3: { fontFamily: mono },
      h4: { fontFamily: mono }, h5: { fontFamily: mono }, h6: { fontFamily: mono },
      button: { textTransform: 'none', fontWeight: 600, letterSpacing: '.01em' },
      overline: { fontFamily: mono, letterSpacing: '.14em' },
    },
    components: {
      MuiCssBaseline: {
        styleOverrides: {
          // A focus ring that is always visible, on every preset. The browser
          // default disappears against the darker stocks, and keyboard users
          // lose their place; :focus-visible keeps it off mouse clicks.
          // A faint dot grid, like the graph paper an instrument plots on. It is
          // left off high contrast, where any texture behind text costs contrast.
          ...(hc ? {} : {
            body: {
              backgroundImage: `radial-gradient(${c.lineStrong}${mode === 'dark' ? '55' : '66'} 1px, transparent 1.2px)`,
              backgroundSize: '22px 22px',
              backgroundAttachment: 'fixed',
            },
          }),
          ':focus-visible': {
            outline: `${hc ? 3 : 2}px solid ${c.primary}`,
            outlineOffset: 2,
          },
          // Reduced motion is honoured whether it comes from the OS or from the
          // appearance setting, so the preference is testable without changing
          // the OS. Transitions are cut to a frame rather than removed, so a
          // component that waits on transitionend still gets its event.
          ...(reducedMotion ? {
            '*, *::before, *::after': {
              animationDuration: '.001ms !important',
              animationIterationCount: '1 !important',
              transitionDuration: '.001ms !important',
              scrollBehavior: 'auto !important',
            },
          } : {}),
          '@media (prefers-reduced-motion: reduce)': {
            '*, *::before, *::after': {
              animationDuration: '.001ms !important',
              animationIterationCount: '1 !important',
              transitionDuration: '.001ms !important',
            },
          },
        },
      },
      MuiPaper: { styleOverrides: { root: { backgroundImage: 'none' } } },
      MuiCard: {
        defaultProps: { elevation: 0 },
        styleOverrides: { root: {
          backgroundImage: 'none', backgroundColor: c.card,
          border: `${rule}px solid ${c.line}`, borderRadius: preset === 'minimal' ? 6 : 3,
          boxShadow: hc ? 'none' : `0 1px 0 ${c.line}`,
        } },
      },
      MuiAppBar: {
        styleOverrides: { root: {
          backgroundImage: 'none', color: c.text, backgroundColor: c.card,
          borderBottom: `${rule}px solid ${c.lineStrong}`, boxShadow: 'none',
        } },
      },
      MuiTableCell: {
        styleOverrides: {
          root: { borderColor: c.line, padding: cellPad },
          head: {
            backgroundColor: c.cardInset, fontFamily: mono, fontWeight: 600,
            fontSize: 10.5, color: c.textSoft, textTransform: 'uppercase', letterSpacing: '.1em',
            borderBottom: `${hc ? 2.5 : 1.5}px solid ${c.lineStrong}`,
            padding: tight ? '4px 8px' : '6px 12px',
          },
        },
      },
      MuiChip: { styleOverrides: { root: { fontFamily: mono, fontSize: 11, fontWeight: 500, borderRadius: 2, height: tight ? 20 : 22 } } },
      MuiButton: {
        defaultProps: { disableElevation: true, disableRipple: true },
        styleOverrides: {
          root: { borderRadius: 2 },
          contained: { boxShadow: 'none', '&:hover': { boxShadow: 'none' } },
          outlined: { borderColor: c.lineStrong, borderWidth: rule },
        },
      },
      MuiTextField: { defaultProps: { size: 'small' } },
      MuiOutlinedInput: {
        styleOverrides: { root: { backgroundColor: c.card, borderRadius: 2 }, notchedOutline: { borderColor: c.lineStrong, borderWidth: rule } },
      },
      MuiToggleButton: {
        styleOverrides: { root: {
          fontFamily: mono, fontSize: 12, textTransform: 'none', borderColor: c.lineStrong, color: c.textSoft,
          '&.Mui-selected': { color: c.primary, backgroundColor: selected },
        } },
      },
      MuiListItemButton: {
        styleOverrides: { root: {
          borderRadius: 2,
          '&.Mui-selected': { backgroundColor: selected, '&:hover': { backgroundColor: selected } },
        } },
      },
      MuiTooltip: {
        styleOverrides: {
          tooltip: { backgroundColor: mode === 'dark' ? c.cardInset : c.text, color: mode === 'dark' ? c.text : c.paper, fontFamily: mono, fontSize: 11, borderRadius: 2, border: `1px solid ${c.lineStrong}` },
          arrow: { color: mode === 'dark' ? c.cardInset : c.text },
        },
      },
      MuiLinearProgress: {
        styleOverrides: { root: { backgroundColor: c.cardInset, borderRadius: 2, border: `1px solid ${c.line}` } },
      },
    },
  })
}

export default makeTheme('light')
