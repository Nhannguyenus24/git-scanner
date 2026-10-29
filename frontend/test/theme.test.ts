import { describe, expect, it } from 'vitest'
import { makeTheme, presetSwatches, THEME_PRESETS, type Mode } from '../src/theme'
import { SEVERITIES } from '../src/types'

// Contrast of a colour pair, by the WCAG 2.1 relative-luminance formula. It is
// reimplemented here rather than imported so the test cannot pass because the
// app and the test share the same mistake.
function luminance(hex: string): number {
  const h = hex.replace('#', '')
  const full = h.length === 3 ? [...h].map(c => c + c).join('') : h.slice(0, 6)
  const [r, g, b] = [0, 2, 4].map(i => parseInt(full.slice(i, i + 2), 16) / 255)
    .map(c => (c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4))
  return 0.2126 * r + 0.7152 * g + 0.0722 * b
}

function contrast(a: string, b: string): number {
  const [x, y] = [luminance(a), luminance(b)].sort((m, n) => n - m)
  return (x + 0.05) / (y + 0.05)
}

const MODES: Mode[] = ['light', 'dark']

describe('every preset builds', () => {
  for (const preset of THEME_PRESETS) {
    for (const mode of MODES) {
      it(`${preset} / ${mode}`, () => {
        const t = makeTheme({ preset, mode })
        expect(t.palette.mode).toBe(mode)
        expect(t.preset).toBe(preset)
        expect(t.chart.categorical.length).toBeGreaterThanOrEqual(6)
        // Severity keeps its full vocabulary in every preset: a missing key
        // would render a chip with an undefined colour and no warning.
        for (const s of SEVERITIES) expect(t.chart.severity[s]).toMatch(/^#[0-9a-f]{3,8}$/i)
      })
    }
  }
})

describe('text is readable on its own surface', () => {
  for (const preset of THEME_PRESETS) {
    for (const mode of MODES) {
      it(`${preset} / ${mode} body text clears AA`, () => {
        const t = makeTheme({ preset, mode })
        expect(contrast(t.chart.text, t.chart.surface)).toBeGreaterThanOrEqual(4.5)
        expect(contrast(t.chart.text, t.chart.paper)).toBeGreaterThanOrEqual(4.5)
      })

      it(`${preset} / ${mode} secondary text clears AA for large text`, () => {
        const t = makeTheme({ preset, mode })
        expect(contrast(t.chart.textSoft, t.chart.surface)).toBeGreaterThanOrEqual(3)
      })
    }
  }

  it('high contrast goes past AAA', () => {
    for (const mode of MODES) {
      const t = makeTheme({ preset: 'high-contrast', mode })
      expect(contrast(t.chart.text, t.chart.surface)).toBeGreaterThanOrEqual(7)
    }
  })
})

describe('severity stays distinguishable from the surface', () => {
  for (const preset of THEME_PRESETS) {
    for (const mode of MODES) {
      it(`${preset} / ${mode}`, () => {
        const t = makeTheme({ preset, mode })
        for (const s of SEVERITIES) {
          // 3:1 is the WCAG threshold for a non-text indicator, which is what a
          // severity swatch is — the word next to it carries the meaning.
          expect(contrast(t.chart.severity[s], t.chart.surface)).toBeGreaterThanOrEqual(3)
        }
      })
    }
  }
})

describe('density and motion', () => {
  it('compact tightens the base spacing without changing the palette', () => {
    const comfortable = makeTheme({ preset: 'paper', mode: 'light' })
    const compact = makeTheme({ preset: 'paper', mode: 'light', density: 'compact' })
    expect(compact.spacing(1)).not.toBe(comfortable.spacing(1))
    expect(compact.chart.text).toBe(comfortable.chart.text)
  })

  it('records the motion preference on the theme', () => {
    expect(makeTheme({ preset: 'paper', mode: 'light', reducedMotion: true }).reducedMotion).toBe(true)
    expect(makeTheme({ preset: 'paper', mode: 'light' }).reducedMotion).toBe(false)
  })

  it('still accepts the old makeTheme(mode) call', () => {
    expect(makeTheme('dark').palette.mode).toBe('dark')
  })
})

describe('presetSwatches', () => {
  it('previews a preset without building a theme', () => {
    for (const preset of THEME_PRESETS) {
      const s = presetSwatches(preset, 'light')
      expect(s.paper).toMatch(/^#/)
      expect(s.accent).toMatch(/^#/)
      expect(s.alert).toMatch(/^#/)
      expect(s.paper).toBe(makeTheme({ preset, mode: 'light' }).chart.paper)
    }
  })
})
