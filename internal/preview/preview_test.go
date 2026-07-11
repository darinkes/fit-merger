package preview_test

import (
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

	full := preview.Polyline(act, 0)
	for i := range tr.Parts {
		got, want := tr.Parts[i], full.Parts[i]
		if len(got) < 2 {
			continue
		}
		if got[0] != want[0] {
			t.Errorf("part %d: first point not preserved: got %v want %v", i, got[0], want[0])
		}
		if got[len(got)-1] != want[len(want)-1] {
			t.Errorf("part %d: last point not preserved: got %v want %v", i, got[len(got)-1], want[len(want)-1])
		}
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
