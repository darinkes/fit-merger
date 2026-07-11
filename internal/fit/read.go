package fit

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/kit/semicircles"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/mesgdef"
	"github.com/muktihari/fit/profile/typedef"

	"github.com/darinkes/fit-merger/internal/model"
)

// ReadFile decodes a FIT activity file at path, recording the path as the
// activity's source.
func ReadFile(path string) (model.Activity, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Activity{}, err
	}
	defer f.Close()
	act, err := Read(f)
	if err != nil {
		return model.Activity{}, fmt.Errorf("decode fit %q: %w", path, err)
	}
	act.Sources = []string{path}
	return act, nil
}

// Read decodes a FIT activity from r into an Activity. Record messages become
// the record stream; the source file's stored session/lap summaries are ignored
// because they are recomputed from the merged points on the way out. The caller
// sets Sources.
func Read(r io.Reader) (model.Activity, error) {
	fit, err := decoder.New(r).Decode()
	if err != nil {
		return model.Activity{}, fmt.Errorf("decode fit: %w", err)
	}

	a := filedef.NewActivity(fit.Messages...)
	var act model.Activity
	if len(a.Sessions) > 0 {
		act.Sport = a.Sessions[0].Sport.String()
	} else if len(a.Sports) > 0 {
		act.Sport = a.Sports[0].Sport.String()
	}
	act.Device = deviceFromFileID(&a.FileId)

	for _, r := range a.Records {
		act.Records = append(act.Records, recordToModel(r))
	}
	for _, lap := range a.Laps {
		act.Laps = append(act.Laps, model.Lap{
			StartTime: lap.StartTime,
			EndTime:   lap.Timestamp,
		})
	}
	act.Active = activeSpans(a.Events)
	return act, nil
}

// activeSpans reconstructs the timer-on intervals from a FIT file's timer
// events. A device emits a timer `start` when recording begins or resumes and a
// `stop`/`stop_all` when it pauses or ends, so each start..stop pair is a span
// during which the athlete was moving. Events are processed in timestamp order;
// an unmatched start (file cut off mid-recording) or stop is ignored. Returns
// nil when the file has no timer events, so callers fall back to speed-based
// moving time.
func activeSpans(events []*mesgdef.Event) []model.TimeSpan {
	timers := make([]*mesgdef.Event, 0, len(events))
	for _, e := range events {
		if e.Event == typedef.EventTimer && !e.Timestamp.IsZero() {
			timers = append(timers, e)
		}
	}
	sort.SliceStable(timers, func(i, j int) bool {
		return timers[i].Timestamp.Before(timers[j].Timestamp)
	})

	var spans []model.TimeSpan
	var start time.Time
	open := false
	for _, e := range timers {
		if e.EventType == typedef.EventTypeStart {
			start, open = e.Timestamp, true
			continue
		}
		// Any stop variant (stop, stop_all, stop_disable, ...) closes the span.
		if open && e.Timestamp.After(start) {
			spans = append(spans, model.TimeSpan{Start: start, End: e.Timestamp})
		}
		open = false
	}
	return spans
}

// deviceFromFileID lifts the recording device's identity out of a FIT file_id
// message, normalizing FIT "invalid" sentinels to zero. Returns nil when the
// file carries no manufacturer or product.
func deviceFromFileID(id *mesgdef.FileId) *model.Device {
	manu := uint16(id.Manufacturer)
	if manu == basetype.Uint16Invalid {
		manu = 0
	}
	product := id.Product
	if product == basetype.Uint16Invalid {
		product = 0
	}
	serial := id.SerialNumber
	if serial == basetype.Uint32Invalid {
		serial = 0
	}
	d := model.Device{
		Manufacturer: manu,
		Product:      product,
		ProductName:  id.ProductName,
		SerialNumber: serial,
	}
	if d.IsZero() {
		return nil
	}
	return &d
}

func recordToModel(r *mesgdef.Record) model.Record {
	out := model.Record{Time: r.Timestamp}

	if r.PositionLat != basetype.Sint32Invalid && r.PositionLong != basetype.Sint32Invalid {
		lat := semicircles.ToDegrees(r.PositionLat)
		lon := semicircles.ToDegrees(r.PositionLong)
		out.Lat = &lat
		out.Lon = &lon
	}
	if alt := altitude(r); !math.IsNaN(alt) {
		out.Altitude = &alt
	}
	if d := r.DistanceScaled(); !math.IsNaN(d) {
		out.Distance = &d
	}
	if sp := speed(r); !math.IsNaN(sp) {
		out.Speed = &sp
	}
	if r.HeartRate != basetype.Uint8Invalid {
		hr := r.HeartRate
		out.HR = &hr
	}
	if r.Cadence != basetype.Uint8Invalid {
		c := r.Cadence
		out.Cadence = &c
	}
	if r.Power != basetype.Uint16Invalid {
		p := r.Power
		out.Power = &p
	}
	if r.Temperature != basetype.Sint8Invalid {
		t := r.Temperature
		out.Temp = &t
	}
	return out
}

// altitude prefers the higher-resolution enhanced_altitude field, falling back
// to the basic altitude. Returns NaN if neither is present.
func altitude(r *mesgdef.Record) float64 {
	if v := r.EnhancedAltitudeScaled(); !math.IsNaN(v) {
		return v
	}
	return r.AltitudeScaled()
}

// speed prefers enhanced_speed, falling back to the basic speed field.
func speed(r *mesgdef.Record) float64 {
	if v := r.EnhancedSpeedScaled(); !math.IsNaN(v) {
		return v
	}
	return r.SpeedScaled()
}
