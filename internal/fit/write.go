package fit

import (
	"fmt"
	"math"
	"os"

	"github.com/muktihari/fit/encoder"
	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"

	"github.com/darinkes/fit-merger/internal/model"
)

// Write encodes an Activity as a FIT activity file. The passed summary supplies
// the recomputed session/activity aggregates; per-lap aggregates come from the
// activity's laps. Distance is taken from each record's (already re-based)
// cumulative Distance field so the record stream and the session total agree.
func Write(path string, act model.Activity, summary model.Summary) error {
	a := buildActivity(act, summary)
	fit := a.ToFIT(nil)

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := encoder.New(f).Encode(&fit); err != nil {
		return fmt.Errorf("encode fit %q: %w", path, err)
	}
	return nil
}

func buildActivity(act model.Activity, s model.Summary) *filedef.Activity {
	a := filedef.NewActivity()
	a.FileId.SetType(typedef.FileActivity).
		SetManufacturer(typedef.ManufacturerDevelopment).
		SetProduct(1).
		SetTimeCreated(s.StartTime)

	for _, r := range act.Records {
		a.Records = append(a.Records, recordFromModel(r))
	}

	// One lap message per merged part.
	for _, lap := range act.Laps {
		a.Laps = append(a.Laps, lapFromSummary(lap.Summary))
	}
	// Fall back to a single lap spanning the whole activity if none were set.
	if len(a.Laps) == 0 {
		a.Laps = append(a.Laps, lapFromSummary(s))
	}

	a.Sessions = append(a.Sessions, sessionFromSummary(act, s, len(a.Laps)))

	a.Activity = mesgdef.NewActivity(nil).
		SetType(typedef.ActivityManual).
		SetTimestamp(s.EndTime).
		SetNumSessions(1).
		SetTotalTimerTimeScaled(s.TotalMoving.Seconds()).
		SetEvent(typedef.EventActivity).
		SetEventType(typedef.EventTypeStop).
		SetLocalTimestamp(s.EndTime)

	return a
}

func recordFromModel(r model.Record) *mesgdef.Record {
	m := mesgdef.NewRecord(nil).SetTimestamp(r.Time)
	if r.Lat != nil {
		m.SetPositionLatDegrees(*r.Lat)
	}
	if r.Lon != nil {
		m.SetPositionLongDegrees(*r.Lon)
	}
	if r.Altitude != nil {
		m.SetEnhancedAltitudeScaled(*r.Altitude)
	}
	if r.Distance != nil {
		m.SetDistanceScaled(*r.Distance)
	}
	if r.Speed != nil {
		m.SetEnhancedSpeedScaled(*r.Speed)
	}
	if r.HR != nil {
		m.SetHeartRate(*r.HR)
	}
	if r.Cadence != nil {
		m.SetCadence(*r.Cadence)
	}
	if r.Power != nil {
		m.SetPower(*r.Power)
	}
	if r.Temp != nil {
		m.SetTemperature(*r.Temp)
	}
	return m
}

func sessionFromSummary(act model.Activity, s model.Summary, numLaps int) *mesgdef.Session {
	m := mesgdef.NewSession(nil).
		SetEvent(typedef.EventSession).
		SetEventType(typedef.EventTypeStop).
		SetStartTime(s.StartTime).
		SetTimestamp(s.EndTime).
		SetSport(sportToTypedef(act.Sport)).
		SetTotalElapsedTimeScaled(s.TotalElapsed.Seconds()).
		SetTotalTimerTimeScaled(s.TotalMoving.Seconds()).
		SetTotalDistanceScaled(s.TotalDistance).
		SetTotalAscent(u16(s.TotalAscent)).
		SetTotalDescent(u16(s.TotalDescent)).
		SetAvgSpeedScaled(s.AvgSpeed).
		SetMaxSpeedScaled(s.MaxSpeed).
		SetFirstLapIndex(0).
		SetNumLaps(uint16(numLaps))

	if lat, lon, ok := firstPosition(act.Records); ok {
		m.SetStartPositionLatDegrees(lat).SetStartPositionLongDegrees(lon)
	}
	if s.MaxHR > 0 {
		m.SetAvgHeartRate(s.AvgHR).SetMaxHeartRate(s.MaxHR)
	}
	return m
}

func lapFromSummary(s model.Summary) *mesgdef.Lap {
	m := mesgdef.NewLap(nil).
		SetEvent(typedef.EventLap).
		SetEventType(typedef.EventTypeStop).
		SetStartTime(s.StartTime).
		SetTimestamp(s.EndTime).
		SetTotalElapsedTimeScaled(s.TotalElapsed.Seconds()).
		SetTotalTimerTimeScaled(s.TotalMoving.Seconds()).
		SetTotalDistanceScaled(s.TotalDistance).
		SetTotalAscent(u16(s.TotalAscent)).
		SetTotalDescent(u16(s.TotalDescent)).
		SetAvgSpeedScaled(s.AvgSpeed).
		SetMaxSpeedScaled(s.MaxSpeed)
	if s.MaxHR > 0 {
		m.SetAvgHeartRate(s.AvgHR).SetMaxHeartRate(s.MaxHR)
	}
	return m
}

func firstPosition(recs []model.Record) (lat, lon float64, ok bool) {
	for _, r := range recs {
		if r.Lat != nil && r.Lon != nil {
			return *r.Lat, *r.Lon, true
		}
	}
	return 0, 0, false
}

// u16 rounds and clamps a meters value into the uint16 range used by FIT
// ascent/descent fields.
func u16(v float64) uint16 {
	v = math.Round(v)
	if v < 0 {
		return 0
	}
	if v > 65534 { // 65535 is the invalid sentinel
		return 65534
	}
	return uint16(v)
}
