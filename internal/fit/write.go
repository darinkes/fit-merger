package fit

import (
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/muktihari/fit/encoder"
	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"

	"github.com/darinkes/fit-merger/internal/model"
)

// WriteFile encodes an Activity as a FIT activity file at path.
func WriteFile(path string, act model.Activity, summary model.Summary) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := Write(f, act, summary); err != nil {
		return fmt.Errorf("encode fit %q: %w", path, err)
	}
	return nil
}

// Write encodes an Activity as a FIT activity to w. The passed summary supplies
// the recomputed session/activity aggregates; per-lap aggregates come from the
// activity's laps. Distance is taken from each record's (already re-based)
// cumulative Distance field so the record stream and the session total agree.
//
// A plain io.Writer (e.g. a bytes.Buffer, as the wasm build uses) is fine: the
// encoder pre-computes the header size and CRC in one pass rather than seeking
// back to backfill them.
func Write(w io.Writer, act model.Activity, summary model.Summary) error {
	a := buildActivity(act, summary)
	fit := a.ToFIT(nil)
	if err := encoder.New(w).Encode(&fit); err != nil {
		return fmt.Errorf("encode fit: %w", err)
	}
	return nil
}

func buildActivity(act model.Activity, s model.Summary) *filedef.Activity {
	a := filedef.NewActivity()
	a.FileId.SetType(typedef.FileActivity).SetTimeCreated(s.StartTime)
	applyDevice(&a.FileId, act.Device)

	for _, r := range act.Records {
		a.Records = append(a.Records, recordFromModel(r))
	}

	// The merged parts, as bounded by the source laps; fall back to the whole
	// activity when the merge set none (e.g. a single-file convert).
	parts := make([]model.Summary, 0, len(act.Laps))
	for _, lap := range act.Laps {
		parts = append(parts, lap.Summary)
	}
	if len(parts) == 0 {
		parts = append(parts, s)
	}

	// Bracket each part with timer start/stop_all events so the gaps between
	// merged parts read as ordinary recording pauses (e.g. a lunch stop) rather
	// than an unexplained hole in the record stream. Importers such as Strava
	// can otherwise crop or split the activity at a naked gap, which would drop
	// the merged distance below the original ride's total.
	a.Events = timerEvents(parts)

	// Always report a single lap spanning the whole activity: a merge yields one
	// continuous effort, not one lap per input file. The per-part boundaries
	// survive as the timer events above.
	a.Laps = append(a.Laps, lapFromSummary(s))

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

// applyDevice writes the source device's identity onto the file_id, preserving
// the original manufacturer/product through a merge. When no device is known
// (e.g. a GPX-only input), it stamps a neutral "development" identity so the
// file is still valid.
func applyDevice(id *mesgdef.FileId, dev *model.Device) {
	if dev == nil || dev.IsZero() {
		id.SetManufacturer(typedef.ManufacturerDevelopment).SetProduct(1)
		return
	}
	if dev.Manufacturer != 0 {
		id.SetManufacturer(typedef.Manufacturer(dev.Manufacturer))
	} else {
		id.SetManufacturer(typedef.ManufacturerDevelopment)
	}
	if dev.Product != 0 {
		id.SetProduct(dev.Product)
	}
	if dev.ProductName != "" {
		id.SetProductName(dev.ProductName)
	}
	if dev.SerialNumber != 0 {
		id.SetSerialNumber(dev.SerialNumber)
	}
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

// timerEvents builds the timer start/stop_all pairs that bracket each part of
// the activity. A device emits a timer `start` when recording begins or resumes
// and a `stop_all` when it pauses or ends, so for two merged parts the sequence
// is start, stop_all, start, stop_all — turning the inter-part gap into an
// explicit pause. The encoder interleaves these with the records by timestamp.
func timerEvents(parts []model.Summary) []*mesgdef.Event {
	events := make([]*mesgdef.Event, 0, len(parts)*2)
	for _, p := range parts {
		events = append(events,
			timerEvent(p.StartTime, typedef.EventTypeStart),
			timerEvent(p.EndTime, typedef.EventTypeStopAll),
		)
	}
	return events
}

func timerEvent(t time.Time, et typedef.EventType) *mesgdef.Event {
	return mesgdef.NewEvent(nil).
		SetTimestamp(t).
		SetEvent(typedef.EventTimer).
		SetEventType(et)
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
