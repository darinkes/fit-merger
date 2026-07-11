package gpx

import (
	"fmt"
	"strconv"
	"time"

	xgpx "github.com/tkrajina/gpxgo/gpx"
)

// Inspect returns ordered, human-readable key/value details specific to a GPX
// file: its version, creator, metadata, and track structure. GPX stores no
// summary figures, so those come from the shared stats layer instead.
func Inspect(path string) ([][2]string, error) {
	g, err := xgpx.ParseFile(path)
	if err != nil {
		return nil, fmt.Errorf("parse gpx %q: %w", path, err)
	}

	segs, pts := 0, 0
	for _, t := range g.Tracks {
		segs += len(t.Segments)
		for _, s := range t.Segments {
			pts += len(s.Points)
		}
	}

	kv := [][2]string{
		{"GPX version", g.Version},
		{"Creator", g.Creator},
	}
	if g.Name != "" {
		kv = append(kv, [2]string{"Name", g.Name})
	}
	if g.Description != "" {
		kv = append(kv, [2]string{"Description", g.Description})
	}
	if g.AuthorName != "" {
		kv = append(kv, [2]string{"Author", g.AuthorName})
	}
	if g.Time != nil {
		kv = append(kv, [2]string{"Time", g.Time.UTC().Format(time.RFC3339)})
	}
	kv = append(kv,
		[2]string{"Tracks", strconv.Itoa(len(g.Tracks))},
		[2]string{"Segments", strconv.Itoa(segs)},
		[2]string{"Track points", strconv.Itoa(pts)},
		[2]string{"Waypoints", strconv.Itoa(len(g.Waypoints))},
		[2]string{"Routes", strconv.Itoa(len(g.Routes))},
	)
	return kv, nil
}
