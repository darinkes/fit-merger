package tcx

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Inspect returns ordered, human-readable key/value details specific to a TCX
// file: its sport, activity/lap/trackpoint structure and time span. TCX stores
// per-lap summaries, but fitmerge recomputes every total, so those come from the
// shared stats layer instead.
func Inspect(path string) ([][2]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var db rdDatabase
	if err := xml.NewDecoder(f).Decode(&db); err != nil {
		return nil, fmt.Errorf("parse tcx %q: %w", path, err)
	}

	sport := ""
	laps, pts := 0, 0
	var first, last time.Time
	for _, a := range db.Activities.Activity {
		if sport == "" {
			sport = a.Sport
		}
		for _, lap := range a.Laps {
			laps++
			for _, trk := range lap.Tracks {
				for _, p := range trk.Points {
					pts++
					if t, err := time.Parse(time.RFC3339, p.Time); err == nil {
						if first.IsZero() || t.Before(first) {
							first = t
						}
						if t.After(last) {
							last = t
						}
					}
				}
			}
		}
	}

	kv := [][2]string{{"Format", "TCX (Training Center XML)"}}
	if sport != "" {
		kv = append(kv, [2]string{"Sport", sport})
	}
	kv = append(kv,
		[2]string{"Activities", strconv.Itoa(len(db.Activities.Activity))},
		[2]string{"Laps", strconv.Itoa(laps)},
		[2]string{"Trackpoints", strconv.Itoa(pts)},
	)
	if !first.IsZero() {
		kv = append(kv,
			[2]string{"First point", first.UTC().Format(time.RFC3339)},
			[2]string{"Last point", last.UTC().Format(time.RFC3339)},
		)
	}
	return kv, nil
}
