// Package fit decodes and encodes FIT files to and from the canonical activity
// model.
//
// Unlike GPX, FIT stores summary messages (session/lap) and a cumulative
// distance on every record, so encoding must recompute those from the merged
// stream to stay consistent — that is exactly what the stats/merge packages
// hand us via the Summary passed to Write.
package fit

import (
	"strings"

	"github.com/muktihari/fit/profile/typedef"
)

// sportToTypedef maps our free-form sport string (as carried by GPX <type> or a
// decoded FIT session) to a FIT sport enum. Unknown sports fall back to
// Generic, which is always valid.
func sportToTypedef(sport string) typedef.Sport {
	switch strings.ToLower(strings.TrimSpace(sport)) {
	case "cycling", "biking", "bike", "road_biking", "9":
		return typedef.SportCycling
	case "running", "run", "1":
		return typedef.SportRunning
	case "walking", "walk":
		return typedef.SportWalking
	case "hiking", "hike":
		return typedef.SportHiking
	default:
		return typedef.SportGeneric
	}
}
