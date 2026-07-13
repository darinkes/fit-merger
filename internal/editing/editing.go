// Package editing trims and privacy-filters a record stream before it is
// (re-)summarized and encoded. It only ever removes whole records from the ends
// of the stream; re-basing distance and recomputing every total is left to the
// merge/stats layer, so the output stays internally consistent.
package editing

import (
	"github.com/darinkes/fitmerge/internal/geo"
	"github.com/darinkes/fitmerge/internal/model"
	"github.com/darinkes/fitmerge/internal/stats"
)

// Trim removes the first startM and last endM metres of the activity, measured
// by 2D distance along the track. Non-positive amounts are no-ops. If the two
// cuts meet or cross, everything is removed.
func Trim(recs []model.Record, startM, endM float64) []model.Record {
	if (startM <= 0 && endM <= 0) || len(recs) == 0 {
		return recs
	}
	cd := stats.CumulativeDistances(recs, false)
	if len(cd) == 0 {
		return recs
	}
	hi := cd[len(cd)-1] - endM
	out := make([]model.Record, 0, len(recs))
	for i, r := range recs {
		if cd[i] >= startM && cd[i] <= hi {
			out = append(out, r)
		}
	}
	return out
}

// PrivacyZone removes leading records within radiusM of the first positioned
// point (when start is set) and trailing records within radiusM of the last
// positioned point (when end is set) — the classic "hide where I live" cut.
func PrivacyZone(recs []model.Record, radiusM float64, start, end bool) []model.Record {
	if radiusM <= 0 || (!start && !end) || len(recs) == 0 {
		return recs
	}
	lo, hi := 0, len(recs)
	if start {
		if p := firstPositioned(recs); p != nil {
			for lo < hi && near(recs[lo], p, radiusM) {
				lo++
			}
		}
	}
	if end {
		if p := lastPositioned(recs); p != nil {
			for hi > lo && near(recs[hi-1], p, radiusM) {
				hi--
			}
		}
	}
	return recs[lo:hi]
}

// near reports whether r lies within radiusM of anchor. Records without a
// position are treated as inside the zone (dropped along with it) so a privacy
// cut doesn't leave stray position-less samples at the very ends.
func near(r model.Record, anchor *model.Record, radiusM float64) bool {
	if !r.HasPosition() {
		return true
	}
	return geo.Haversine(*r.Lat, *r.Lon, *anchor.Lat, *anchor.Lon) <= radiusM
}

func firstPositioned(recs []model.Record) *model.Record {
	for i := range recs {
		if recs[i].HasPosition() {
			return &recs[i]
		}
	}
	return nil
}

func lastPositioned(recs []model.Record) *model.Record {
	for i := len(recs) - 1; i >= 0; i-- {
		if recs[i].HasPosition() {
			return &recs[i]
		}
	}
	return nil
}
