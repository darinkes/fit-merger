package stats

import (
	"testing"
	"time"

	"github.com/darinkes/fitmerge/internal/model"
)

func f(v float64) *float64 { return &v }
func hr(v uint8) *uint8    { return &v }

// track builds records at lat 0 stepping east by 0.001° every 10s, so each
// segment is ~111 m and clearly "moving".
func track(base time.Time, eles ...float64) []model.Record {
	recs := make([]model.Record, len(eles))
	for i, e := range eles {
		recs[i] = model.Record{
			Time:     base.Add(time.Duration(i) * 10 * time.Second),
			Lat:      f(0),
			Lon:      f(0.001 * float64(i)),
			Altitude: f(e),
			HR:       hr(uint8(100 + i)),
		}
	}
	return recs
}

func TestComputeBasics(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	recs := track(base, 100, 105, 104) // +5 then -1 (below threshold)
	s := Compute(recs, DefaultOptions())

	if s.TotalDistance < 221 || s.TotalDistance > 224 {
		t.Errorf("distance = %.2f, want ~222.4", s.TotalDistance)
	}
	if s.TotalAscent != 5 {
		t.Errorf("ascent = %.1f, want 5 (the -1 dip is below the 3m threshold)", s.TotalAscent)
	}
	if s.TotalDescent != 0 {
		t.Errorf("descent = %.1f, want 0", s.TotalDescent)
	}
	if s.TotalMoving != 20*time.Second {
		t.Errorf("moving = %s, want 20s", s.TotalMoving)
	}
	if s.TotalElapsed != 20*time.Second {
		t.Errorf("elapsed = %s, want 20s", s.TotalElapsed)
	}
	if s.AvgSpeed < 11.0 || s.AvgSpeed > 11.3 {
		t.Errorf("avg speed = %.3f m/s, want ~11.1", s.AvgSpeed)
	}
	if s.MaxHR != 102 || s.AvgHR != 101 {
		t.Errorf("HR avg=%d max=%d, want avg=101 max=102", s.AvgHR, s.MaxHR)
	}
}

func TestMovingTimeExcludesPause(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	// Three samples in place (no movement) then a long jump.
	recs := []model.Record{
		{Time: base, Lat: f(0), Lon: f(0), Altitude: f(100)},
		{Time: base.Add(60 * time.Second), Lat: f(0), Lon: f(0), Altitude: f(100)},     // stopped 60s
		{Time: base.Add(70 * time.Second), Lat: f(0), Lon: f(0.001), Altitude: f(100)}, // moving 10s
	}
	s := Compute(recs, DefaultOptions())
	if s.TotalElapsed != 70*time.Second {
		t.Errorf("elapsed = %s, want 70s", s.TotalElapsed)
	}
	if s.TotalMoving != 10*time.Second {
		t.Errorf("moving = %s, want 10s (60s pause excluded)", s.TotalMoving)
	}
}

func TestMovingTimeFromSpans(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	// Records span a continuous 60s; the timer paused 20s..40s, so the device's
	// own moving time is 40s even though the samples never stop.
	recs := track(base, 100, 101, 102, 103, 104, 105, 106) // 7 recs, 10s apart => 60s
	spans := []model.TimeSpan{
		{Start: base, End: base.Add(20 * time.Second)},
		{Start: base.Add(40 * time.Second), End: base.Add(60 * time.Second)},
	}
	if got := MovingTimeFromSpans(recs, spans); got != 40*time.Second {
		t.Errorf("moving from spans = %s, want 40s", got)
	}

	// Spans are clamped to the record range: a span starting before the first
	// record only counts from the first record onward.
	wide := []model.TimeSpan{{Start: base.Add(-time.Minute), End: base.Add(30 * time.Second)}}
	if got := MovingTimeFromSpans(recs, wide); got != 30*time.Second {
		t.Errorf("clamped moving = %s, want 30s", got)
	}

	if got := MovingTimeFromSpans(recs, nil); got != 0 {
		t.Errorf("moving with no spans = %s, want 0", got)
	}
}

func TestAscentThresholdSuppressesJitter(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	// Elevation wobbles ±1m around 100 then climbs 10m for real.
	recs := track(base, 100, 101, 99, 100, 110)
	loose := Compute(recs, Options{AscentThreshold: 3, MovingSpeedThreshold: 0.5})
	if loose.TotalAscent != 10 {
		t.Errorf("ascent with 3m threshold = %.1f, want 10 (jitter ignored)", loose.TotalAscent)
	}
	// With no threshold the jitter is counted, inflating ascent.
	none := Compute(recs, Options{AscentThreshold: 0, MovingSpeedThreshold: 0.5})
	if none.TotalAscent <= loose.TotalAscent {
		t.Errorf("ascent with 0 threshold = %.1f, want > %.1f", none.TotalAscent, loose.TotalAscent)
	}
}
