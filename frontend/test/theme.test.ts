import { describe, expect, it } from 'vitest'
import { makeTheme } from '../src/theme'

describe('theme palettes', () => {
  it.each(['light', 'dark'] as const)('contains severity colours in %s mode', mode => {
    const theme = makeTheme(mode)
    expect(theme.palette.mode).toBe(mode)
    expect(Object.values(theme.chart.severity)).toHaveLength(5)
    expect(theme.chart.text).toBeTruthy()
    expect(theme.chart.surface).toBeTruthy()
  })
})
