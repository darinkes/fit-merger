package fit

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"

	"github.com/darinkes/fit-merger/internal/model"
	"github.com/darinkes/fit-merger/internal/stats"
)

func f(v float64) *float64 { return &v }
func h(v uint8) *uint8     { return &v }

func TestRoundTrip(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	var recs []model.Record
	for i := 0; i < 5; i++ {
		recs = append(recs, model.Record{
			Time:     base.Add(time.Duration(i) * 10 * time.Second),
			Lat:      f(47.0),
			Lon:      f(8.0 + 0.001*float64(i)),
			Altitude: f(1000 + 5*float64(i)),
			Distance: f(76 * float64(i)),
			HR:       h(uint8(120 + i)),
		})
	}
	sum := stats.Compute(recs, stats.DefaultOptions())
	act := model.Activity{
		Sport:   "cycling",
		Records: recs,
		Laps:    []model.Lap{{StartTime: recs[0].Time, EndTime: recs[4].Time, Summary: sum}},
	}

	path := filepath.Join(t.TempDir(), "out.fit")
	if err := WriteFile(path, act, sum); err != nil {
		t.Fatal(err)
	}

	// The record stream survives the binary round-trip.
	got, err := ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Records) != len(recs) {
		t.Fatalf("records = %d, want %d", len(got.Records), len(recs))
	}
	if got.Sport != "cycling" {
		t.Errorf("sport = %q, want cycling", got.Sport)
	}
	r0 := got.Records[0]
	if r0.Lat == nil || math.Abs(*r0.Lat-47.0) > 1e-4 {
		t.Errorf("lat = %v, want ~47.0", deref(r0.Lat))
	}
	if r0.Lon == nil || math.Abs(*r0.Lon-8.0) > 1e-4 {
		t.Errorf("lon = %v, want ~8.0", deref(r0.Lon))
	}
	if r0.HR == nil || *r0.HR != 120 {
		t.Errorf("hr = %v, want 120", r0.HR)
	}
	if r0.Altitude == nil || math.Abs(*r0.Altitude-1000) > 0.5 {
		t.Errorf("altitude = %v, want ~1000", deref(r0.Altitude))
	}

	// The *stored* session summary — the figures other tools read directly —
	// matches the recomputed totals.
	sess := decodeSession(t, path)
	if d := sess.TotalDistanceScaled(); math.Abs(d-sum.TotalDistance) > 0.1 {
		t.Errorf("stored session distance = %.2f, want %.2f", d, sum.TotalDistance)
	}
	if int(sess.TotalAscent) != int(math.Round(sum.TotalAscent)) {
		t.Errorf("stored session ascent = %d, want %.0f", sess.TotalAscent, sum.TotalAscent)
	}
	if tt := sess.TotalTimerTimeScaled(); math.Abs(tt-sum.TotalMoving.Seconds()) > 0.1 {
		t.Errorf("stored session timer = %.1f, want %.1f", tt, sum.TotalMoving.Seconds())
	}
	if sess.MaxHeartRate != sum.MaxHR {
		t.Errorf("stored session maxHR = %d, want %d", sess.MaxHeartRate, sum.MaxHR)
	}
}

// TestTimerEventsBracketParts verifies that a merged, multi-part activity is
// written with a timer start/stop_all pair around each part, so the gap between
// parts (e.g. a lunch stop) reads as an explicit recording pause rather than a
// naked hole in the record stream.
func TestTimerEventsBracketParts(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	part := func(start time.Time) ([]model.Record, model.Summary) {
		var recs []model.Record
		for i := 0; i < 5; i++ {
			recs = append(recs, model.Record{
				Time:     start.Add(time.Duration(i) * 10 * time.Second),
				Lat:      f(47.0),
				Lon:      f(8.0 + 0.001*float64(i)),
				Distance: f(76 * float64(i)),
			})
		}
		return recs, stats.Compute(recs, stats.DefaultOptions())
	}

	// Two parts an hour apart: the gap stands in for a lunch break.
	recsA, sumA := part(base)
	recsB, sumB := part(base.Add(time.Hour))

	all := append(append([]model.Record{}, recsA...), recsB...)
	act := model.Activity{
		Sport:   "cycling",
		Records: all,
		Laps: []model.Lap{
			{StartTime: sumA.StartTime, EndTime: sumA.EndTime, Summary: sumA},
			{StartTime: sumB.StartTime, EndTime: sumB.EndTime, Summary: sumB},
		},
	}

	path := filepath.Join(t.TempDir(), "out.fit")
	if err := WriteFile(path, act, stats.Compute(all, stats.DefaultOptions())); err != nil {
		t.Fatal(err)
	}

	events := decodeEvents(t, path)
	// Expect start, stop_all, start, stop_all — bracketing each of the two parts.
	want := []struct {
		et typedef.EventType
		ts time.Time
	}{
		{typedef.EventTypeStart, sumA.StartTime},
		{typedef.EventTypeStopAll, sumA.EndTime},
		{typedef.EventTypeStart, sumB.StartTime},
		{typedef.EventTypeStopAll, sumB.EndTime},
	}
	if len(events) != len(want) {
		t.Fatalf("got %d timer events, want %d", len(events), len(want))
	}
	for i, w := range want {
		e := events[i]
		if e.Event != typedef.EventTimer {
			t.Errorf("event %d: event = %v, want timer", i, e.Event)
		}
		if e.EventType != w.et {
			t.Errorf("event %d: event_type = %v, want %v", i, e.EventType, w.et)
		}
		if !e.Timestamp.Equal(w.ts) {
			t.Errorf("event %d: timestamp = %s, want %s", i, e.Timestamp, w.ts)
		}
	}

	// A merge is written as a single lap spanning the whole activity, no matter
	// how many parts were combined — the part boundaries live in the events.
	if n := decodeLapCount(t, path); n != 1 {
		t.Errorf("laps = %d, want 1", n)
	}
}

// decodeLapCount returns how many lap messages the file contains.
func decodeLapCount(t *testing.T, path string) int {
	t.Helper()
	fp, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fp.Close()
	fit, err := decoder.New(fp).Decode()
	if err != nil {
		t.Fatal(err)
	}
	return len(filedef.NewActivity(fit.Messages...).Laps)
}

// decodeEvents returns the file's timer events in encoded (timestamp) order.
func decodeEvents(t *testing.T, path string) []*mesgdef.Event {
	t.Helper()
	fp, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fp.Close()
	fit, err := decoder.New(fp).Decode()
	if err != nil {
		t.Fatal(err)
	}
	a := filedef.NewActivity(fit.Messages...)
	var timers []*mesgdef.Event
	for _, e := range a.Events {
		if e.Event == typedef.EventTimer {
			timers = append(timers, e)
		}
	}
	return timers
}

func decodeSession(t *testing.T, path string) *mesgdef.Session {
	t.Helper()
	fp, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer fp.Close()
	fit, err := decoder.New(fp).Decode()
	if err != nil {
		t.Fatal(err)
	}
	a := filedef.NewActivity(fit.Messages...)
	if len(a.Sessions) == 0 {
		t.Fatal("no session message written")
	}
	return a.Sessions[0]
}

func deref(p *float64) float64 {
	if p == nil {
		return math.NaN()
	}
	return *p
}
