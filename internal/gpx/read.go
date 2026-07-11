// Package gpx decodes and encodes GPX 1.1 files to and from the canonical
// activity model. GPX stores no summary figures — only the point stream — so
// reading is a straight projection of track points, and every derived total is
// left to the stats package.
package gpx

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	xgpx "github.com/tkrajina/gpxgo/gpx"

	"github.com/darinkes/fit-merger/internal/model"
)

// ReadFile parses a GPX file at path into an Activity, recording the path as
// the activity's source.
func ReadFile(path string) (model.Activity, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Activity{}, err
	}
	defer f.Close()
	act, err := Read(f)
	if err != nil {
		return model.Activity{}, fmt.Errorf("parse gpx %q: %w", path, err)
	}
	act.Sources = []string{path}
	return act, nil
}

// Read parses GPX from r into an Activity. Track points become records; heart
// rate, cadence, temperature and power are pulled from the Garmin
// TrackPointExtension (and common variants) on a best-effort basis. The caller
// sets Sources.
func Read(r io.Reader) (model.Activity, error) {
	g, err := xgpx.Parse(r)
	if err != nil {
		return model.Activity{}, fmt.Errorf("parse gpx: %w", err)
	}

	var act model.Activity
	for _, trk := range g.Tracks {
		if act.Sport == "" {
			act.Sport = trk.Type
		}
		for _, seg := range trk.Segments {
			for i := range seg.Points {
				act.Records = append(act.Records, pointToRecord(&seg.Points[i]))
			}
		}
	}
	return act, nil
}

func pointToRecord(p *xgpx.GPXPoint) model.Record {
	lat := p.Latitude
	lon := p.Longitude
	r := model.Record{Time: p.Timestamp, Lat: &lat, Lon: &lon}
	if p.Elevation.NotNull() {
		e := p.Elevation.Value()
		r.Altitude = &e
	}
	if v, ok := extFloat(p.Extensions, "hr"); ok {
		r.HR = u8(v)
	}
	if v, ok := extFloat(p.Extensions, "cad"); ok {
		r.Cadence = u8(v)
	}
	if v, ok := extFloat(p.Extensions, "atemp", "temp"); ok {
		r.Temp = i8(v)
	}
	if v, ok := extFloat(p.Extensions, "power", "PowerInWatts"); ok {
		r.Power = u16(v)
	}
	if v, ok := extFloat(p.Extensions, "speed"); ok {
		r.Speed = &v
	}
	return r
}

// extFloat searches an extension tree (any depth, any namespace) for the first
// node whose local name matches one of names and parses its text as a float.
func extFloat(ext xgpx.Extension, names ...string) (float64, bool) {
	s, ok := extText(ext.Nodes, names)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func extText(nodes []xgpx.ExtensionNode, names []string) (string, bool) {
	for i := range nodes {
		n := &nodes[i]
		for _, name := range names {
			if strings.EqualFold(n.LocalName(), name) {
				if s := strings.TrimSpace(n.Data); s != "" {
					return s, true
				}
			}
		}
		if s, ok := extText(n.Nodes, names); ok {
			return s, true
		}
	}
	return "", false
}

func u8(f float64) *uint8 {
	v := uint8(clamp(f, 0, 255))
	return &v
}

func u16(f float64) *uint16 {
	v := uint16(clamp(f, 0, 65535))
	return &v
}

func i8(f float64) *int8 {
	v := int8(clamp(f, -128, 127))
	return &v
}

func clamp(f, lo, hi float64) float64 {
	if f < lo {
		return lo
	}
	if f > hi {
		return hi
	}
	return f
}
