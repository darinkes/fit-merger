package tcx

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/darinkes/fitmerge/internal/model"
)

// TCX namespaces: the default TrainingCenterDatabase schema plus the Garmin
// ActivityExtension (ns3) used for per-trackpoint speed and power.
const (
	nsTCX     = "http://www.garmin.com/xmlschemas/TrainingCenterDatabase/v2"
	nsExt     = "http://www.garmin.com/xmlschemas/ActivityExtension/v2"
	nsXSI     = "http://www.w3.org/2001/XMLSchema-instance"
	schemaLoc = nsTCX + " http://www.garmin.com/xmlschemas/TrainingCenterDatabasev2.xsd"
)

// Creator identifies this tool in the TCX <Author> block.
const Creator = "fitmerge (https://github.com/darinkes/fitmerge)"

// WriteFile encodes an Activity as TCX to path.
func WriteFile(path string, act model.Activity, summary model.Summary) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := Write(f, act, summary); err != nil {
		return fmt.Errorf("write tcx %q: %w", path, err)
	}
	return nil
}

// Write encodes an Activity as TCX to w. Each lap becomes a <Lap> carrying its
// recomputed summary; with no laps, all records go into a single lap described
// by the activity-wide summary. Distance is written per trackpoint as the merged
// cumulative distance, so the stored lap total agrees with the point stream.
func Write(w io.Writer, act model.Activity, summary model.Summary) error {
	db := build(act, summary)
	if _, err := io.WriteString(w, xml.Header); err != nil {
		return err
	}
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(db); err != nil {
		return fmt.Errorf("encode tcx: %w", err)
	}
	_, err := io.WriteString(w, "\n")
	return err
}

func build(act model.Activity, summary model.Summary) wrDatabase {
	id := summary.StartTime
	if id.IsZero() && len(act.Records) > 0 {
		id = act.Records[0].Time
	}

	a := wrActivity{Sport: sportFromModel(act.Sport), ID: ts(id)}
	for _, seg := range laps(act, summary) {
		a.Laps = append(a.Laps, buildLap(seg.records, seg.summary))
	}

	return wrDatabase{
		Xmlns:      nsTCX,
		Ns3:        nsExt,
		Xsi:        nsXSI,
		SchemaLoc:  schemaLoc,
		Activities: wrActivityList{Activity: []wrActivity{a}},
		Author: &wrAuthor{
			XsiType: "Application_t", Name: Creator, LangID: "en", PartNumber: "000-00000-00",
			Build: wrBuild{Version: wrVersion{}},
		},
	}
}

// lapSeg pairs a lap's records with the summary to stamp on it.
type lapSeg struct {
	records []model.Record
	summary model.Summary
}

// laps splits the record stream into per-lap segments. Records are in time order
// and laps are non-overlapping (resolved during merge); with no laps the whole
// stream is one segment described by the activity-wide summary.
func laps(act model.Activity, summary model.Summary) []lapSeg {
	if len(act.Laps) == 0 {
		return []lapSeg{{records: act.Records, summary: summary}}
	}
	var out []lapSeg
	i := 0
	for _, lap := range act.Laps {
		var seg []model.Record
		for i < len(act.Records) && !act.Records[i].Time.After(lap.EndTime) {
			seg = append(seg, act.Records[i])
			i++
		}
		if len(seg) > 0 {
			out = append(out, lapSeg{records: seg, summary: lap.Summary})
		}
	}
	if i < len(act.Records) { // records past the last lap window
		out = append(out, lapSeg{records: act.Records[i:], summary: summary})
	}
	return out
}

func buildLap(recs []model.Record, s model.Summary) wrLap {
	start := s.StartTime
	if start.IsZero() && len(recs) > 0 {
		start = recs[0].Time
	}
	lap := wrLap{
		StartTime:     ts(start),
		TotalSeconds:  s.TotalElapsed.Seconds(),
		DistanceMeter: f2(s.TotalDistance),
		Calories:      0,
		Intensity:     "Active",
		Trigger:       "Manual",
	}
	if s.MaxSpeed > 0 {
		v := f3(s.MaxSpeed)
		lap.MaxSpeed = &v
	}
	if s.AvgHR > 0 {
		lap.AvgHR = &wrHR{Value: s.AvgHR}
	}
	if s.MaxHR > 0 {
		lap.MaxHR = &wrHR{Value: s.MaxHR}
	}
	for _, r := range recs {
		lap.Track.Points = append(lap.Track.Points, point(r))
	}
	return lap
}

