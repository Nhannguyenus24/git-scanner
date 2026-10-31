import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import { THEME_PRESETS, type Density, type Mode, type ThemePreset } from '../theme'

// Presentation-only preferences, kept in the browser and nowhere else.
//
// This is deliberately separate from store/settings (the API address) and from
// the server's AppSettings (masking, disclosure owners). Those decide what the
// server will show; these decide how this browser draws it. Mixing them is how
// a "remember my layout" feature ends up able to turn masking off, so the two
// never share a store, and nothing here is ever sent to the API.
//
// Nothing read from a finding may be written here: a saved filter stores the
// filter's own fields, never a credential, and the free-text box is a filter
// field like any other — see sanitiseFilter below.

export type ColorMode = Mode | 'system'
export type MotionPref = boolean | 'system'

/** One overview card or panel the user can hide or move. */
export type WidgetId =
  | 'posture' | 'stats' | 'severity' | 'provider' | 'triage' | 'activity' | 'scans'

export const ALL_WIDGETS: WidgetId[] = ['posture', 'stats', 'severity', 'provider', 'triage', 'activity', 'scans']

/** A named finding filter, stored as the query fields only. */
export interface SavedFilter {
  id:     string
  name:   string
  params: Record<string, string>
}

export interface UiPreferences {
  themePreset:      ThemePreset
  colorMode:        ColorMode
  density:          Density
  reducedMotion:    MotionPref
  sidebarCollapsed: boolean
  widgets:          WidgetId[]      // visible, in order
  savedFilters:     SavedFilter[]
  onboarded:        boolean
  /** Browser notifications for finished scans and new live keys. */
  desktopAlerts:    boolean
}

export const DEFAULTS: UiPreferences = {
  themePreset: 'paper',
  colorMode: 'system',
  density: 'comfortable',
  reducedMotion: 'system',
  sidebarCollapsed: false,
  widgets: [...ALL_WIDGETS],
  savedFilters: [],
  onboarded: false,
  desktopAlerts: false,
}

/** The dashboard arrangements offered in settings, each a widget order. */
export const LAYOUT_PRESETS: Record<string, WidgetId[]> = {
  default:   [...ALL_WIDGETS],
  triage:    ['stats', 'severity', 'provider', 'triage', 'scans'],
  executive: ['posture', 'stats', 'severity', 'activity'],
  soc:       ['stats', 'posture', 'activity', 'scans', 'severity', 'provider', 'triage'],
}

const KEY = 'gs-prefs'

const oneOf = <T,>(v: unknown, all: readonly T[], fallback: T): T =>
  (all as readonly unknown[]).includes(v) ? v as T : fallback

/** Drops anything that is not a known filter field; see the note at the top. */
export function sanitiseFilter(params: Record<string, unknown>): Record<string, string> {
  const ALLOWED = ['severity', 'provider', 'status', 'q', 'repository_id', 'scan_id', 'size', 'exposure', 'sort']
  const out: Record<string, string> = {}
  for (const k of ALLOWED) {
    const v = params[k]
    if (typeof v === 'string' && v) out[k] = v.slice(0, 200)
  }
  return out
}

/**
 * Reads the stored preferences, falling back field by field. A stored value that
 * is no longer valid — a preset that was removed, a widget that no longer
 * exists — is replaced with the default rather than failing the whole load, so
 * one stale field cannot cost someone every other preference they set.
 */
