// Package merge concatenates several decoded activities into one, keeping the
// time line monotonic and the cumulative distance continuous across file
// boundaries.
package merge

import (
	"fmt"
	"sort"
	"time"

	"github.com/darinkes/fitmerge/internal/model"
	"github.com/darinkes/fitmerge/internal/stats"
)

// OverlapStrategy decides what to do when a later file starts before the
// previous one ended.
type OverlapStrategy string

const (
	// OverlapError aborts the merge (the safe default: overlapping inputs are
	// almost always a mistake or need an explicit decision).
	OverlapError OverlapStrategy = "error"
	// OverlapKeep concatenates anyway, leaving the overlapping samples in place.
	OverlapKeep OverlapStrategy = "keep"
	// OverlapTrim drops the leading samples of the later file that fall at or
	// before the previous file's end.
	OverlapTrim OverlapStrategy = "trim"
)

// Options controls ordering, overlap handling, and the stats definitions used
// while re-basing distance and summarizing each part.
type Options struct {
	Sort    bool // order inputs by their first timestamp
	Overlap OverlapStrategy
	Stats   stats.Options
}

// Result is the outcome of a merge: the combined activity plus the summary of
// the whole and of each part, in final order, for reporting.
type Result struct {
	Activity model.Activity
	Summary  model.Summary
	Parts    []model.Summary
}

// Merge combines activities per opts. Empty activities are dropped. The
// returned activity has a single continuous, monotonic cumulative-distance
// series with inter-file gaps contributing zero distance (a gap is a pause,
// not a leap).
func Merge(acts []model.Activity, opts Options) (Result, error) {
	// Drop activities with no usable records.
	ordered := make([]model.Activity, 0, len(acts))
	for _, a := range acts {
		if len(a.Records) > 0 {
			ordered = append(ordered, a)
		}
	}
	if len(ordered) == 0 {
		return Result{}, fmt.Errorf("no records to merge")
	}

	// Ordering and overlap detection are time-based, so multi-file merges need
	// timestamps. A single input is just a copy/convert and may lack them.
	if len(ordered) > 1 {
		for _, a := range ordered {
			if a.Records[0].Time.IsZero() {
				return Result{}, fmt.Errorf(
					"cannot merge %s: it has no timestamps, so inputs can't be ordered in time",
					sourceName(a))
			}
		}
	}

	if opts.Sort {
		sort.SliceStable(ordered, func(i, j int) bool {
			return ordered[i].Records[0].Time.Before(ordered[j].Records[0].Time)
		})
	}

	var (
		out         model.Activity
		parts       []model.Summary
		runningDist float64
		prevEnd     = ordered[0].Records[0].Time // sentinel; reset below
		havePrevEnd bool
	)
	out.Sport = ordered[0].Sport
	// Preserve the original recording device: keep the first input that has one,
	// so a merged FIT reports the real hardware rather than a generic identity.
	for _, a := range ordered {
		if a.Device != nil {
			out.Device = a.Device
			break
		}
	}

	for _, a := range ordered {
		// Work on a time-sorted copy so a slightly disordered input can't break
		// the monotonic-time assumption the stats/distance code relies on.
		recs := sortedByTime(a.Records)

		if havePrevEnd {
			first := recs[0].Time
			if !first.After(prevEnd) {
				switch opts.Overlap {
				case OverlapError, "":
					return Result{}, fmt.Errorf(
						"time overlap: %s starts at %s, not after previous end %s (use --overlap=trim|keep)",
						sourceName(a), first.Format("2006-01-02T15:04:05Z"),
						prevEnd.Format("2006-01-02T15:04:05Z"))
				case OverlapTrim:
					recs = trimLeading(recs, prevEnd)
					if len(recs) == 0 {
						continue // fully contained in previous part
					}
				case OverlapKeep:
					// leave as-is
				default:
					return Result{}, fmt.Errorf("unknown overlap strategy %q", opts.Overlap)
				}
			}
		}

		// Re-base distance: within-part cumulative distance, offset by the total
		// distance merged so far. The boundary segment is never measured, so the
		// gap between two files adds no distance.
		cd := stats.CumulativeDistances(recs, opts.Stats.Use3D)
		rebased := make([]model.Record, len(recs))
		for i, r := range recs {
			d := runningDist + cd[i]
			r.Distance = &d
			rebased[i] = r
		}
		out.Records = append(out.Records, rebased...)

		// Summarize this part from its own records (see stats.Combine for why).
		// Re-basing only offsets the absolute distance field by a constant, which
		// cancels in the per-segment deltas, so recs and rebased summarize alike.
		part := stats.Compute(recs, opts.Stats)
		// Honor the device's own timer events for moving time when the input
		// carried them: the speed threshold only estimates the pauses the device
		// already recorded exactly.
		if len(a.Active) > 0 {
			part.TotalMoving = stats.MovingTimeFromSpans(recs, a.Active)
			part.AvgSpeed = 0
			if part.TotalMoving > 0 {
				part.AvgSpeed = part.TotalDistance / part.TotalMoving.Seconds()
			}
		}

		out.Laps = append(out.Laps, model.Lap{
			StartTime: recs[0].Time,
			EndTime:   recs[len(recs)-1].Time,
			Summary:   part,
		})
		if out.Sport == "" {
			out.Sport = a.Sport
		}
		out.Sources = append(out.Sources, a.Sources...)
		parts = append(parts, part)

		if len(cd) > 0 {
			runningDist += cd[len(cd)-1]
		}
		prevEnd = recs[len(recs)-1].Time
		havePrevEnd = true
	}

	return Result{
		Activity: out,
		Summary:  stats.Combine(parts),
		Parts:    parts,
	}, nil
}

// sortedByTime returns a stably time-sorted copy of recs, leaving the input
// untouched.
func sortedByTime(recs []model.Record) []model.Record {
	out := append([]model.Record(nil), recs...)
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Time.Before(out[j].Time)
	})
	return out
}

// trimLeading drops records whose time is at or before cutoff.
func trimLeading(recs []model.Record, cutoff time.Time) []model.Record {
	i := 0
	for i < len(recs) && !recs[i].Time.After(cutoff) {
		i++
	}
	return recs[i:]
}

func sourceName(a model.Activity) string {
	if len(a.Sources) > 0 {
		return a.Sources[0]
	}
	return "input"
}