func point(r model.Record) wrPoint {
	// Altitude stays exact (it feeds ascent on re-read); distance and speed are
	// derived and rounded for a tidy, compact file.
	p := wrPoint{Time: ts(r.Time), Altitude: r.Altitude, Cadence: r.Cadence}
	if r.HasPosition() {
		p.Position = &wrPosition{Lat: *r.Lat, Lon: *r.Lon}
	}
	if r.Distance != nil {
		d := f2(*r.Distance)
		p.Distance = &d
	}
	if r.HR != nil {
		p.HR = &wrHR{Value: *r.HR}
	}
	if r.Speed != nil || r.Power != nil {
		ext := &wrExt{TPX: wrTPX{Watts: r.Power}}
		if r.Speed != nil {
			s := f3(*r.Speed)
			ext.TPX.Speed = &s
		}
		p.Ext = ext
	}
	return p
}

// ts formats a timestamp as UTC RFC3339, TCX's expected form.
func ts(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func f2(v float64) string { return strconv.FormatFloat(v, 'f', 2, 64) }
func f3(v float64) string { return strconv.FormatFloat(v, 'f', 3, 64) }

// sportFromModel maps the model's free-form sport tag to a TCX Sport attribute.
func sportFromModel(s string) string {
	switch strings.ToLower(s) {
	case "cycling", "biking", "bike", "ride", "virtualride", "ebikeride":
		return "Biking"
	case "running", "run", "trailrun", "virtualrun":
		return "Running"
	default:
		return "Other"
	}
}

// --- write-side XML structs (field order defines element/attribute order, which
// TCX's schema is sensitive to). ns3:/xsi: prefixes are emitted literally. ---

type wrDatabase struct {
	XMLName    xml.Name       `xml:"TrainingCenterDatabase"`
	Xmlns      string         `xml:"xmlns,attr"`
	Ns3        string         `xml:"xmlns:ns3,attr"`
	Xsi        string         `xml:"xmlns:xsi,attr"`
	SchemaLoc  string         `xml:"xsi:schemaLocation,attr"`
	Activities wrActivityList `xml:"Activities"`
	Author     *wrAuthor      `xml:"Author"`
}

type wrActivityList struct {
	Activity []wrActivity `xml:"Activity"`
}

type wrActivity struct {
	Sport string  `xml:"Sport,attr"`
	ID    string  `xml:"Id"`
	Laps  []wrLap `xml:"Lap"`
}

type wrLap struct {
	StartTime     string  `xml:"StartTime,attr"`
	TotalSeconds  float64 `xml:"TotalTimeSeconds"`
	DistanceMeter string  `xml:"DistanceMeters"`
	MaxSpeed      *string `xml:"MaximumSpeed,omitempty"`
	Calories      int     `xml:"Calories"`
	AvgHR         *wrHR   `xml:"AverageHeartRateBpm"`
	MaxHR         *wrHR   `xml:"MaximumHeartRateBpm"`
	Intensity     string  `xml:"Intensity"`
	Trigger       string  `xml:"TriggerMethod"`
	Track         wrTrack `xml:"Track"`
}

type wrTrack struct {
	Points []wrPoint `xml:"Trackpoint"`
}

type wrPoint struct {
	Time     string      `xml:"Time"`
	Position *wrPosition `xml:"Position"`
	Altitude *float64    `xml:"AltitudeMeters,omitempty"`
	Distance *string     `xml:"DistanceMeters,omitempty"`
	HR       *wrHR       `xml:"HeartRateBpm"`
	Cadence  *uint8      `xml:"Cadence,omitempty"`
	Ext      *wrExt      `xml:"Extensions"`
}

type wrPosition struct {
	Lat float64 `xml:"LatitudeDegrees"`
	Lon float64 `xml:"LongitudeDegrees"`
}

type wrHR struct {
	Value uint8 `xml:"Value"`
}

type wrExt struct {
	TPX wrTPX `xml:"ns3:TPX"`
}

type wrTPX struct {
	Speed *string `xml:"ns3:Speed,omitempty"`
	Watts *uint16 `xml:"ns3:Watts,omitempty"`
}

type wrAuthor struct {
	XsiType    string  `xml:"xsi:type,attr"`
	Name       string  `xml:"Name"`
	Build      wrBuild `xml:"Build"`
	LangID     string  `xml:"LangID"`
	PartNumber string  `xml:"PartNumber"`
}

type wrBuild struct {
	Version wrVersion `xml:"Version"`
}

type wrVersion struct {
	Major int `xml:"VersionMajor"`
	Minor int `xml:"VersionMinor"`
}
