package gpx

import (
	"fmt"
	"io"
	"os"
	"strconv"

	xgpx "github.com/tkrajina/gpxgo/gpx"

	"github.com/darinkes/fitmerge/internal/model"
)

// tpxNS is the Garmin TrackPointExtension namespace used for hr/cad/atemp.
const tpxNS = "http://www.garmin.com/xmlschemas/TrackPointExtension/v1"

// Creator is written to the GPX <gpx creator="..."> attribute.
const Creator = "fitmerge (https://github.com/darinkes/fitmerge)"

// WriteFile encodes an Activity as GPX 1.1 to path.
func WriteFile(path string, act model.Activity) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := Write(f, act); err != nil {
		return fmt.Errorf("write gpx %q: %w", path, err)
	}
	return nil
}

// Write encodes an Activity as GPX 1.1 to w. Each lap becomes its own track
// segment, which represents the pause between merged files; if the activity has
// no laps, all records go into a single segment.
func Write(w io.Writer, act model.Activity) error {
	g := build(act)
	data, err := xgpx.ToXml(g, xgpx.ToXmlParams{Version: "1.1", Indent: true})
	if err != nil {
		return fmt.Errorf("encode gpx: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}

func build(act model.Activity) *xgpx.GPX {
	g := &xgpx.GPX{Version: "1.1", Creator: Creator}
	g.RegisterNamespace("gpxtpx", tpxNS)

	trk := xgpx.GPXTrack{Type: act.Sport}
	for _, seg := range segments(act) {
		s := xgpx.GPXTrackSegment{}
		for _, r := range seg {
			s.Points = append(s.Points, recordToPoint(r))
		}
		trk.Segments = append(trk.Segments, s)
	}
	g.Tracks = append(g.Tracks, trk)
	return g
}

// segments splits the record stream by lap boundaries. Records are already in
// time order, and laps are non-overlapping (overlaps are resolved during
// merge), so a simple per-lap time-window partition is unambiguous.
func segments(act model.Activity) [][]model.Record {
	if len(act.Laps) == 0 {
		return [][]model.Record{act.Records}
	}
	var out [][]model.Record
	i := 0
	for _, lap := range act.Laps {
		var seg []model.Record
		for i < len(act.Records) && !act.Records[i].Time.After(lap.EndTime) {
			seg = append(seg, act.Records[i])
			i++
		}
		if len(seg) > 0 {
			out = append(out, seg)
		}
	}
	// Any trailing records not covered by a lap window go into a final segment.
	if i < len(act.Records) {
		out = append(out, act.Records[i:])
	}
	return out
}

func recordToPoint(r model.Record) xgpx.GPXPoint {
	p := xgpx.GPXPoint{Timestamp: r.Time}
	if r.Lat != nil {
		p.Latitude = *r.Lat
	}
	if r.Lon != nil {
		p.Longitude = *r.Lon
	}
	if r.Altitude != nil {
		p.Point.Elevation = *xgpx.NewNullableFloat64(*r.Altitude)
	}

	// Heart rate, cadence and temperature live in the Garmin
	// TrackPointExtension; power is not part of it, so Strava/others use a plain
	// node instead.
	if r.HR != nil {
		p.Extensions.GetOrCreateNode(xgpx.NamespaceURL(tpxNS), "TrackPointExtension", "hr").Data = strconv.Itoa(int(*r.HR))
	}
	if r.Cadence != nil {
		p.Extensions.GetOrCreateNode(xgpx.NamespaceURL(tpxNS), "TrackPointExtension", "cad").Data = strconv.Itoa(int(*r.Cadence))
	}
	if r.Temp != nil {
		p.Extensions.GetOrCreateNode(xgpx.NamespaceURL(tpxNS), "TrackPointExtension", "atemp").Data = strconv.Itoa(int(*r.Temp))
	}
	if r.Power != nil {
		p.Extensions.GetOrCreateNode(xgpx.NoNamespace, "power").Data = strconv.Itoa(int(*r.Power))
	}
	return p
}
