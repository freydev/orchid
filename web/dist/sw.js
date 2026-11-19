/* orchid service worker.
   Strategy: network-first for the app shell (so rebuilds aren't stale), with a
   cache fallback for offline. Live data — /api and /ws — is never cached
   (WebSockets aren't fetch events anyway; the /api guard keeps stale data out).
   Bump CACHE to force old caches out on the next activate. */
const CACHE = 'orchid-shell-v1';
const SHELL = [
  '/',
  '/index.html',
  '/bundle.js',
  '/bundle.css',
  '/orchid-logo.svg',
  '/icon-192.png',
  '/icon-512.png',
];

self.addEventListener('install', (e) => {
  e.waitUntil(
    caches.open(CACHE)
      .then((c) => c.addAll(SHELL))
      .catch(() => {})
      .then(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (e) => {
  e.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

self.addEventListener('fetch', (e) => {
  const req = e.request;
  if (req.method !== 'GET') return;
  const url = new URL(req.url);
  if (url.origin !== location.origin) return;            // fonts/CDN: let the browser handle
  if (url.pathname.startsWith('/api') || url.pathname.startsWith('/ws')) return; // live data

  e.respondWith(
    fetch(req)
      .then((res) => {
        if (res && res.ok) {
          const copy = res.clone();
          caches.open(CACHE).then((c) => c.put(req, copy)).catch(() => {});
        }
        return res;
      })
      .catch(() =>
        caches.match(req).then((hit) => hit || caches.match('/index.html'))
      )
  );
});
