import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { DICT_UI } from '../src/i18n.ui'
import { LANGUAGES } from '../src/i18n'

// Partial coverage degrades to an English-ish label rather than blank space, so
// a missing translation is invisible in the running app. These tests make it
// visible here instead.

const LANGS = LANGUAGES.map(l => l.code)

describe('the UI dictionary', () => {
  it('covers every supported language for every key', () => {
    const gaps: string[] = []
    for (const [key, entry] of Object.entries(DICT_UI)) {
      for (const lang of LANGS) {
        if (!entry[lang]?.trim()) gaps.push(`${key}/${lang}`)
      }
    }
    expect(gaps).toEqual([])
  })

  it('uses the same placeholders in every language', () => {
    const vars = (s: string) => [...s.matchAll(/\{(\w+)\}/g)].map(m => m[1]).sort().join(',')
    const mismatches: string[] = []
    for (const [key, entry] of Object.entries(DICT_UI)) {
      const expected = vars(entry.en)
      for (const lang of LANGS) {
        if (vars(entry[lang]) !== expected) mismatches.push(`${key}/${lang}`)
      }
    }
    expect(mismatches).toEqual([])
  })
})

describe('every key the app asks for exists', () => {
  // t() returns the key when it is missing, which reads as a stray identifier in
  // the interface. Scanning the source for literal t('…') calls catches the
  // common case: a key that was typed into a component and never translated.
  const CORE = new Set(
    [...readFileSync(new URL('../src/i18n.tsx', import.meta.url), 'utf8')
      .matchAll(/^ {2}'([\w.:-]+)':/gm)].map(m => m[1]),
  )

  const sources = [
    'App.tsx', 'components/ActivityFeed.tsx', 'components/BulkBar.tsx', 'components/CommandPalette.tsx',
    'components/DashboardWidget.tsx', 'components/ExportMenu.tsx', 'components/FindingDrawer.tsx',
    'components/FindingsTable.tsx', 'components/Layout.tsx', 'components/OnboardingTour.tsx',
    'components/PostureScore.tsx', 'components/SavedFilterMenu.tsx', 'components/ScanTimeline.tsx',
    'components/States.tsx', 'components/TopBar.tsx',
    'pages/AnalyticsPage.tsx', 'pages/FindingsPage.tsx', 'pages/OverviewPage.tsx',
    'pages/RepositoriesPage.tsx', 'pages/ScanDetailPage.tsx', 'pages/SettingsPage.tsx',
  ]

  it('across every component and page', () => {
    const missing = new Set<string>()
    for (const file of sources) {
      const src = readFileSync(new URL(`../src/${file}`, import.meta.url), 'utf8')
      for (const m of src.matchAll(/\bt\('([\w.:-]+)'/g)) {
        const key = m[1]
        if (!CORE.has(key) && !(key in DICT_UI)) missing.add(`${file}: ${key}`)
      }
    }
    expect([...missing].sort()).toEqual([])
  })
})