export function loadPreferences(): UiPreferences {
  let raw: Record<string, unknown> = {}
  try {
    raw = JSON.parse(localStorage.getItem(KEY) || '{}') as Record<string, unknown>
  } catch { /* unparseable or storage blocked: defaults below */ }

  const widgets = Array.isArray(raw.widgets)
    ? (raw.widgets as unknown[]).filter((w): w is WidgetId => ALL_WIDGETS.includes(w as WidgetId))
    : null

  const filters = Array.isArray(raw.savedFilters)
    ? (raw.savedFilters as Record<string, unknown>[])
        .filter(f => f && typeof f.name === 'string')
        .slice(0, 50)
        .map((f, i) => ({
          id: typeof f.id === 'string' ? f.id : `f${i}`,
          name: String(f.name).slice(0, 60),
          params: sanitiseFilter((f.params ?? {}) as Record<string, unknown>),
        }))
    : []

  return {
    themePreset: oneOf(raw.themePreset, THEME_PRESETS, DEFAULTS.themePreset),
    // gs-mode is the pre-preferences dark-mode toggle. An explicit light/dark
    // there is honoured once and then lives here, so upgrading does not silently
    // flip someone back to following their OS.
    colorMode: oneOf(raw.colorMode, ['light', 'dark', 'system'] as const, legacyMode()),
    density: oneOf(raw.density, ['comfortable', 'compact'] as const, DEFAULTS.density),
    reducedMotion: oneOf(raw.reducedMotion, [true, false, 'system'] as const, DEFAULTS.reducedMotion),
    sidebarCollapsed: raw.sidebarCollapsed === true,
    widgets: widgets?.length ? widgets : [...DEFAULTS.widgets],
    savedFilters: filters,
    onboarded: raw.onboarded === true,
    desktopAlerts: raw.desktopAlerts === true,
  }
}

function legacyMode(): ColorMode {
  try {
    const old = localStorage.getItem('gs-mode')
    if (old === 'light' || old === 'dark') return old
  } catch { /* ignore */ }
  return DEFAULTS.colorMode
}

interface Ctx {
  prefs:  UiPreferences
  set:    <K extends keyof UiPreferences>(key: K, value: UiPreferences[K]) => void
  reset:  () => void
  /** The light/dark stock in force once 'system' is resolved. */
  mode:   Mode
  /** Whether motion should be suppressed, once 'system' is resolved. */
  motionOff: boolean
}

const Ctx = createContext<Ctx | null>(null)

/** Tracks a media query and re-renders when it flips. */
function useMedia(query: string): boolean {
  const [on, setOn] = useState(() => window.matchMedia?.(query).matches ?? false)
  useEffect(() => {
    const mq = window.matchMedia?.(query)
    if (!mq) return
    const fn = (e: MediaQueryListEvent) => setOn(e.matches)
    setOn(mq.matches)
    mq.addEventListener('change', fn)
    return () => mq.removeEventListener('change', fn)
  }, [query])
  return on
}

export function PreferencesProvider({ children }: { children: React.ReactNode }) {
  const [prefs, setPrefs] = useState<UiPreferences>(loadPreferences)
  const systemDark   = useMedia('(prefers-color-scheme: dark)')
  const systemMotion = useMedia('(prefers-reduced-motion: reduce)')

  useEffect(() => {
    try { localStorage.setItem(KEY, JSON.stringify(prefs)) } catch { /* storage blocked: this session only */ }
  }, [prefs])

  const set = useCallback(<K extends keyof UiPreferences>(key: K, value: UiPreferences[K]) =>
    setPrefs(p => ({ ...p, [key]: value })), [])

  const reset = useCallback(() => {
    // onboarding completion survives a reset: offering the tour again to someone
    // who only wanted their colours back would read as a bug.
    setPrefs(p => ({ ...DEFAULTS, onboarded: p.onboarded }))
    try { localStorage.removeItem('gs-mode') } catch { /* ignore */ }
  }, [])

  const mode: Mode = prefs.colorMode === 'system' ? (systemDark ? 'dark' : 'light') : prefs.colorMode
  const motionOff = prefs.reducedMotion === 'system' ? systemMotion : prefs.reducedMotion

  const value = useMemo(() => ({ prefs, set, reset, mode, motionOff }), [prefs, set, reset, mode, motionOff])
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function usePrefs(): Ctx {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('usePrefs must be inside PreferencesProvider')
  return ctx
}
