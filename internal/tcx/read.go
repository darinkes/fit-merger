// Package tcx decodes and encodes Garmin Training Center XML (TCX) files to and
// from the canonical activity model. Like GPX, TCX stores per-trackpoint samples
// (and per-lap summaries we deliberately ignore); every derived total is left to
// the stats package so a merged file's figures always match its points.
//
// The codec is hand-rolled on encoding/xml — TCX is plain XML and this keeps the
// project free of another dependency. Elements are matched by local name, so
// files that declare the standard default namespace parse without special care.
package tcx

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/darinkes/fitmerge/internal/model"
)

// ReadFile parses a TCX file at path into an Activity, recording the path as the
// activity's source.
func ReadFile(path string) (model.Activity, error) {
	f, err := os.Open(path)
	if err != nil {
		return model.Activity{}, err
	}
	defer f.Close()
	act, err := Read(f)
	if err != nil {
		return model.Activity{}, fmt.Errorf("parse tcx %q: %w", path, err)
	}
	act.Sources = []string{path}
	return act, nil
}

// Read parses TCX from r into an Activity. Trackpoints across every activity and
// lap become records, in document order; heart rate, cadence, speed and power
// (from the Garmin ActivityExtension TPX) are pulled on a best-effort basis. The
// caller sets Sources.
func Read(r io.Reader) (model.Activity, error) {
	var db rdDatabase
	if err := xml.NewDecoder(r).Decode(&db); err != nil {
		return model.Activity{}, fmt.Errorf("parse tcx: %w", err)
	}

	var act model.Activity
	for _, a := range db.Activities.Activity {
		if act.Sport == "" {
			act.Sport = sportToModel(a.Sport)
		}
		for _, lap := range a.Laps {
			for _, trk := range lap.Tracks {
				for i := range trk.Points {
					rec, err := trk.Points[i].record()
					if err != nil {
						return model.Activity{}, err
					}
					act.Records = append(act.Records, rec)
				}
			}
		}
	}
	return act, nil
}

// --- read-side XML structs. Local-name tags match the standard TCX namespace
// (and the ns3 ActivityExtension) without hard-coding namespace URLs. ---

type rdDatabase struct {
	XMLName    xml.Name       `xml:"TrainingCenterDatabase"`
	Activities rdActivityList `xml:"Activities"`
}

type rdActivityList struct {
	Activity []rdActivity `xml:"Activity"`
}

type rdActivity struct {
	Sport string  `xml:"Sport,attr"`
	Laps  []rdLap `xml:"Lap"`
}

type rdLap struct {
	Tracks []rdTrack `xml:"Track"`
}

type rdTrack struct {
	Points []rdPoint `xml:"Trackpoint"`
}

type rdPoint struct {
	Time     string   `xml:"Time"`
	Lat      *float64 `xml:"Position>LatitudeDegrees"`
	Lon      *float64 `xml:"Position>LongitudeDegrees"`
	Altitude *float64 `xml:"AltitudeMeters"`
	Distance *float64 `xml:"DistanceMeters"`
	HR       *uint8   `xml:"HeartRateBpm>Value"`
	Cadence  *uint8   `xml:"Cadence"`
	Speed    *float64 `xml:"Extensions>TPX>Speed"`
	Watts    *uint16  `xml:"Extensions>TPX>Watts"`
}

func (p rdPoint) record() (model.Record, error) {
	r := model.Record{
		Lat: p.Lat, Lon: p.Lon, Altitude: p.Altitude,
		Distance: p.Distance, Speed: p.Speed, HR: p.HR,
		Cadence: p.Cadence, Power: p.Watts,
	}
	if s := strings.TrimSpace(p.Time); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return model.Record{}, fmt.Errorf("tcx trackpoint time %q: %w", s, err)
		}
		r.Time = t
	}
	return r, nil
}

// sportToModel maps a TCX Sport attribute to the model's free-form sport tag.
func sportToModel(s string) string {
	switch s {
	case "Running":
		return "running"
	case "Biking":
		return "cycling"
	default:
		return "" // "Other" or unset
	}
}
