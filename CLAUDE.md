# CLAUDE.md

Guidance for Claude Code when working in this repository.

## What this is

`fitmerge` is a Go CLI that merges two or more GPX/FIT activity files into a
single GPX or FIT file, **recomputing every summary figure** (distance, ascent,
moving/elapsed time, speed, heart rate) from the merged point stream. The same
engine is compiled to WebAssembly for an in-browser UI (`web/`, deployed to
GitHub Pages).

Module: `github.com/darinkes/fit-merger`. Requires Go 1.26+.

## Commands

```sh
make build      # -> ./fitmerge, version stamped from `git describe`
make test       # go test ./...
make vet        # go vet ./...
make fmt        # gofmt -w .
make web        # build web/fitmerge.wasm + copy wasm_exec.js
make dist       # cross-compile release binaries into dist/
make all        # vet + test + build
```

Run a single package's tests: `go test ./internal/stats/`. Run one test:
`go test ./internal/merge/ -run TestMergeSortsInputs -v`.

The Makefile uses Unix tools (`rm`, `cp`, `mkdir -p`), so on Windows run `make`
from Git Bash, or invoke the underlying `go` commands directly.

### Before committing

CI enforces all of these — run them locally:

```sh
gofmt -l .                 # must print nothing
go vet ./...
go test ./...
go mod tidy                # go.mod/go.sum must not change
GOOS=js GOARCH=wasm go build -o /dev/null ./cmd/fitmerge-wasm  # wasm still builds
```

`go build ./...` / `go vet ./...` do **not** cover `cmd/fitmerge-wasm` (it's
behind a `js && wasm` build tag, with a stub for other platforms), so check the
wasm target explicitly after touching anything it imports.

## Architecture

Both formats decode into one canonical model, are merged there, and encode back
out — so cross-format merging (e.g. FIT+GPX → FIT) falls out for free:

```
FIT/GPX ──decode──▶ model.Activity ──merge + recompute──▶ Activity ──encode──▶ FIT/GPX
```

| Package | Responsibility |
| --- | --- |
| `cmd/fitmerge` | CLI: flags, the `merge` (default) and `dump` subcommands, text/JSON reporting |
| `cmd/fitmerge-wasm` | WebAssembly entry point; exposes `fitmerge.merge(inputs, opts)` to JS |
| `internal/model` | Canonical `Activity`/`Record`/`Lap`/`Summary`/`Device` |
| `internal/geo` | Haversine + 3D distance |
| `internal/stats` | Recompute summaries from points; combine per-part summaries |
| `internal/merge` | Ordering, overlap handling, distance re-basing across files |
| `internal/gpx` | GPX 1.1 codec (`github.com/tkrajina/gpxgo`) |
| `internal/fit` | FIT codec (`github.com/muktihari/fit`) |
| `internal/format` | Extension-based codec dispatch; path- **and** byte-slice APIs |
| `internal/preview` | Downsampled route/elevation polyline for the browser preview |
| `web/` | Static single-page UI (`index.html`) served alongside the wasm build |

## Core invariants — do not break these

- **Summaries are always recomputed from the point stream, never copied from a
  source file's stored summary.** This is the whole point of the tool: after a
  merge the reported totals must match the actual points. `stats.Compute` is the
  single source of truth.
- **File boundaries are not distance.** Each part is summarized independently and
  `stats.Combine` sums them, so the geographic/temporal gap *between* two merged
  files (you drove home between rides) counts as elapsed time but never as
  distance or speed. Do not "simplify" this into one `Compute` over all merged
  records — that would fold the gap into distance/speed.
- **Cumulative distance is monotonic across boundaries.** `merge` re-bases each
  part's within-part distance by the running total; the boundary segment is never
  measured. FIT records carry this cumulative `Distance` so the record stream and
  the stored `session` total agree.
- **`Record`'s optional fields are pointers** (`*float64`, `*uint8`, …) so
  "field absent" is distinct from a genuine `0`. Preserve this when adding fields;
  a `nil` altitude must never be treated as `0 m`.
- **FIT output brackets each part with timer `start`/`stop_all` events** so
  inter-part gaps read as recording pauses (importers like Strava otherwise crop
  at a naked gap). A merge writes a *single* lap spanning the whole activity; the
  part boundaries live in the events, not in laps.
- **The recording device is preserved** through a merge (first FIT input that has
  one wins); GPX-only merges get a neutral `development` identity. See
  `model.Device.IsZero`.

## Tunable definitions (`stats.Options`)

Two figures are matters of definition, not measurement, and are exposed as flags:
- `AscentThreshold` (`-ascent-threshold`, default 3 m): hysteresis that suppresses
  GPS altitude jitter before counting climb. This is why tools disagree on ascent.
- `MovingSpeedThreshold` (`-moving-threshold`, default 0.5 m/s): below this, time
  is a pause and excluded from moving time.
- `Use3D` (`-3d`): include the vertical component in distance.

## Conventions

- Standard Go style; `gofmt` is enforced by CI. Package-level doc comments live in
  the primary file of each package (e.g. `stats.go`, `gpx/read.go`).
- Tests are plain `testing` (no framework). Shared GPX fixtures live in
  `testdata/part1.gpx` / `part2.gpx` (two 5-point cycling tracks 10 minutes
  apart). `internal/preview` and `internal/format` tests merge these to get a
  realistic multi-part activity.
- The wasm package can't be run under `go test`, so any logic worth testing (e.g.
  polyline downsampling) lives in a normal package like `internal/preview` and is
  tested there.
- Version is injected via `-ldflags "-X main.version=..."` (from `git describe`);
  `go install` builds fall back to the module version.

## CI / deploy (`.github/workflows/`)

- `ci.yml` — build/vet/test on Linux/macOS/Windows; `-race` + coverage; gofmt,
  vet, and `go mod tidy` cleanliness.
- `pages.yml` — on push to `main`, build `web/fitmerge.wasm`, copy `wasm_exec.js`,
  publish `web/` to GitHub Pages. Everything runs client-side.
- `release.yml` — on a `v*` tag, `make dist` and publish a GitHub release.
