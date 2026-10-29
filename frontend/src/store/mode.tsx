import React, { createContext, useContext, useEffect, useMemo } from 'react'
import CssBaseline from '@mui/material/CssBaseline'
import { ThemeProvider } from '@mui/material/styles'
import { makeTheme, type Mode } from '../theme'
import { usePrefs } from './preferences'

// The chosen stock (light/dark paper) and the preset it is printed in. Both now
// live in store/preferences; this file stays because the toggle in the top bar
// and every `useColorMode()` caller only ever wanted "which stock, and flip it".
//
// toggle() writes an explicit light or dark, leaving 'system'. That is the point
// of a toggle: someone who presses it has stopped delegating to the OS. The
// appearance settings offer 'system' back.
interface Ctx { mode: Mode; toggle: () => void }
const ColorModeCtx = createContext<Ctx>({ mode: 'light', toggle: () => {} })

export function ColorModeProvider({ children }: { children: React.ReactNode }) {
  const { prefs, set, mode, motionOff } = usePrefs()
  const theme = useMemo(
    () => makeTheme({ preset: prefs.themePreset, mode, density: prefs.density, reducedMotion: motionOff }),
    [prefs.themePreset, prefs.density, mode, motionOff],
  )

  useEffect(() => {
    // Keep the pre-React body background (set in index.html) in step, so the
    // page chrome around the app matches and dark mode has no light gutter.
    document.body.style.backgroundColor = theme.palette.background.default
    document.documentElement.style.colorScheme = mode
  }, [mode, theme])

  const value = useMemo(() => ({
    mode,
    toggle: () => set('colorMode', mode === 'dark' ? 'light' : 'dark'),
  }), [mode, set])

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
