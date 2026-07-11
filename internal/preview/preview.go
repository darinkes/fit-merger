// Package preview builds a lightweight, downsampled polyline of a merged
// activity for the browser's route map and elevation profile. It is kept
// separate from the WebAssembly entrypoint so the sampling logic can be
// unit-tested on any platform (the wasm package can't be run under `go test`).
package preview

import "github.com/darinkes/fit-merger/internal/model"

// Point is one sample of the preview polyline.
type Point struct {
	Lat, Lon, Ele, Dist float64
}

// Track is the merged route split into parts (one per source file / lap),
// plus whether any elevation data was present in the stream.
type Track struct {
	Parts        [][]Point
	HasElevation bool
}

// Polyline groups the activity's positioned records into parts by lap and
// downsamples them to at most maxPoints in total. Elevation carries forward the
// last known altitude so occasional missing samples don't spike the profile,
// and the distance is taken from the (already re-based) record stream. Records
// without a position are skipped; the first and last point of every part are
// always kept so adjacent parts join up. A maxPoints <= 0 disables downsampling.
func Polyline(act model.Activity, maxPoints int) Track {
	nParts := len(act.Laps)
	if nParts == 0 {
		nParts = 1
	}
	grouped := make([][]model.Record, nParts)

	// Walk records once, advancing to the next lap as time passes each lap's
	// end. Only positioned records contribute to the drawn track.
	li, total := 0, 0
	hasEle := false
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
	}

	stride := 1
	if maxPoints > 0 && total > maxPoints {
		stride = (total + maxPoints - 1) / maxPoints
	}

	var lastEle, lastDist float64
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
			if i%stride != 0 && i != len(recs)-1 {
				continue
			}
			pts = append(pts, Point{Lat: *r.Lat, Lon: *r.Lon, Ele: lastEle, Dist: lastDist})
		}
		parts = append(parts, pts)
	}
	return Track{Parts: parts, HasElevation: hasEle}
}
