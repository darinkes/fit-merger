package main

import (
	"testing"
	"time"

	"github.com/darinkes/fitmerge/internal/model"
)

func fp(v float64) *float64 { return &v }
func hp(v uint8) *uint8     { return &v }

func TestScanFields(t *testing.T) {
	recs := []model.Record{
		{Time: time.Now(), Lat: fp(1), Lon: fp(2), Altitude: fp(100)},
		{Time: time.Now(), Lat: fp(1), Lon: fp(2), HR: hp(120)},
	}
	p := scanFields(recs)
	if !p.Position || !p.Altitude || !p.HR {
		t.Errorf("expected position/altitude/hr present, got %+v", p)
	}
	if p.Power || p.Cadence || p.Temp || p.Speed || p.Distance {
		t.Errorf("expected power/cadence/temp/speed/distance absent, got %+v", p)
	}
	if got := p.present(); len(got) != 3 {
		t.Errorf("present() = %v, want 3 entries", got)
	}
	if got := p.missing(); len(got) != 5 {
		t.Errorf("missing() = %v, want 5 entries", got)
	}
}

func TestLapSummaries(t *testing.T) {
	base := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	var recs []model.Record
	for i := 0; i < 6; i++ {
		recs = append(recs, model.Record{
			Time: base.Add(time.Duration(i) * 10 * time.Second),
			Lat:  fp(0), Lon: fp(0.001 * float64(i)), Altitude: fp(100),
		})
	}
	act := model.Activity{
		Records: recs,
		Laps: []model.Lap{
			{StartTime: recs[0].Time, EndTime: recs[2].Time},
			{StartTime: recs[3].Time, EndTime: recs[5].Time},
		},
	}
	laps := lapSummaries(act)
	if len(laps) != 2 {
		t.Fatalf("laps = %d, want 2", len(laps))
	}
	if laps[0].Records != 3 || laps[1].Records != 3 {
		t.Errorf("lap record counts = %d,%d, want 3,3", laps[0].Records, laps[1].Records)
	}
	if laps[0].TotalDistance <= 0 {
		t.Errorf("lap 0 distance = %.2f, want > 0", laps[0].TotalDistance)
	}
}
