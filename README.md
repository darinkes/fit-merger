# fitmerge

A Go CLI that merges two or more **GPX/FIT** activity files into a single GPX or
FIT file, **recomputing every summary figure** — distance, ascent/descent,
moving time, elapsed time, average/max speed, heart rate — so the merged file's
totals faithfully describe the combined track.

Typical use: stitching a multi-part or multi-day activity (a ride split across
several files, a tour recorded day by day) back into one file.

## Status

| Format | Read | Write |
| ------ | :--: | :---: |
| GPX    |  ✅  |  ✅   |
| FIT    |  🚧  |  🚧   |

GPX↔GPX merging is complete. FIT support and cross-format merging (FIT+GPX → either)
are in progress — see [Roadmap](#roadmap).

## Install

```sh
go build -o fitmerge ./cmd/fitmerge
```

Requires Go 1.26+.

## Usage

```sh
fitmerge [flags] <input1> <input2> [input3...] -o <output>
```

Merge two GPX files, sorted by start time, into one:

```sh
fitmerge -o tour.gpx day1.gpx day2.gpx
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
- **Moving time** — sum of the time between samples whose speed exceeds
  `-moving-threshold`; pauses are excluded.
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
cmd/fitmerge     CLI, flag parsing, summary report
internal/model   canonical Activity/Record/Summary
internal/geo     haversine / 3D distance
internal/stats   distance, ascent, moving time, speed, HR; part combination
internal/merge   ordering, overlap handling, distance re-basing
internal/gpx     GPX 1.1 codec
internal/fit     FIT codec (in progress)
internal/format  extension-based codec dispatch
```

## Roadmap

- [x] Canonical model, stats engine, merge engine
- [x] GPX read/write, GPX↔GPX merge
- [ ] FIT read/write with re-based distance and recomputed `session`/`lap`
- [ ] Cross-format merge (FIT+GPX → either)
- [ ] FIT timer-event–aware moving time; developer-field preservation
- [ ] Golden-file tests, release builds

## License

TBD.
