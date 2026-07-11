package merge

import (
	"testing"
	"time"

	"github.com/darinkes/fit-merger/internal/model"
	"github.com/darinkes/fit-merger/internal/stats"
)

func f(v float64) *float64 { return &v }

func track(name string, base time.Time, n int) model.Activity {
	recs := make([]model.Record, n)
	for i := 0; i < n; i++ {
		recs[i] = model.Record{
			Time:     base.Add(time.Duration(i) * 10 * time.Second),
			Lat:      f(0),
			Lon:      f(0.001 * float64(i)),
			Altitude: f(100 + 5*float64(i)),
		}
	}
	return model.Activity{Sport: "cycling", Records: recs, Sources: []string{name}}
}

func defaults() Options {
	return Options{Sort: true, Overlap: OverlapError, Stats: stats.DefaultOptions()}
}

func TestMergeConcatenatesAndSumsTotals(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	a := track("a", base, 5)
	b := track("b", base.Add(10*time.Minute), 5) // 10-minute gap after a

	res, err := Merge([]model.Activity{a, b}, defaults())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Activity.Records) != 10 {
		t.Fatalf("records = %d, want 10", len(res.Activity.Records))
	}
	if len(res.Parts) != 2 {
		t.Fatalf("parts = %d, want 2", len(res.Parts))
	}

	// Total distance/ascent/moving are the sum of the parts...
	wantDist := res.Parts[0].TotalDistance + res.Parts[1].TotalDistance
	if diff := res.Summary.TotalDistance - wantDist; diff < -0.01 || diff > 0.01 {
		t.Errorf("total distance = %.2f, want sum of parts %.2f", res.Summary.TotalDistance, wantDist)
	}
	wantAsc := res.Parts[0].TotalAscent + res.Parts[1].TotalAscent
	if res.Summary.TotalAscent != wantAsc {
		t.Errorf("total ascent = %.1f, want %.1f", res.Summary.TotalAscent, wantAsc)
	}
	wantMoving := res.Parts[0].TotalMoving + res.Parts[1].TotalMoving
	if res.Summary.TotalMoving != wantMoving {
		t.Errorf("total moving = %s, want %s", res.Summary.TotalMoving, wantMoving)
	}

	// ...but elapsed spans the whole activity, the 10-minute gap included.
	if res.Summary.TotalElapsed <= wantMoving {
		t.Errorf("elapsed %s should exceed moving %s (gap counts as elapsed)",
			res.Summary.TotalElapsed, wantMoving)
	}
}

func TestMergeCumulativeDistanceIsMonotonic(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	a := track("a", base, 5)
	b := track("b", base.Add(10*time.Minute), 5)

	res, err := Merge([]model.Activity{a, b}, defaults())
	if err != nil {
		t.Fatal(err)
	}
	prev := -1.0
	for i, r := range res.Activity.Records {
		if r.Distance == nil {
			t.Fatalf("record %d has nil distance", i)
		}
		if *r.Distance < prev {
			t.Fatalf("distance not monotonic at %d: %.2f < %.2f", i, *r.Distance, prev)
		}
		prev = *r.Distance
	}
	// The boundary between files must not add the geographic jump as distance:
	// last cumulative == sum of the two parts' distances.
	want := res.Parts[0].TotalDistance + res.Parts[1].TotalDistance
	if diff := prev - want; diff < -0.01 || diff > 0.01 {
		t.Errorf("final cumulative distance = %.2f, want %.2f", prev, want)
	}
}

func TestMergeSortsInputs(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	late := track("late", base.Add(time.Hour), 3)
	early := track("early", base, 3)

	res, err := Merge([]model.Activity{late, early}, defaults())
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Activity.Sources; got[0] != "early" || got[1] != "late" {
		t.Errorf("sources order = %v, want [early late]", got)
	}
}

func TestMergeThreeFiles(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	a := track("a", base, 4)
	b := track("b", base.Add(5*time.Minute), 4)
	c := track("c", base.Add(10*time.Minute), 4)

	res, err := Merge([]model.Activity{a, b, c}, defaults())
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Parts) != 3 {
		t.Fatalf("parts = %d, want 3", len(res.Parts))
	}
	if len(res.Activity.Records) != 12 {
		t.Errorf("records = %d, want 12", len(res.Activity.Records))
	}
	want := res.Parts[0].TotalDistance + res.Parts[1].TotalDistance + res.Parts[2].TotalDistance
	if diff := res.Summary.TotalDistance - want; diff < -0.01 || diff > 0.01 {
		t.Errorf("total distance = %.2f, want sum of 3 parts %.2f", res.Summary.TotalDistance, want)
	}
}

func TestMergeRejectsMissingTimestamps(t *testing.T) {
	a := track("a", time.Time{}, 3) // zero base => zero timestamps
	b := track("b", time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC), 3)
	if _, err := Merge([]model.Activity{a, b}, defaults()); err == nil {
		t.Fatal("expected error for input without timestamps, got nil")
	}
}

func TestMergeCarriesDevice(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	early := track("early", base, 3) // no device, sorts first
	late := track("late", base.Add(time.Hour), 3)
	late.Device = &model.Device{Manufacturer: 1, Product: 3121}

	res, err := Merge([]model.Activity{early, late}, defaults())
	if err != nil {
		t.Fatal(err)
	}
	// The device is kept from the first input that has one, regardless of order.
	if res.Activity.Device == nil {
		t.Fatal("merged activity dropped the device")
	}
	if res.Activity.Device.Manufacturer != 1 || res.Activity.Device.Product != 3121 {
		t.Errorf("device = %+v, want {Manufacturer:1 Product:3121}", *res.Activity.Device)
	}
}

func TestMergeOverlapErrors(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	a := track("a", base, 5)
	b := track("b", base.Add(20*time.Second), 5) // starts before a ends (a ends +40s)

	if _, err := Merge([]model.Activity{a, b}, defaults()); err == nil {
		t.Fatal("expected overlap error, got nil")
	}
}

func TestMergeOverlapTrim(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	a := track("a", base, 5)                     // ends at +40s
	b := track("b", base.Add(20*time.Second), 5) // first 3 samples (+20,+30,+40) overlap

	opts := defaults()
	opts.Overlap = OverlapTrim
	res, err := Merge([]model.Activity{a, b}, opts)
	if err != nil {
		t.Fatal(err)
	}
	// a's 5 records + b's 2 non-overlapping (+50s, +60s) = 7.
	if len(res.Activity.Records) != 7 {
		t.Errorf("records = %d, want 7 after trimming overlap", len(res.Activity.Records))
	}
}
