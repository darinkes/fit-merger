// Service worker for the fitmerge PWA: makes the app installable and available
// offline, without ever serving a stale/mismatched engine.
//
// The cache name carries the build version (stamped in at deploy time), so every
// release lands in a fresh namespace and the old one is deleted on activate —
// that's the "clean on update" guarantee. index.html, the wasm engine and JS are
// a version-coupled set, so they're served network-first (always fresh online,
// cache only as an offline fallback); static art (icons, og image, manifest) is
// served stale-while-revalidate for speed.
const VERSION = "dev"; // replaced with the git version at build time
const CACHE = "fitmerge-" + VERSION;
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

// networkFirst keeps the HTML+wasm+JS trio matched: try the network, fall back
// to cache only when offline.
function networkFirst(req) {
  return fetch(req)
    .then((res) => { cachePut(req, res.clone()); return res; })
    .catch(() => caches.match(req).then((h) => h || caches.match("./index.html")));
}

// staleWhileRevalidate serves cached art instantly and refreshes it in the
// background — safe because these assets don't affect correctness.
function staleWhileRevalidate(req) {
  return caches.match(req).then((hit) => {
    const net = fetch(req).then((res) => { cachePut(req, res.clone()); return res; }).catch(() => hit);
    return hit || net;
  });
}

self.addEventListener("fetch", (e) => {
  const req = e.request;
  if (req.method !== "GET") return;
  const url = new URL(req.url);
  if (url.origin !== self.location.origin) return; // OSM tiles etc. go straight to network

  const coupled = req.mode === "navigate" || url.pathname.endsWith("/") || /\.(wasm|js|html)$/.test(url.pathname);
  e.respondWith(coupled ? networkFirst(req) : staleWhileRevalidate(req));
});
