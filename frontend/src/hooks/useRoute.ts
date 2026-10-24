import { useEffect, useState } from 'react'

// Hash routing keeps every view linkable and survives a reload without a
// router dependency or any server-side fallback route.

export interface Route { page: string; id?: string; params: URLSearchParams }

function parse(): Route {
  const raw = window.location.hash.replace(/^#\/?/, '')
  const [path, qs = ''] = raw.split('?')
  const [page = 'overview', id] = path.split('/').filter(Boolean)
  return { page, id, params: new URLSearchParams(qs) }
}

export function useRoute(): Route {
  const [route, setRoute] = useState(parse)
  useEffect(() => {
    const on = () => setRoute(parse())
    window.addEventListener('hashchange', on)
    return () => window.removeEventListener('hashchange', on)
  }, [])
  return route
}

export function href(page: string, id?: string, params?: Record<string, string | undefined>): string {
  const q = new URLSearchParams()
  for (const [k, v] of Object.entries(params ?? {})) if (v) q.set(k, v)
  const s = q.toString()
  return `#/${page}${id ? `/${id}` : ''}${s ? `?${s}` : ''}`
}

export function navigate(to: string) { window.location.hash = to.replace(/^#/, '') }
