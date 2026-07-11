// Package stats derives activity summaries (distance, ascent, moving time,
// speed, heart rate) from a record stream.
//
// Every figure is recomputed from the points rather than copied from a source
// file's stored summary. This is deliberate: after merging two files the only
// way to guarantee the reported totals match the actual track is to compute
// them from the merged points with one consistent algorithm.
package stats

import (
	"time"

	"github.com/darinkes/fit-merger/internal/geo"
	"github.com/darinkes/fit-merger/internal/model"
)

// Options tunes the two figures that are genuinely a matter of definition
// rather than measurement: how much vertical noise to ignore before counting
// climb, and how slow counts as "stopped".
type Options struct {
	// AscentThreshold is the minimum sustained elevation change, in meters,
	// counted as real climb/descent. It suppresses GPS altitude jitter, which
	// would otherwise inflate total ascent. This is the main reason Garmin,
	// Strava and friends disagree on climb; we make it explicit.
	AscentThreshold float64
	// MovingSpeedThreshold is the speed, in m/s, below which time is treated
	// as a pause and excluded from moving time.
	MovingSpeedThreshold float64
	// Use3D includes the vertical component when measuring distance between
	// samples. Off by default; horizontal distance matches most tools.
	Use3D bool
}

// DefaultOptions are sensible starting values: ignore sub-3m elevation wiggle,
// treat below 0.5 m/s (~1.8 km/h) as stopped.
func DefaultOptions() Options {
	return Options{AscentThreshold: 3.0, MovingSpeedThreshold: 0.5}
}

// Compute derives a Summary from records, which must be in ascending time
// order. Segments whose time delta is non-positive are ignored for time-based
// figures but still contribute geometric distance.
func Compute(records []model.Record, opt Options) model.Summary {
	var s model.Summary
	s.Records = len(records)
	if len(records) == 0 {
		return s
	}

	s.StartTime = records[0].Time
	s.EndTime = records[len(records)-1].Time
	if s.EndTime.After(s.StartTime) {
		s.TotalElapsed = s.EndTime.Sub(s.StartTime)
	}

	var (
		hrSum   int
		hrCount int
	)

	prev := records[0]
	if prev.HR != nil {
		hrSum += int(*prev.HR)
		hrCount++
		s.MaxHR = *prev.HR
	}
	if prev.Speed != nil && *prev.Speed > s.MaxSpeed {
		s.MaxSpeed = *prev.Speed
	}

	for i := 1; i < len(records); i++ {
		cur := records[i]

		// Distance between this sample and the last.
		segDist := segmentDistance(prev, cur, opt.Use3D)
		s.TotalDistance += segDist

		// Time-based figures require a forward-moving clock.
		dt := cur.Time.Sub(prev.Time).Seconds()
		if dt > 0 {
			speed := segSpeed(cur, segDist, dt)
			if speed > s.MaxSpeed {
				s.MaxSpeed = speed
			}
			if speed >= opt.MovingSpeedThreshold {
				s.TotalMoving += cur.Time.Sub(prev.Time)
			}
		}

		if cur.Speed != nil && *cur.Speed > s.MaxSpeed {
			s.MaxSpeed = *cur.Speed
		}
		if cur.HR != nil {
			hrSum += int(*cur.HR)
			hrCount++
			if *cur.HR > s.MaxHR {
				s.MaxHR = *cur.HR
			}
		}

		prev = cur
	}

	s.TotalAscent, s.TotalDescent = ascentDescent(records, opt.AscentThreshold)

	if s.TotalMoving > 0 {
		s.AvgSpeed = s.TotalDistance / s.TotalMoving.Seconds()
	}
	if hrCount > 0 {
		s.AvgHR = uint8(hrSum / hrCount)
	}
	return s
}

// MovingTimeFromSpans sums the timer-on spans, clamped to the record time
// range, giving the moving time the recording device itself measured. It is
// preferred over the speed-threshold estimate when a FIT input carries timer
// events, since it honors the device's own pause detection exactly. Returns 0
// when there are no spans or records.
func MovingTimeFromSpans(records []model.Record, spans []model.TimeSpan) time.Duration {
	if len(records) == 0 || len(spans) == 0 {
		return 0
	}
	first := records[0].Time
	last := records[len(records)-1].Time
	var total time.Duration
	for _, s := range spans {
		start, end := s.Start, s.End
		if start.Before(first) {
			start = first
		}
		if end.After(last) {
			end = last
		}
		if end.After(start) {
			total += end.Sub(start)
		}
	}
	return total
}

// CumulativeDistances returns, for each record, the running distance in meters
// from the first record. FIT encoding needs this on every record, and it keeps
// the model's Distance field consistent with the recomputed total.
func CumulativeDistances(records []model.Record, use3D bool) []float64 {
	out := make([]float64, len(records))
	for i := 1; i < len(records); i++ {
		out[i] = out[i-1] + segmentDistance(records[i-1], records[i], use3D)
	}
	return out
}

// segmentDistance measures the distance between two samples, preferring GPS
// positions and falling back to the difference of stored cumulative distances
// (as FIT records carry) when positions are unavailable.
func segmentDistance(a, b model.Record, use3D bool) float64 {
	if a.HasPosition() && b.HasPosition() {
		if use3D && a.Altitude != nil && b.Altitude != nil {
			return geo.Distance3D(*a.Lat, *a.Lon, *a.Altitude, *b.Lat, *b.Lon, *b.Altitude)
		}
		return geo.Haversine(*a.Lat, *a.Lon, *b.Lat, *b.Lon)
	}
	if a.Distance != nil && b.Distance != nil {
		if d := *b.Distance - *a.Distance; d > 0 {
			return d
		}
	}
	return 0
}

// segSpeed determines a segment's speed, preferring the device-reported speed
// on the ending sample and falling back to distance over time.
func segSpeed(b model.Record, segDist, dt float64) float64 {
	if b.Speed != nil {
		return *b.Speed
	}
	return segDist / dt
}

// ascentDescent accumulates climb and descent using a hysteresis reference:
// elevation must move at least `threshold` meters from the current reference
// before it counts, after which the reference advances. This ignores jitter
// while still capturing sustained climbs assembled from small steps.
func ascentDescent(records []model.Record, threshold float64) (asc, desc float64) {
	ref, haveRef := 0.0, false
	for _, r := range records {
		if r.Altitude == nil {
			continue
		}
		e := *r.Altitude
		if !haveRef {
			ref, haveRef = e, true
			continue
		}
		switch d := e - ref; {
		case d >= threshold:
			asc += d
			ref = e
		case -d >= threshold:
			desc += -d
			ref = e
		}
	}
	return asc, desc
}
