// Service worker for the fitmerge PWA: makes the app installable and available
// offline. It precaches the small app shell on install; the ~6 MB wasm engine is
// NOT precached (that would defeat the lazy load) — it is cached on first use via
// the stale-while-revalidate handler below, so offline merges work after one
// online run. OpenStreetMap tiles are cross-origin and never cached.
const CACHE = "fitmerge-v1";
const SHELL = [
  "./",
  "./index.html",
  "./wasm_exec.js",
  "./favicon.svg",
  "./manifest.webmanifest",
  "./icon-192.png",
  "./icon-512.png",
  "./icon-maskable.png",
  "./og-image.png",
];

self.addEventListener("install", (e) => {
  e.waitUntil(caches.open(CACHE).then((c) => c.addAll(SHELL)).then(() => self.skipWaiting()));
});

self.addEventListener("activate", (e) => {
  e.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.filter((k) => k !== CACHE).map((k) => caches.delete(k))))
      .then(() => self.clients.claim())
  );
});

async function cachePut(req, res) {
  if (res && res.ok) {
    const c = await caches.open(CACHE);
    await c.put(req, res);
  }
}

self.addEventListener("fetch", (e) => {
  const req = e.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return; // OSM tiles etc. go straight to network

  // HTML navigations: network-first so a new deploy shows immediately; fall back
  // to the cached shell when offline.
  if (req.mode === "navigate") {
    e.respondWith(
      fetch(req)
        .then((res) => { cachePut(req, res.clone()); return res; })
        .catch(() => caches.match(req).then((h) => h || caches.match("./index.html")))
    );
    return;
  }

  // Everything else (wasm, JS, images): serve cached instantly, refresh in the
  // background.
  e.respondWith(
    caches.match(req).then((hit) => {
      const net = fetch(req).then((res) => { cachePut(req, res.clone()); return res; }).catch(() => hit);
      return hit || net;
    })
  );
});
