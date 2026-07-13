# fitmerge

A Go CLI that merges two or more **GPX/FIT/TCX** activity files into a single
GPX, FIT or TCX file, **recomputing every summary figure** — distance,
ascent/descent, moving time, elapsed time, average/max speed, heart rate — so the
merged file's totals faithfully describe the combined track.

Typical use: stitching a multi-part or multi-day activity (a ride split across
several files, a tour recorded day by day) back into one file. Give it a single
input to simply **convert** between formats (e.g. FIT → GPX); the summaries are
recomputed either way. The browser UI exposes this as a Merge / Convert switch.

**🌐 Try it in your browser (no install): <https://darinkes.github.io/fitmerge/>**
— the same engine compiled to WebAssembly; your files never leave the page.

## Status

| Format | Read | Write |
| ------ | :--: | :---: |
| GPX    |  ✅  |  ✅   |
| FIT    |  ✅  |  ✅   |
| TCX    |  ✅  |  ✅   |

All combinations work: GPX, FIT and TCX inputs can be freely mixed and written to
any of the three formats. FIT output carries correct stored `session`/`lap`
summaries, and TCX output carries per-lap totals, so the merged file shows the
right distance, climb and moving time when imported into Garmin Connect, Strava,
etc.

## Install

```sh
make build           # -> ./fitmerge, version stamped from git
# or
go build -o fitmerge ./cmd/fitmerge
# or
go install github.com/darinkes/fitmerge/cmd/fitmerge@latest
```

`make dist` cross-compiles release binaries for linux/macOS/windows into `dist/`.
Requires Go 1.26+.

### Self-host the web UI

The browser UI can be hosted anywhere with just Docker — no Go toolchain and no
separate web server. `make web-docker` builds the WebAssembly UI into an nginx
image and serves it:

```sh
make web-docker              # build + serve on http://localhost:8080 (Ctrl-C to stop)
make web-docker PORT=9000    # publish on a different port
```

Or drive Docker directly (e.g. to run detached):

```sh
docker build -f Dockerfile.web -t fitmerge-web .
docker run --rm -p 8080:80 fitmerge-web
```

Everything still runs client-side; files never leave the browser.

## Usage

```sh
fitmerge [flags] <input1> <input2> [input3...] -o <output>
```

Merge two GPX files, sorted by start time, into one:

```sh
fitmerge -o tour.gpx day1.gpx day2.gpx
```

Mix formats freely — merge a FIT and a GPX file into a FIT:

```sh
fitmerge -o tour.fit morning.fit afternoon.gpx
```

A single input is a valid "merge" too, so `fitmerge` doubles as a converter:

```sh
fitmerge -o ride.fit ride.gpx
```

Preview the merged summary without writing anything:

```sh
fitmerge -dry-run day1.gpx day2.gpx
```

```
Merged activity
───────────────
  part 1:   0.30 km  ↑   24 ↓    0 m  moving 0:00:40  elapsed 0:00:40  avg 27.3 max 27.3 km/h  HR avg 119 max 130
  part 2:   0.30 km  ↑   18 ↓    0 m  moving 0:00:40  elapsed 0:00:40  avg 27.3 max 27.3 km/h  HR avg 128 max 135
  ─────
  TOTAL:    0.61 km  ↑   42 ↓    0 m  moving 0:01:20  elapsed 0:10:40  avg 27.3 max 27.3 km/h  HR avg 123 max 135
```

The per-part breakdown lets you confirm the whole equals the sum of its parts.

### Flags

| Flag | Default | Meaning |
| ---- | ------- | ------- |
| `-o` | — | Output file (`.gpx`/`.fit`); format inferred from the extension |
| `-format` | (from `-o`) | Force output format: `gpx` or `fit` |
| `-sort` | `true` | Order inputs by their first timestamp |
| `-overlap` | `error` | When inputs overlap in time: `error`, `keep`, or `trim` |
| `-ascent-threshold` | `3` | Minimum sustained elevation change counted as climb (m) |
| `-moving-threshold` | `0.5` | Speed below which time is treated as a pause (m/s) |
| `-3d` | `false` | Include elevation when measuring distance |
| `-sport` | — | Override the sport tag on the output |
| `-dry-run` | `false` | Print the merged summary without writing output |
| `-v` | `false` | Verbose |

### Inspecting a file — `fitmerge dump`

Show everything in a single GPX/FIT file: format-specific header/metadata, which
fields are present, a computed summary, per-lap figures, and a record sample.

