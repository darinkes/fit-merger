package editing_test

import (
	"testing"
	"time"

	"github.com/darinkes/fit-merger/internal/editing"
	"github.com/darinkes/fit-merger/internal/model"
)

// line builds n records marching due east from (lat0, lon0) in ~fixed steps, so
// cumulative distance grows roughly linearly and is easy to reason about.
func line(n int, lat0, lonStep float64) []model.Record {
	recs := make([]model.Record, n)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		lat := lat0
		lon := float64(i) * lonStep
		recs[i] = model.Record{Time: base.Add(time.Duration(i) * time.Second), Lat: &lat, Lon: &lon}
	}
	return recs
}

func dist(a, b model.Record) float64 {
	// crude equality helper for endpoints
	return *a.Lon - *b.Lon
}

func TestTrimRemovesEnds(t *testing.T) {
	recs := line(11, 0, 0.001) // ~111 m per step at the equator
	trimmed := editing.Trim(recs, 200, 200)
	if len(trimmed) == 0 || len(trimmed) >= len(recs) {
		t.Fatalf("trim kept %d of %d records", len(trimmed), len(recs))
	}
	// The kept span must be strictly inside the original.
	if dist(trimmed[0], recs[0]) <= 0 {
		t.Errorf("start not trimmed: first lon %v vs %v", *trimmed[0].Lon, *recs[0].Lon)
	}
	if dist(recs[len(recs)-1], trimmed[len(trimmed)-1]) <= 0 {
		t.Errorf("end not trimmed: last lon %v vs %v", *trimmed[len(trimmed)-1].Lon, *recs[len(recs)-1].Lon)
	}
}

func TestTrimNoopWhenZero(t *testing.T) {
	recs := line(5, 0, 0.001)
	if got := editing.Trim(recs, 0, 0); len(got) != len(recs) {
		t.Errorf("zero trim changed length: %d != %d", len(got), len(recs))
	}
}

func TestPrivacyZoneHidesEnds(t *testing.T) {
	recs := line(11, 0, 0.001)                        // ~1.11 km total
	out := editing.PrivacyZone(recs, 250, true, true) // ~250 m ≈ 2 steps each end
	if len(out) == 0 || len(out) >= len(recs) {
		t.Fatalf("privacy kept %d of %d records", len(out), len(recs))
	}
	// First/last kept points must be clear of the original endpoints.
	if *out[0].Lon <= *recs[0].Lon {
		t.Errorf("start zone not applied: %v", *out[0].Lon)
	}
	if *out[len(out)-1].Lon >= *recs[len(recs)-1].Lon {
		t.Errorf("end zone not applied: %v", *out[len(out)-1].Lon)
	}
}

func TestPrivacyZoneStartOnly(t *testing.T) {
	recs := line(11, 0, 0.001)
	out := editing.PrivacyZone(recs, 250, true, false)
	if *out[0].Lon <= *recs[0].Lon {
		t.Errorf("start not hidden")
	}
	if *out[len(out)-1].Lon != *recs[len(recs)-1].Lon {
		t.Errorf("end should be untouched, got %v", *out[len(out)-1].Lon)
	}
}
