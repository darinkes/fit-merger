// Package preview builds a lightweight, downsampled polyline of a merged
// activity for the browser's route map, elevation profile and data charts. It is
// kept separate from the WebAssembly entrypoint so the sampling logic can be
// unit-tested on any platform (the wasm package can't be run under `go test`).
package preview

import (
	"math"

	"github.com/darinkes/fit-merger/internal/model"
)

// Point is one sample of the preview polyline. HR, Cadence and Power are NaN
// until the first known sample of that channel (so a chart starts where its data
// does), then carry the last known value across brief dropouts.
type Point struct {
	Lat, Lon, Ele, Dist float64
	HR, Cadence, Power  float64
}

// Track is the merged route split into parts (one per source file / lap), plus
// flags for which optional channels appear anywhere in the stream.
type Track struct {
	Parts        [][]Point
	HasElevation bool
	HasHR        bool
	HasCadence   bool
	HasPower     bool
}

// Polyline groups the activity's positioned records into parts by lap and
// downsamples them to at most maxPoints in total. Elevation carries forward the
// last known altitude so occasional missing samples don't spike the profile, and
// the distance is taken from the (already re-based) record stream; heart rate,
// cadence and power carry forward the same way. Records without a position are
// skipped; the first and last point of every part are always kept so adjacent
// parts join up. A maxPoints <= 0 disables downsampling.
func Polyline(act model.Activity, maxPoints int) Track {
	nParts := len(act.Laps)
	if nParts == 0 {
		nParts = 1
	}
	grouped := make([][]model.Record, nParts)

	// Walk records once, advancing to the next lap as time passes each lap's
	// end. Only positioned records contribute to the drawn track.
	li, total := 0, 0
	var hasEle, hasHR, hasCad, hasPow bool
	for _, r := range act.Records {
		if !r.HasPosition() {
			continue
		}
		for li < len(act.Laps)-1 && r.Time.After(act.Laps[li].EndTime) {
			li++
		}
		grouped[li] = append(grouped[li], r)
		total++
		if r.Altitude != nil {
			hasEle = true
		}
		if r.HR != nil {
			hasHR = true
		}
		if r.Cadence != nil {
			hasCad = true
		}
		if r.Power != nil {
			hasPow = true
		}
	}

	stride := 1
	if maxPoints > 0 && total > maxPoints {
		stride = (total + maxPoints - 1) / maxPoints
	}

	// Seed the carried-forward altitude with the first known sample so leading
	// position-only records (before the altimeter locks) read the real starting
	// elevation instead of 0 m, which would otherwise sink the profile's minimum.
	var lastEle, lastDist float64
seed:
	for _, recs := range grouped {
		for _, r := range recs {
			if r.Altitude != nil {
				lastEle = *r.Altitude
				break seed
			}
		}
	}
	// Channels start "unknown" (NaN) so a chart begins where its data does.
	lastHR, lastCad, lastPow := math.NaN(), math.NaN(), math.NaN()

	parts := make([][]Point, 0, nParts)
	for _, recs := range grouped {
		pts := make([]Point, 0, len(recs)/stride+1)
		for i, r := range recs {
			if r.Altitude != nil {
				lastEle = *r.Altitude
			}
			if r.Distance != nil {
				lastDist = *r.Distance
			}
			if r.HR != nil {
				lastHR = float64(*r.HR)
			}
			if r.Cadence != nil {
				lastCad = float64(*r.Cadence)
			}
			if r.Power != nil {
				lastPow = float64(*r.Power)
			}
			if i%stride != 0 && i != len(recs)-1 {
				continue
			}
			pts = append(pts, Point{
				Lat: *r.Lat, Lon: *r.Lon, Ele: lastEle, Dist: lastDist,
				HR: lastHR, Cadence: lastCad, Power: lastPow,
			})
		}
		parts = append(parts, pts)
	}
	return Track{Parts: parts, HasElevation: hasEle, HasHR: hasHR, HasCadence: hasCad, HasPower: hasPow}
}
