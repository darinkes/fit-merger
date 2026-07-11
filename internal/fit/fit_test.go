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

	"github.com/rinkes/fit-merger/internal/model"
	"github.com/rinkes/fit-merger/internal/stats"
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
	if err := Write(path, act, sum); err != nil {
		t.Fatal(err)
	}

	// The record stream survives the binary round-trip.
	got, err := Read(path)
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
