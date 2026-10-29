import { beforeEach, describe, expect, it } from 'vitest'
import { DEFAULTS, loadPreferences, sanitiseFilter } from '../src/store/preferences'

// A fake localStorage, because these tests are about what happens when what is
// in storage is wrong — missing, stale, hostile or unparseable.
class Mem implements Storage {
  private map = new Map<string, string>()
  get length() { return this.map.size }
  clear() { this.map.clear() }
  getItem(k: string) { return this.map.get(k) ?? null }
  key(i: number) { return [...this.map.keys()][i] ?? null }
  removeItem(k: string) { this.map.delete(k) }
  setItem(k: string, v: string) { this.map.set(k, v) }
}

const store = new Mem()
Object.defineProperty(globalThis, 'localStorage', { value: store, configurable: true })
// loadPreferences falls back to the OS colour scheme only through the provider;
// the loader itself never touches matchMedia, so nothing else is stubbed.

beforeEach(() => store.clear())

describe('loadPreferences', () => {
  it('returns the defaults when nothing is stored', () => {
    expect(loadPreferences()).toEqual(DEFAULTS)
  })

  it('survives unparseable JSON', () => {
    store.setItem('gs-prefs', '{not json')
    expect(loadPreferences()).toEqual(DEFAULTS)
  })

  it('keeps the valid fields and replaces only the invalid ones', () => {
    store.setItem('gs-prefs', JSON.stringify({
      themePreset: 'nonsense', density: 'compact', colorMode: 'dark', sidebarCollapsed: true,
    }))
    const p = loadPreferences()
    expect(p.themePreset).toBe(DEFAULTS.themePreset) // replaced
    expect(p.density).toBe('compact')                 // kept
    expect(p.colorMode).toBe('dark')
    expect(p.sidebarCollapsed).toBe(true)
  })

  it('migrates the pre-preferences gs-mode toggle once', () => {
    store.setItem('gs-mode', 'dark')
    expect(loadPreferences().colorMode).toBe('dark')
  })

  it('ignores gs-mode once a colour mode of its own is stored', () => {
    store.setItem('gs-mode', 'dark')
    store.setItem('gs-prefs', JSON.stringify({ colorMode: 'light' }))
    expect(loadPreferences().colorMode).toBe('light')
  })

  it('drops widget ids that no longer exist, and falls back when none survive', () => {
    store.setItem('gs-prefs', JSON.stringify({ widgets: ['stats', 'ghost', 'triage'] }))
    expect(loadPreferences().widgets).toEqual(['stats', 'triage'])
    store.setItem('gs-prefs', JSON.stringify({ widgets: ['ghost'] }))
    expect(loadPreferences().widgets).toEqual(DEFAULTS.widgets)
  })

  it('keeps saved filters to filter fields only', () => {
    store.setItem('gs-prefs', JSON.stringify({
      savedFilters: [{ id: 'a', name: 'Mine', params: { severity: 'critical', secret: 'AKIA-REAL-KEY', note: 'x' } }],
    }))
    const [f] = loadPreferences().savedFilters
    expect(f.params).toEqual({ severity: 'critical' })
  })

  it('discards a saved filter with no name', () => {
    store.setItem('gs-prefs', JSON.stringify({ savedFilters: [{ id: 'a', params: {} }] }))
    expect(loadPreferences().savedFilters).toEqual([])
  })
})

describe('sanitiseFilter', () => {
  it('passes the known filter fields through', () => {
    expect(sanitiseFilter({ severity: 'high', provider: 'aws', status: 'open', q: 'key', sort: 'recent' }))
      .toEqual({ severity: 'high', provider: 'aws', status: 'open', q: 'key', sort: 'recent' })
  })

  it('drops unknown keys, empty values and non-strings', () => {
    expect(sanitiseFilter({ severity: '', token: 'sk-live-abc', limit: 5, q: 'ok' })).toEqual({ q: 'ok' })
  })

  it('caps a field so a pasted blob cannot be persisted whole', () => {
    expect(sanitiseFilter({ q: 'x'.repeat(5000) }).q).toHaveLength(200)
  })
})
