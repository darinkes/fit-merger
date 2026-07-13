//go:build js && wasm

// Command fitmerge-wasm exposes the merge engine to JavaScript so the tool can
// run entirely in the browser (see the web/ directory and the Pages workflow).
//
// It registers a global `fitmerge` object with:
//
//	fitmerge.version                      -> string
//	fitmerge.merge(inputs, options)       -> result
//
// where inputs is an array of { name: string, data: Uint8Array } and options is
//
//	{ format: "gpx"|"fit"|"tcx", sort: bool, overlap: "error"|"keep"|"trim",
//	  ascentThreshold: number, movingThreshold: number, use3d: bool, sport: string }
//
// and result is
//
//	{ ok: bool, error?: string, filename?: string, data?: Uint8Array,
//	  summary?: {...}, parts?: [{...}] }
//
// Everything runs client-side; the activity files never leave the browser.
package main

import (
	"fmt"
	"syscall/js"

	"github.com/darinkes/fit-merger/internal/format"
	"github.com/darinkes/fit-merger/internal/merge"
	"github.com/darinkes/fit-merger/internal/model"
	"github.com/darinkes/fit-merger/internal/preview"
	"github.com/darinkes/fit-merger/internal/stats"
)

// version is stamped at build time with -ldflags "-X main.version=...".
var version = "dev"

func main() {
	obj := js.Global().Get("Object").New()
	obj.Set("version", version)
	obj.Set("merge", js.FuncOf(mergeFn))
	js.Global().Set("fitmerge", obj)

	// Let the page know the API is ready, then block forever so the exported
	// functions stay callable.
	if ready := js.Global().Get("onFitmergeReady"); ready.Type() == js.TypeFunction {
		ready.Invoke()
	}
	select {}
}

func mergeFn(_ js.Value, args []js.Value) (result any) {
	defer func() {
		if r := recover(); r != nil {
			result = fail(fmt.Sprintf("internal error: %v", r))
		}
	}()

	if len(args) < 1 || args[0].Type() != js.TypeObject {
		return fail("merge(inputs, options): inputs must be an array")
	}
	inputs := args[0]

	var opts js.Value
	if len(args) >= 2 && args[1].Type() == js.TypeObject {
		opts = args[1]
	}

	acts, err := decodeInputs(inputs)
	if err != nil {
		return fail(err.Error())
	}
	if len(acts) == 0 {
		return fail("no input files")
	}

	outKind := format.Kind(optString(opts, "format", "gpx"))
	if outKind != format.GPX && outKind != format.FIT && outKind != format.TCX {
		return fail(fmt.Sprintf("invalid output format %q (want gpx, fit or tcx)", outKind))
	}
	overlap := merge.OverlapStrategy(optString(opts, "overlap", string(merge.OverlapError)))
	switch overlap {
	case merge.OverlapError, merge.OverlapKeep, merge.OverlapTrim:
	default:
		return fail(fmt.Sprintf("invalid overlap %q (want error|keep|trim)", overlap))
	}

	statOpts := stats.Options{
		AscentThreshold:      optFloat(opts, "ascentThreshold", 3.0),
		MovingSpeedThreshold: optFloat(opts, "movingThreshold", 0.5),
		Use3D:                optBool(opts, "use3d", false),
	}
	res, err := merge.Merge(acts, merge.Options{
		Sort:    optBool(opts, "sort", true),
		Overlap: overlap,
		Stats:   statOpts,
	})
	if err != nil {
		return fail(err.Error())
	}
	if sport := optString(opts, "sport", ""); sport != "" {
		res.Activity.Sport = sport
	}

	data, err := format.Encode(outKind, res.Activity, res.Summary)
	if err != nil {
		return fail(err.Error())
	}

	parts := make([]any, len(res.Parts))
	for i, p := range res.Parts {
		parts[i] = summaryToJS(p)
	}
	return map[string]any{
		"ok":       true,
		"filename": "merged." + string(outKind),
		"data":     toUint8Array(data),
		"summary":  summaryToJS(res.Summary),
		"parts":    parts,
		"track":    trackToJS(res.Activity),
	}
}

// maxTrackPoints caps how many points the browser preview receives: enough to
// draw a faithful route and elevation profile, few enough to stay snappy and
// keep the JS<->wasm crossing cheap.
const maxTrackPoints = 2000

// trackToJS marshals the downsampled preview polyline into a JS-friendly shape:
// one flat [lat, lon, ele, dist, ...] array per part, plus a hasElevation flag.
func trackToJS(act model.Activity) map[string]any {
	tr := preview.Polyline(act, maxTrackPoints)
	jsParts := make([]any, len(tr.Parts))
	for i, part := range tr.Parts {
		flat := make([]any, 0, len(part)*4)
		for _, p := range part {
			flat = append(flat, p.Lat, p.Lon, p.Ele, p.Dist)
		}
		jsParts[i] = flat
	}
	return map[string]any{
		"parts":        jsParts,
		"hasElevation": tr.HasElevation,
	}
}

func decodeInputs(inputs js.Value) ([]model.Activity, error) {
	n := inputs.Length()
	acts := make([]model.Activity, 0, n)
	for i := 0; i < n; i++ {
		item := inputs.Index(i)
		name := item.Get("name").String()

		kind, err := format.Detect(name)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		buf := bytesFromUint8Array(item.Get("data"))
		act, err := format.Decode(buf, kind)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		act.Sources = []string{name}
		acts = append(acts, act)
	}
	return acts, nil
}

func summaryToJS(s model.Summary) map[string]any {
	return map[string]any{
		"distance_m":   s.TotalDistance,
		"ascent_m":     s.TotalAscent,
		"descent_m":    s.TotalDescent,
		"moving_s":     s.TotalMoving.Seconds(),
		"elapsed_s":    s.TotalElapsed.Seconds(),
		"avg_speed_ms": s.AvgSpeed,
		"max_speed_ms": s.MaxSpeed,
		"avg_hr":       int(s.AvgHR),
		"max_hr":       int(s.MaxHR),
		"records":      s.Records,
	}
}

func fail(msg string) map[string]any {
	return map[string]any{"ok": false, "error": msg}
}

// --- JS interop helpers ---

func bytesFromUint8Array(v js.Value) []byte {
	buf := make([]byte, v.Get("length").Int())
	js.CopyBytesToGo(buf, v)
	return buf
}

func toUint8Array(b []byte) js.Value {
	u8 := js.Global().Get("Uint8Array").New(len(b))
	js.CopyBytesToJS(u8, b)
	return u8
}

func optString(o js.Value, key, def string) string {
	if o.Type() != js.TypeObject {
		return def
	}
	if v := o.Get(key); v.Type() == js.TypeString {
		return v.String()
	}
	return def
}

func optBool(o js.Value, key string, def bool) bool {
	if o.Type() != js.TypeObject {
		return def
	}
	if v := o.Get(key); v.Type() == js.TypeBoolean {
		return v.Bool()
	}
	return def
}

func optFloat(o js.Value, key string, def float64) float64 {
	if o.Type() != js.TypeObject {
		return def
	}
	if v := o.Get(key); v.Type() == js.TypeNumber {
		return v.Float()
	}
	return def
}
