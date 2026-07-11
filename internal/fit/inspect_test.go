package fit

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/darinkes/fit-merger/internal/model"
	"github.com/darinkes/fit-merger/internal/stats"
)

func TestInspect(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	var recs []model.Record
	for i := 0; i < 5; i++ {
		recs = append(recs, model.Record{
			Time:     base.Add(time.Duration(i) * 10 * time.Second),
			Lat:      f(47.0),
			Lon:      f(8.0 + 0.001*float64(i)),
			Altitude: f(1000 + 5*float64(i)),
			HR:       h(uint8(120 + i)),
		})
	}
	sum := stats.Compute(recs, stats.DefaultOptions())
	act := model.Activity{
		Sport:   "cycling",
		Records: recs,
		Laps:    []model.Lap{{StartTime: recs[0].Time, EndTime: recs[4].Time, Summary: sum}},
	}

	path := filepath.Join(t.TempDir(), "a.fit")
	if err := WriteFile(path, act, sum); err != nil {
		t.Fatal(err)
	}

	kv, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, p := range kv {
		m[p[0]] = p[1]
	}

	if m["File type"] != "activity" {
		t.Errorf("File type = %q, want activity", m["File type"])
	}
	if m["Manufacturer"] == "" {
		t.Error("Manufacturer missing")
	}
	if m["Stored sport"] != "cycling" {
		t.Errorf("Stored sport = %q, want cycling", m["Stored sport"])
	}
	if _, ok := m["Stored distance"]; !ok {
		t.Error("Stored distance missing")
	}
	if m["Stored max HR"] != "124 bpm" {
		t.Errorf("Stored max HR = %q, want 124 bpm", m["Stored max HR"])
	}
}
