import { createTheme } from '@mui/material/styles'

const theme = createTheme({
  palette: {
    mode:       'dark',
    primary:    { main: '#f5a623' },
    secondary:  { main: '#56b6f7' },
    success:    { main: '#4ec9a3' },
    warning:    { main: '#fbbf24' },
    error:      { main: '#f07070' },
    background: { default: '#0b0e17', paper: '#131825' },
    text:       { primary: '#dde4f5', secondary: '#6b789a' },
    divider:    '#232d43',
  },
  shape: { borderRadius: 8 },
  typography: { fontFamily: '"Roboto", "Helvetica", "Arial", sans-serif' },
  components: {
    MuiCard: {
      styleOverrides: { root: { backgroundImage: 'none', border: '1px solid #232d43' } },
    },
    MuiPaper: {
      styleOverrides: { root: { backgroundImage: 'none' } },
    },
    MuiAppBar: {
      styleOverrides: { root: { backgroundImage: 'none', backgroundColor: '#131825', borderBottom: '1px solid #232d43' } },
    },
    MuiTableCell: {
      styleOverrides: {
        root: { borderColor: '#232d43' },
        head: { backgroundColor: '#1a2035', fontWeight: 600, fontSize: 11, color: '#6b789a', textTransform: 'uppercase', letterSpacing: '.07em' },
      },
    },
    MuiChip:      { styleOverrides: { root: { fontFamily: 'monospace', fontSize: 11 } } },
    MuiTextField: { defaultProps: { size: 'small' } },
    MuiButton:    { defaultProps: { disableElevation: true } },
    MuiListItemButton: {
      styleOverrides: { root: { borderRadius: 6, '&.Mui-selected': { backgroundColor: 'rgba(245,166,35,.10)' } } },
    },
  },
})

export default theme
