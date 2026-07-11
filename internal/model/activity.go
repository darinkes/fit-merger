// Package model defines the canonical, format-agnostic representation of an
// activity. Both GPX and FIT files are decoded into this model, merged here,
// and then encoded back out. Keeping a single intermediate representation is
// what makes cross-format merging (e.g. FIT + GPX -> FIT) fall out for free.
package model

import "time"

// Record is a single sample in an activity's time series.
//
// Optional fields are pointers so that "field absent" is distinguishable from a
// genuine zero value: one source file may record power while another does not,
// and 0 W must not be confused with "no power data".
type Record struct {
	Time     time.Time
	Lat      *float64 // degrees, WGS84
	Lon      *float64 // degrees, WGS84
	Altitude *float64 // meters
	Distance *float64 // cumulative meters from the start of the activity
	Speed    *float64 // instantaneous speed, m/s
	HR       *uint8   // heart rate, bpm
	Cadence  *uint8   // rpm / spm
	Power    *uint16  // watts
	Temp     *int8    // degrees Celsius
}

// HasPosition reports whether the record carries a usable lat/lon pair.
func (r Record) HasPosition() bool {
	return r.Lat != nil && r.Lon != nil
}

// Lap marks a contiguous span of the activity. In GPX this maps to a track
// segment; in FIT to a lap message. Summaries are recomputed on merge, never
// trusted from the source file.
type Lap struct {
	StartTime time.Time
	EndTime   time.Time
	Summary   Summary
}

// Summary holds the aggregate figures a merged file must report correctly:
// distance, climb, elapsed vs. moving time, and speed/heart-rate extremes.
// Every field is derived from the record stream by the stats package so the
// numbers are always internally consistent with the points they describe.
type Summary struct {
	StartTime     time.Time
	EndTime       time.Time
	TotalDistance float64       // meters
	TotalAscent   float64       // meters climbed
	TotalDescent  float64       // meters descended
	TotalElapsed  time.Duration // wall-clock: last sample - first sample
	TotalMoving   time.Duration // time spent above the moving-speed threshold
	AvgSpeed      float64       // m/s, over moving time
	MaxSpeed      float64       // m/s
	AvgHR         uint8         // bpm
	MaxHR         uint8         // bpm
	Records       int
}

// Device identifies the recording device, as carried by a FIT file_id message.
// It is preserved through a merge so the output keeps the original hardware's
// identity rather than being stamped as generic. GPX has no equivalent, so it
// is nil for GPX-only inputs. Zero fields mean "unset".
type Device struct {
	Manufacturer uint16 // FIT manufacturer id (e.g. 1 = Garmin)
	Product      uint16
	ProductName  string
	SerialNumber uint32
}

// IsZero reports whether the device carries no identifying information, in which
// case it is indistinguishable from no device at all.
func (d Device) IsZero() bool {
	return d.Manufacturer == 0 && d.Product == 0 && d.ProductName == "" && d.SerialNumber == 0
}

// TimeSpan is a half-open interval [Start, End) during which the recording
// timer was running — i.e. the device considered the athlete active. FIT files
// carry these as timer start/stop events; a pause splits one span into two.
// GPX has no equivalent.
type TimeSpan struct {
	Start time.Time
	End   time.Time
}

// Activity is a fully decoded workout: an ordered stream of records plus the
// laps and provenance that produced it.
type Activity struct {
	Sport   string
	Records []Record
	Laps    []Lap
	Device  *Device    // recording device, if known (from FIT)
	Active  []TimeSpan // timer-on spans from FIT timer events; nil if unknown
	Sources []string   // input file paths, in merge order
}

// TimeBounds returns the timestamps of the first and last records. The zero
// Time is returned for an empty activity.
func (a Activity) TimeBounds() (first, last time.Time) {
	if len(a.Records) == 0 {
		return time.Time{}, time.Time{}
	}
	return a.Records[0].Time, a.Records[len(a.Records)-1].Time
}