```sh
fitmerge dump ride.fit          # human-readable
fitmerge dump -records ride.fit # include every record
fitmerge dump -json ride.gpx    # machine-readable
```

For FIT, `dump` shows both the **stored** session summary (what the recording
device wrote, and what Garmin Connect/Strava read) and the **computed** summary
(recomputed from the points) — a handy consistency check.

```
File details (fit)
  Manufacturer:        garmin
  Stored distance:     42350.0 m
  Stored ascent:       512 m
  Stored moving time:  1h38m20s
  ...
Computed summary
  Distance    42.35 km
  Ascent      512 m
  ...
```

## How the numbers are computed

Every total is derived from the merged point stream by one consistent algorithm,
never copied from a source file. This is the only way to guarantee a merged file's
reported totals match its actual points.

- **Distance** — great-circle (haversine) distance between consecutive samples;
  add `-3d` to include the vertical component. FIT records without GPS fall back
  to their stored cumulative distance.
- **Ascent / descent** — accumulated with a hysteresis threshold
  (`-ascent-threshold`): elevation must move at least *N* meters from the current
  reference before it counts. This suppresses GPS altitude jitter, which would
  otherwise inflate climb. (It's exactly why Garmin, Strava, etc. report
  different climb totals — here it's explicit and tunable.)
- **Moving time** — when a FIT input carries timer start/stop events, the sum of
  the device's own timer-on spans (its exact pause detection). Otherwise, and for
  GPX, the sum of the time between samples whose speed exceeds `-moving-threshold`.
- **Elapsed time** — last timestamp minus first, **including** gaps between
  merged files (a gap is a pause, not distance).
- **Average speed** — distance ÷ moving time. **Max speed** — the fastest sample.

**File boundaries matter:** the gap between two consecutively recorded files
(you drove home between rides) is never counted as distance or speed. Each part
is summarized independently and the totals are then combined.

## Architecture

Both formats are decoded into one canonical model, merged there, and encoded
back out — so cross-format merging falls out for free:

```
FIT/GPX ──decode──▶  Activity (model)  ──merge + recompute──▶  Activity  ──encode──▶  FIT/GPX
```

```
cmd/fitmerge       CLI: flag parsing, the merge + dump subcommands, reporting
cmd/fitmerge-wasm  WebAssembly entry point exposing the engine to the browser UI
internal/model     canonical Activity/Record/Summary
internal/geo       haversine / 3D distance
internal/stats     distance, ascent, moving time, speed, HR; part combination
internal/merge     ordering, overlap handling, distance re-basing
internal/gpx       GPX 1.1 codec
internal/fit       FIT codec
internal/format    extension-based codec dispatch (path- and byte-slice APIs)
internal/preview   downsampled route/elevation polyline for the browser preview
web/               static browser UI (index.html) served with the wasm build
```

## Notes & limitations

- **Inputs are ordered by time.** Merging more than one file requires
  timestamps; a file without them is rejected with a clear error (a single
  timeless file can still be converted).
- **Moving time honors FIT timer events when present.** A FIT file's timer
  start/stop events record exactly when the device considered the athlete active,
  so moving time is summed from those spans. Inputs without them (GPX, or FIT
  files that record none) fall back to the `-moving-threshold` speed estimate.
- **FIT developer (custom) fields are not carried across — by design.** The
  canonical model is deliberately standard-fields-only — position, altitude,
  distance, speed, HR, cadence, power, temperature — which is what keeps the
  format-agnostic merge simple. Developer fields are FIT-specific custom metrics;
  when two merged inputs define the same field differently there is no
  unambiguous way to combine them, so they are dropped rather than guessed at.
- **The recording device is preserved.** A merged FIT keeps the original
  manufacturer, product, product name and serial number from the first FIT
  input that has them; a GPX-only merge is stamped with a neutral `development`
  identity.
- **Overlapping inputs** are an error by default; choose `-overlap=trim` or
  `-overlap=keep` to decide explicitly.

## Milestones

- [x] Canonical model, stats engine, merge engine
- [x] GPX read/write, GPX↔GPX merge
- [x] FIT read/write with re-based distance and recomputed `session`/`lap`
- [x] Cross-format merge (FIT+GPX → either)
- [x] Cross-compiled release binaries + in-browser (WebAssembly) UI
- [x] Golden-file tests for the merged GPX/FIT wire output
- [x] FIT timer-event–aware moving time

FIT developer-field preservation is intentionally out of scope — see
[Notes & limitations](#notes--limitations).

## License

[MIT](LICENSE) © Stefan Rinkes
