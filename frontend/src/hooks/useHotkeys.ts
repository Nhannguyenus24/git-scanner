import { useEffect } from 'react'

// A keyboard shortcut must never eat a keystroke meant for a text box. Every
// handler below is skipped while the focus is in an input, a textarea, a select
// or anything contenteditable — except for Escape and the modifier-carrying
// combinations, which are unambiguous.

export const IS_MAC = typeof navigator !== 'undefined' && /mac|iphone|ipad/i.test(navigator.platform || navigator.userAgent)

/** "⌘K" on a Mac, "Ctrl+K" elsewhere — for printing a shortcut in the UI. */
export function shortcutLabel(key: string, mod = true): string {
  if (!mod) return key.toUpperCase()
  return IS_MAC ? `⌘${key.toUpperCase()}` : `Ctrl+${key.toUpperCase()}`
}

function inTextField(el: EventTarget | null): boolean {
  const n = el as HTMLElement | null
  if (!n || !n.tagName) return false
  if (n.isContentEditable) return true
  return ['INPUT', 'TEXTAREA', 'SELECT'].includes(n.tagName)
}

export interface Hotkey {
  /** A single character, or 'Escape' / '?' / 'ArrowDown' etc. */
  key:    string
  /** Requires Ctrl (or ⌘ on a Mac). */
  mod?:   boolean
  shift?: boolean
  /** Allowed to fire while a text field has focus. Default false. */
  inText?: boolean
  run:    (e: KeyboardEvent) => void
}

/**
 * Binds shortcuts for as long as the component is mounted. `keys` is read from a
 * ref-free closure on every event, so a handler always sees current props
 * without the caller having to memoise the array.
 */
export function useHotkeys(keys: Hotkey[], enabled = true) {
  useEffect(() => {
    if (!enabled) return
    const on = (e: KeyboardEvent) => {
      for (const k of keys) {
        const mod = k.mod ? (IS_MAC ? e.metaKey : e.ctrlKey) : !(e.ctrlKey || e.metaKey)
        if (!mod) continue
        if (!!k.shift !== e.shiftKey) continue
        if (e.key.toLowerCase() !== k.key.toLowerCase()) continue
        if (!k.inText && !k.mod && inTextField(e.target)) continue
        e.preventDefault()
        k.run(e)
        return
      }
    }
    window.addEventListener('keydown', on)
    return () => window.removeEventListener('keydown', on)
  })
}
