import React, { createContext, useContext, useEffect, useMemo, useState } from 'react'
import CssBaseline from '@mui/material/CssBaseline'
import { ThemeProvider } from '@mui/material/styles'
import { makeTheme, type Mode } from '../theme'

// The chosen stock (light/dark paper), remembered per browser. First visit
// follows the OS preference; after that the toggle wins.
interface Ctx { mode: Mode; toggle: () => void }
const ColorModeCtx = createContext<Ctx>({ mode: 'light', toggle: () => {} })

function initial(): Mode {
  try {
    const saved = localStorage.getItem('gs-mode')
    if (saved === 'light' || saved === 'dark') return saved
  } catch { /* storage blocked */ }
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function ColorModeProvider({ children }: { children: React.ReactNode }) {
  const [mode, setMode] = useState<Mode>(initial)
  const theme = useMemo(() => makeTheme(mode), [mode])

  useEffect(() => {
    try { localStorage.setItem('gs-mode', mode) } catch { /* ignore */ }
    // Keep the pre-React body background (set in index.html) in step, so the
    // page chrome around the app matches and dark mode has no light gutter.
    document.body.style.backgroundColor = theme.palette.background.default
    document.documentElement.style.colorScheme = mode
  }, [mode, theme])

  const value = useMemo(() => ({ mode, toggle: () => setMode(m => (m === 'dark' ? 'light' : 'dark')) }), [mode])
  return (
    <ColorModeCtx.Provider value={value}>
      <ThemeProvider theme={theme}>
        <CssBaseline />
        {children}
      </ThemeProvider>
    </ColorModeCtx.Provider>
  )
}

export const useColorMode = () => useContext(ColorModeCtx)
