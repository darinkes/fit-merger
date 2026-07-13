package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/darinkes/fit-merger/internal/format"
	"github.com/darinkes/fit-merger/internal/model"
	"github.com/darinkes/fit-merger/internal/stats"
)

// runDump implements `fitmerge dump <file>`: a comprehensive read-only view of a
// single GPX/FIT file — format-specific header/stored data, a computed summary,
// which fields are present, per-lap figures, and (optionally) every record.
func runDump(args []string) error {
	fs := flag.NewFlagSet("fitmerge dump", flag.ContinueOnError)
	var (
		asJSON     bool
		allRecords bool
	)
	fs.BoolVar(&asJSON, "json", false, "output everything as JSON")
	fs.BoolVar(&allRecords, "records", false, "list every record (default: a small sample)")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: fitmerge dump [flags] <file.gpx|file.fit|file.tcx>\n\n"+
			"Show all information in a single activity file.\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("dump takes exactly one input file")
	}
	path := fs.Arg(0)

	act, err := format.Read(path)
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	kind, details, err := format.Inspect(path)
	if err != nil {
		return fmt.Errorf("inspecting %s: %w", path, err)
	}

	sum := stats.Compute(act.Records, stats.DefaultOptions())
	laps := lapSummaries(act)

	if asJSON {
		return dumpJSON(os.Stdout, path, kind, details, act, sum, laps)
	}
	dumpText(os.Stdout, path, kind, details, act, sum, laps, allRecords)
	return nil
}

// lapSummaries computes each lap's figures from the records inside its time
// window, so per-lap stats are consistent with the whole-file summary.
func lapSummaries(act model.Activity) []model.Summary {
	out := make([]model.Summary, 0, len(act.Laps))
	for _, lap := range act.Laps {
		var recs []model.Record
		for _, r := range act.Records {
			if !r.Time.Before(lap.StartTime) && !r.Time.After(lap.EndTime) {
				recs = append(recs, r)
			}
		}
		out = append(out, stats.Compute(recs, stats.DefaultOptions()))
	}
	return out
}

type fieldPresence struct {
	Position bool `json:"position"`
	Altitude bool `json:"altitude"`
	Distance bool `json:"distance"`
	Speed    bool `json:"speed"`
	HR       bool `json:"hr"`
	Cadence  bool `json:"cadence"`
	Power    bool `json:"power"`
	Temp     bool `json:"temp"`
}

func scanFields(recs []model.Record) fieldPresence {
	var p fieldPresence
	for _, r := range recs {
		p.Position = p.Position || r.HasPosition()
		p.Altitude = p.Altitude || r.Altitude != nil
		p.Distance = p.Distance || r.Distance != nil
		p.Speed = p.Speed || r.Speed != nil
		p.HR = p.HR || r.HR != nil
		p.Cadence = p.Cadence || r.Cadence != nil
		p.Power = p.Power || r.Power != nil
		p.Temp = p.Temp || r.Temp != nil
	}
	return p
}

func (p fieldPresence) present() []string {
	return p.filter(true)
}
func (p fieldPresence) missing() []string {
	return p.filter(false)
}
func (p fieldPresence) filter(want bool) []string {
	all := []struct {
		name string
		set  bool
	}{
		{"position", p.Position}, {"altitude", p.Altitude}, {"distance", p.Distance},
		{"speed", p.Speed}, {"hr", p.HR}, {"cadence", p.Cadence},
		{"power", p.Power}, {"temp", p.Temp},
	}
	var out []string
	for _, f := range all {
		if f.set == want {
			out = append(out, f.name)
		}
	}
	return out
}

// --- text output ---

func dumpText(w io.Writer, path string, kind format.Kind, details [][2]string,
	act model.Activity, sum model.Summary, laps []model.Summary, allRecords bool) {

	first, last := act.TimeBounds()
	fmt.Fprintf(w, "File:     %s\n", path)
	fmt.Fprintf(w, "Format:   %s\n", kind)
	if act.Sport != "" {
		fmt.Fprintf(w, "Sport:    %s\n", act.Sport)
	}
	fmt.Fprintf(w, "Records:  %d   (%s .. %s)\n", len(act.Records), fmtTime(first), fmtTime(last))

	fields := scanFields(act.Records)
	fmt.Fprintf(w, "Fields:   %s", strings.Join(orDash(fields.present()), ", "))
	if miss := fields.missing(); len(miss) > 0 {
		fmt.Fprintf(w, "   (missing: %s)", strings.Join(miss, ", "))
	}
	fmt.Fprintln(w)

	if len(details) > 0 {
		fmt.Fprintf(w, "\nFile details (%s)\n", kind)
		width := 0
		for _, kv := range details {
			if len(kv[0]) > width {
				width = len(kv[0])
			}
		}
		for _, kv := range details {
			fmt.Fprintf(w, "  %-*s  %s\n", width, kv[0]+":", kv[1])
		}
	}

	fmt.Fprintln(w, "\nComputed summary")
	fmt.Fprintf(w, "  Distance    %.2f km\n", sum.TotalDistance/1000)
	fmt.Fprintf(w, "  Ascent      %.0f m\n", sum.TotalAscent)
	fmt.Fprintf(w, "  Descent     %.0f m\n", sum.TotalDescent)
	fmt.Fprintf(w, "  Moving      %s\n", fmtDur(sum.TotalMoving))
	fmt.Fprintf(w, "  Elapsed     %s\n", fmtDur(sum.TotalElapsed))
	fmt.Fprintf(w, "  Avg speed   %.1f km/h\n", sum.AvgSpeed*3.6)
	fmt.Fprintf(w, "  Max speed   %.1f km/h\n", sum.MaxSpeed*3.6)
	if sum.MaxHR > 0 {
		fmt.Fprintf(w, "  Avg HR      %d bpm\n", sum.AvgHR)
		fmt.Fprintf(w, "  Max HR      %d bpm\n", sum.MaxHR)
	}

	if len(laps) > 0 {
		fmt.Fprintf(w, "\nLaps: %d\n", len(laps))
		fmt.Fprintf(w, "  %-3s %-21s %-9s %-9s %5s %5s\n", "#", "start", "duration", "distance", "asc", "desc")
		for i, l := range laps {
			fmt.Fprintf(w, "  %-3d %-21s %-9s %6.2f km %5.0f %5.0f\n",
				i+1, fmtTime(l.StartTime), fmtDur(l.TotalElapsed), l.TotalDistance/1000, l.TotalAscent, l.TotalDescent)
		}
	}

	if len(act.Records) > 0 {
		printRecords(w, act.Records, allRecords)
	}
}

