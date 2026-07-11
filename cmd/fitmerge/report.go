package main

import (
	"fmt"
	"io"
	"time"

	"github.com/rinkes/fit-merger/internal/merge"
	"github.com/rinkes/fit-merger/internal/model"
)

// printSummary renders the merged total plus a per-part breakdown, so the user
// can sanity-check that the whole equals the sum of its parts.
func printSummary(w io.Writer, res merge.Result) {
	fmt.Fprintln(w, "Merged activity")
	fmt.Fprintln(w, "───────────────")
	for i, p := range res.Parts {
		fmt.Fprintf(w, "  part %d: %s\n", i+1, summaryLine(p))
	}
	fmt.Fprintln(w, "  ─────")
	fmt.Fprintf(w, "  TOTAL:  %s\n", summaryLine(res.Summary))
}

func summaryLine(s model.Summary) string {
	return fmt.Sprintf(
		"%6.2f km  ↑%5.0f ↓%5.0f m  moving %s  elapsed %s  avg %4.1f max %4.1f km/h%s",
		s.TotalDistance/1000,
		s.TotalAscent, s.TotalDescent,
		fmtDur(s.TotalMoving), fmtDur(s.TotalElapsed),
		s.AvgSpeed*3.6, s.MaxSpeed*3.6,
		hrPart(s),
	)
}

func hrPart(s model.Summary) string {
	if s.MaxHR == 0 {
		return ""
	}
	return fmt.Sprintf("  HR avg %d max %d", s.AvgHR, s.MaxHR)
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%d:%02d:%02d", h, m, s)
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return t.UTC().Format("2006-01-02 15:04:05Z")
}
