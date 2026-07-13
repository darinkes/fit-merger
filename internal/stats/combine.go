package stats

import "github.com/darinkes/fitmerge/internal/model"

// Combine folds the per-part summaries of a concatenated activity into one.
//
// This is preferred over recomputing a single summary from the merged record
// stream because it keeps file boundaries meaningful: distance, ascent and
// moving time are additive across parts, but the geographic gap *between* two
// consecutively recorded files (you drove home between rides) must never be
// counted as distance or speed. Computing each part independently and summing
// achieves exactly that.
//
// Parts must be in chronological (merged) order.
func Combine(parts []model.Summary) model.Summary {
	var out model.Summary
	var hrWeighted, hrRecords int
	for _, p := range parts {
		if p.Records == 0 {
			continue
		}
		out.TotalDistance += p.TotalDistance
		out.TotalAscent += p.TotalAscent
		out.TotalDescent += p.TotalDescent
		out.TotalMoving += p.TotalMoving
		out.Records += p.Records

		if p.MaxSpeed > out.MaxSpeed {
			out.MaxSpeed = p.MaxSpeed
		}
		if p.MaxHR > out.MaxHR {
			out.MaxHR = p.MaxHR
		}
		if p.AvgHR > 0 {
			hrWeighted += int(p.AvgHR) * p.Records
			hrRecords += p.Records
		}

		if out.StartTime.IsZero() || p.StartTime.Before(out.StartTime) {
			out.StartTime = p.StartTime
		}
		if p.EndTime.After(out.EndTime) {
			out.EndTime = p.EndTime
		}
	}

	// Elapsed spans the whole merged activity, gaps between parts included.
	if out.EndTime.After(out.StartTime) {
		out.TotalElapsed = out.EndTime.Sub(out.StartTime)
	}
	if out.TotalMoving > 0 {
		out.AvgSpeed = out.TotalDistance / out.TotalMoving.Seconds()
	}
	if hrRecords > 0 {
		out.AvgHR = uint8(hrWeighted / hrRecords)
	}
	return out
}