func printRecords(w io.Writer, recs []model.Record, all bool) {
	const sample = 3
	show := recs
	note := ""
	if !all && len(recs) > 2*sample {
		note = fmt.Sprintf(" (first & last %d of %d; -records for all)", sample, len(recs))
	}

	fmt.Fprintf(w, "\nRecords%s\n", note)
	fmt.Fprintf(w, "  %-4s %-10s %-10s %-10s %6s %8s %6s %4s %4s %4s %4s\n",
		"#", "time", "lat", "lon", "alt", "dist", "spd", "hr", "cad", "pow", "tmp")

	emit := func(i int, r model.Record) {
		fmt.Fprintf(w, "  %-4d %-10s %-10s %-10s %6s %8s %6s %4s %4s %4s %4s\n",
			i+1, r.Time.UTC().Format("15:04:05Z"),
			fnum(r.Lat, "%.5f"), fnum(r.Lon, "%.5f"), fnum(r.Altitude, "%.0f"),
			fnum(r.Distance, "%.1f"), fnum(r.Speed, "%.2f"),
			unum(r.HR), unum(r.Cadence), pnum(r.Power), inum(r.Temp))
	}

	if all || len(recs) <= 2*sample {
		for i, r := range show {
			emit(i, r)
		}
		return
	}
	for i := 0; i < sample; i++ {
		emit(i, recs[i])
	}
	fmt.Fprintln(w, "  …")
	for i := len(recs) - sample; i < len(recs); i++ {
		emit(i, recs[i])
	}
}

// --- JSON output ---

func dumpJSON(w io.Writer, path string, kind format.Kind, details [][2]string,
	act model.Activity, sum model.Summary, laps []model.Summary) error {

	type kv struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}
	detailsJSON := make([]kv, len(details))
	for i, d := range details {
		detailsJSON[i] = kv{d[0], d[1]}
	}

	first, last := act.TimeBounds()
	out := map[string]any{
		"file":    path,
		"format":  string(kind),
		"sport":   act.Sport,
		"records": len(act.Records),
		"start":   timeOrEmpty(first),
		"end":     timeOrEmpty(last),
		"fields":  scanFields(act.Records),
		"details": detailsJSON,
		"summary": summaryJSON(sum),
		"laps":    lapsJSON(laps),
		"record_list": func() []map[string]any {
			rs := make([]map[string]any, len(act.Records))
			for i, r := range act.Records {
				rs[i] = recordJSON(r)
			}
			return rs
		}(),
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

func summaryJSON(s model.Summary) map[string]any {
	m := map[string]any{
		"distance_m":   round2(s.TotalDistance),
		"ascent_m":     s.TotalAscent,
		"descent_m":    s.TotalDescent,
		"moving_s":     s.TotalMoving.Seconds(),
		"elapsed_s":    s.TotalElapsed.Seconds(),
		"avg_speed_ms": round2(s.AvgSpeed),
		"max_speed_ms": round2(s.MaxSpeed),
		"records":      s.Records,
	}
	if s.MaxHR > 0 {
		m["avg_hr"] = s.AvgHR
		m["max_hr"] = s.MaxHR
	}
	return m
}

func lapsJSON(laps []model.Summary) []map[string]any {
	out := make([]map[string]any, len(laps))
	for i, l := range laps {
		out[i] = summaryJSON(l)
	}
	return out
}

func recordJSON(r model.Record) map[string]any {
	m := map[string]any{"time": timeOrEmpty(r.Time)}
	putF(m, "lat", r.Lat)
	putF(m, "lon", r.Lon)
	putF(m, "altitude_m", r.Altitude)
	putF(m, "distance_m", r.Distance)
	putF(m, "speed_ms", r.Speed)
	if r.HR != nil {
		m["hr"] = *r.HR
	}
	if r.Cadence != nil {
		m["cadence"] = *r.Cadence
	}
	if r.Power != nil {
		m["power"] = *r.Power
	}
	if r.Temp != nil {
		m["temp_c"] = *r.Temp
	}
	return m
}

// --- small formatting helpers ---

func orDash(s []string) []string {
	if len(s) == 0 {
		return []string{"—"}
	}
	return s
}

func fnum(p *float64, format string) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf(format, *p)
}

func unum(p *uint8) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *p)
}

func pnum(p *uint16) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *p)
}

func inum(p *int8) string {
	if p == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *p)
}

func putF(m map[string]any, key string, p *float64) {
	if p != nil {
		m[key] = round2(*p)
	}
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func timeOrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
