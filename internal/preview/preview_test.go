package preview_test

import (
	"math"
	"os"
	"testing"

	"github.com/darinkes/fit-merger/internal/format"
	"github.com/darinkes/fit-merger/internal/merge"
	"github.com/darinkes/fit-merger/internal/model"
	"github.com/darinkes/fit-merger/internal/preview"
)

// mergedTestData decodes the two sample GPX files and merges them the same way
// the CLI/wasm does, giving a realistic multi-part activity to preview.
func mergedTestData(t *testing.T) model.Activity {
	t.Helper()
	var acts []model.Activity
	for _, name := range []string{"part1.gpx", "part2.gpx"} {
		buf, err := os.ReadFile("../../testdata/" + name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		act, err := format.Decode(buf, format.GPX)
		if err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		act.Sources = []string{name}
		acts = append(acts, act)
	}
	res, err := merge.Merge(acts, merge.Options{Sort: true, Overlap: merge.OverlapError})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	return res.Activity
}

func TestPolylineSplitsByPart(t *testing.T) {
	tr := preview.Polyline(mergedTestData(t), 0)

	if len(tr.Parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(tr.Parts))
	}
	if !tr.HasElevation {
		t.Error("HasElevation = false, want true (test data has <ele>)")
	}
	for i, part := range tr.Parts {
		if len(part) != 5 {
			t.Errorf("part %d: got %d points, want 5", i, len(part))
		}
	}
	// Distance must be continuous and monotonic across the part boundary.
	last := -1.0
	for _, part := range tr.Parts {
		for _, p := range part {
			if p.Dist < last {
				t.Fatalf("distance went backwards: %v after %v", p.Dist, last)
			}
			last = p.Dist
		}
	}
}

func TestPolylineDownsampleKeepsEndpoints(t *testing.T) {
	act := mergedTestData(t)

	// Cap well below the point count and confirm the cap is respected while the
	// first and last point of each part are preserved.
	const max = 4
	tr := preview.Polyline(act, max)

	total := 0
	for _, part := range tr.Parts {
		total += len(part)
	}
	if total > max+len(tr.Parts) { // +1 forced last-point per part
		t.Errorf("downsampled to %d points, want <= %d", total, max+len(tr.Parts))
	}

	// Compare by position: Point now carries channel fields that may be NaN
	// (NaN != NaN would make a whole-struct comparison always fail).
	samePos := func(a, b preview.Point) bool { return a.Lat == b.Lat && a.Lon == b.Lon && a.Dist == b.Dist }
	full := preview.Polyline(act, 0)
	for i := range tr.Parts {
		got, want := tr.Parts[i], full.Parts[i]
		if len(got) < 2 {
			continue
		}
		if !samePos(got[0], want[0]) {
			t.Errorf("part %d: first point not preserved: got %v want %v", i, got[0], want[0])
		}
		if !samePos(got[len(got)-1], want[len(want)-1]) {
			t.Errorf("part %d: last point not preserved: got %v want %v", i, got[len(got)-1], want[len(want)-1])
		}
	}
}

func TestPolylineSeedsLeadingAltitude(t *testing.T) {
	lat, lon := 47.0, 8.0
	ele := 500.0
	// The first positioned record has no altitude yet (e.g. the barometric
	// altimeter hasn't locked); the profile must start at the first known
	// elevation, not fall back to 0 m.
	act := model.Activity{
		Records: []model.Record{
			{Lat: &lat, Lon: &lon},
			{Lat: &lat, Lon: &lon, Altitude: &ele},
		},
	}
	tr := preview.Polyline(act, 0)
	if !tr.HasElevation {
		t.Fatal("HasElevation = false, want true")
	}
	if got := tr.Parts[0][0].Ele; got != ele {
		t.Errorf("leading point elevation = %v, want %v (back-filled from first known sample)", got, ele)
	}
}

func TestPolylineCarriesChannels(t *testing.T) {
	lat, lon := 47.0, 8.0
	hr, pw, cad := uint8(120), uint16(200), uint8(90)
	act := model.Activity{
		Records: []model.Record{
			{Lat: &lat, Lon: &lon}, // channels unknown yet
			{Lat: &lat, Lon: &lon, HR: &hr, Power: &pw, Cadence: &cad}, // first samples
			{Lat: &lat, Lon: &lon}, // dropout -> carry forward
		},
	}
	tr := preview.Polyline(act, 0)
	if !tr.HasHR || !tr.HasPower || !tr.HasCadence {
		t.Fatalf("flags hr:%v pow:%v cad:%v, want all true", tr.HasHR, tr.HasPower, tr.HasCadence)
	}
	p := tr.Parts[0]
	if !math.IsNaN(p[0].HR) {
		t.Errorf("leading HR = %v, want NaN before the first sample", p[0].HR)
	}
	if p[1].HR != 120 || p[1].Power != 200 || p[1].Cadence != 90 {
		t.Errorf("sample point hr:%v pow:%v cad:%v", p[1].HR, p[1].Power, p[1].Cadence)
	}
	if p[2].HR != 120 { // carried across the dropout
		t.Errorf("carried HR = %v, want 120", p[2].HR)
	}
}

func TestPolylineSkipsRecordsWithoutPosition(t *testing.T) {
	lat, lon := 47.0, 8.0
	act := model.Activity{
		Records: []model.Record{
			{Lat: &lat, Lon: &lon},
			{}, // no position — must be skipped
			{Lat: &lat, Lon: &lon},
		},
	}
	tr := preview.Polyline(act, 0)
	if len(tr.Parts) != 1 || len(tr.Parts[0]) != 2 {
		t.Fatalf("got parts %v, want a single part with 2 positioned points", tr.Parts)
	}
	if tr.HasElevation {
		t.Error("HasElevation = true, want false (no altitude set)")
	}
}
