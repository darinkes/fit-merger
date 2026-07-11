# Deploying the web UI

The browser version (WebAssembly + `web/index.html`) is built and ready. Hosting
is **on hold**: GitHub Pages needs a **public** repo on the free plan, or GitHub
Pro for a private one. `pages.yml` lives here (inert) rather than in
`.github/workflows/` so it doesn't run until you activate it.

## Go live on GitHub Pages (when ready)

1. Make the repo public (Settings → General → Danger Zone) **or** upgrade to GitHub Pro.
2. Enable Pages with the GitHub Actions source:
   ```sh
   gh api --method POST repos/darinkes/fit-merger/pages -f build_type=workflow
   ```
3. Activate the workflow and push:
   ```sh
   git mv deploy/pages.yml .github/workflows/pages.yml
   git commit -m "Enable Pages deploy" && git push
   ```

It publishes to `https://darinkes.github.io/fit-merger/` and redeploys on every
push to `main`.

## Run it locally (works today, no hosting needed)

WebAssembly won't load over `file://`, so serve the `web/` directory over HTTP.

```sh
make web                              # builds web/fitmerge.wasm + copies wasm_exec.js
cd web && python -m http.server 8080  # or any static file server
# open http://localhost:8080
```

Windows PowerShell, without `make`:

```powershell
$env:GOOS="js"; $env:GOARCH="wasm"; go build -o web/fitmerge.wasm ./cmd/fitmerge-wasm
Copy-Item "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/
cd web; python -m http.server 8080
```
