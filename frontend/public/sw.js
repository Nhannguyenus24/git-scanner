// GitScanner service worker: makes the dashboard installable and lets its shell
// open without a network.
//
// It caches the app shell only — the HTML and the hashed /assets/ files Vite
// emits. It never touches a request to another origin, which is where the API
// lives, and never caches a /v1/ path on this one: findings, previews and
// settings must not come to rest in the browser's cache, and a stale scan
// count would be worse than an honest "API unreachable".
const SHELL = 'gs-shell-v1'

self.addEventListener('install', () => self.skipWaiting())

self.addEventListener('activate', e => {
  e.waitUntil((async () => {
    for (const key of await caches.keys()) if (key !== SHELL) await caches.delete(key)
    await self.clients.claim()
  })())
})

self.addEventListener('fetch', e => {
  const url = new URL(e.request.url)
  if (e.request.method !== 'GET' || url.origin !== self.location.origin || url.pathname.startsWith('/v1/')) return

  // Hashed assets never change under one name: cache first.
  if (url.pathname.startsWith('/assets/')) {
    e.respondWith((async () => {
      const cache = await caches.open(SHELL)
      const hit = await cache.match(e.request)
      if (hit) return hit
      const res = await fetch(e.request)
      if (res.ok) cache.put(e.request, res.clone())
      return res
    })())
    return
  }

  // The page itself: network first, so a deploy shows up on the next load,
  // with the last good copy as the offline fallback.
  if (e.request.mode === 'navigate') {
    e.respondWith((async () => {
      const cache = await caches.open(SHELL)
      try {
        const res = await fetch(e.request)
        if (res.ok) cache.put('/', res.clone())
        return res
      } catch {
        return (await cache.match('/')) ?? Response.error()
      }
    })())
  }
})

// A click on a notification focuses an open dashboard rather than opening a
// second one.
self.addEventListener('notificationclick', e => {
  e.notification.close()
  e.waitUntil((async () => {
    const all = await self.clients.matchAll({ type: 'window', includeUncontrolled: true })
    if (all[0]) return all[0].focus()
    return self.clients.openWindow('/')
  })())
})
