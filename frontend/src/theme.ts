import { createTheme } from '@mui/material/styles'

// Vintage paper, but a technical instrument rather than a scrapbook: aged stock,
// ink in petrol-teal and oxblood, monospaced labels like a lab printout, hairline
// rules and the occasional double rule of a letterpress frame. Flat — paper has
// no drop shadow — so depth comes from rules and insets, not blur.

export const ink = {
  paper:     '#f3efe3', // the page
  card:      '#faf7ee', // a sheet laid on the page
  cardInset: '#efeadb', // a recessed panel / table head
  line:      '#ddd5c0', // hairline rule
  lineStrong:'#c6bda2',
  primary:   '#1f6f66', // petrol teal — the instrument's ink
  oxblood:   '#9c3b2e', // the warning/accent stamp
  amber:     '#a9781a',
  indigo:    '#2f4ba8',
  text:      '#2c2823',
  textSoft:  '#6b6353',
  textFaint: '#938a76',
}

const mono = '"IBM Plex Mono", ui-monospace, "SFMono-Regular", Menlo, monospace'
const sans = '"IBM Plex Sans", system-ui, -apple-system, "Helvetica Neue", sans-serif'

const theme = createTheme({
  palette: {
    mode:       'light',
    primary:    { main: ink.primary },
    secondary:  { main: ink.indigo },
    success:    { main: '#3f7d57' },
    warning:    { main: ink.amber },
    error:      { main: ink.oxblood },
    background: { default: ink.paper, paper: ink.card },
    text:       { primary: ink.text, secondary: ink.textSoft },
    divider:    ink.line,
    action:     { hover: 'rgba(31,111,102,.06)', selected: 'rgba(31,111,102,.10)' },
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
        backgroundImage: 'none', backgroundColor: ink.card,
        border: `1px solid ${ink.line}`, borderRadius: 3,
        boxShadow: `0 1px 0 ${ink.line}`, // a single ruled line under the sheet, not a glow
      } },
    },
    MuiAppBar: {
      styleOverrides: { root: {
        backgroundImage: 'none', color: ink.text, backgroundColor: ink.card,
        borderBottom: `1px solid ${ink.lineStrong}`, boxShadow: 'none',
      } },
    },
    MuiTableCell: {
      styleOverrides: {
        root: { borderColor: ink.line },
        head: {
          backgroundColor: ink.cardInset, fontFamily: mono, fontWeight: 600,
          fontSize: 10.5, color: ink.textSoft, textTransform: 'uppercase', letterSpacing: '.1em',
          borderBottom: `1.5px solid ${ink.lineStrong}`,
        },
      },
    },
    MuiChip: {
      styleOverrides: { root: {
        fontFamily: mono, fontSize: 11, fontWeight: 500, borderRadius: 2, height: 22,
      } },
    },
    MuiButton: {
      defaultProps: { disableElevation: true, disableRipple: true },
      styleOverrides: {
        root: { borderRadius: 2 },
        contained: { boxShadow: 'none', '&:hover': { boxShadow: 'none' } },
        outlined: { borderColor: ink.lineStrong },
      },
    },
    MuiTextField: { defaultProps: { size: 'small' } },
    MuiOutlinedInput: {
      styleOverrides: {
        root: { backgroundColor: ink.card, borderRadius: 2 },
        notchedOutline: { borderColor: ink.lineStrong },
      },
    },
    MuiToggleButton: {
      styleOverrides: { root: {
        fontFamily: mono, fontSize: 12, textTransform: 'none', borderColor: ink.lineStrong,
        color: ink.textSoft, '&.Mui-selected': { color: ink.primary, backgroundColor: 'rgba(31,111,102,.10)' },
      } },
    },
    MuiListItemButton: {
      styleOverrides: { root: {
        borderRadius: 2,
        '&.Mui-selected': { backgroundColor: 'rgba(31,111,102,.10)', '&:hover': { backgroundColor: 'rgba(31,111,102,.14)' } },
      } },
    },
    MuiTooltip: {
      styleOverrides: { tooltip: {
        backgroundColor: ink.text, fontFamily: mono, fontSize: 11, borderRadius: 2,
      }, arrow: { color: ink.text } },
    },
    MuiLinearProgress: {
      styleOverrides: { root: { backgroundColor: ink.cardInset, borderRadius: 2, border: `1px solid ${ink.line}` } },
    },
  },
})

export default theme
