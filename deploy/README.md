# Deploying the web UI

The browser version (WebAssembly + `web/index.html`) is **live** at
<https://darinkes.github.io/fit-merger/>.

Deployment is handled by `.github/workflows/pages.yml`: on every push to `main`
it builds `web/fitmerge.wasm`, copies Go's `wasm_exec.js`, and publishes the
`web/` directory to GitHub Pages. Nothing runs server-side — the merge happens
entirely in the visitor's browser.

Prerequisites (already done): the repo is public and Pages is set to the
"GitHub Actions" build source.

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
