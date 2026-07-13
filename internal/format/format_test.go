package format_test

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/darinkes/fitmerge/internal/format"
	"github.com/darinkes/fitmerge/internal/merge"
	"github.com/darinkes/fitmerge/internal/model"
	"github.com/darinkes/fitmerge/internal/stats"
)

// testInputs are the two sample GPX parts shipped in the repo's testdata.
func testInputs(t *testing.T) []model.Activity {
	t.Helper()
	var acts []model.Activity
	for _, name := range []string{"part1.gpx", "part2.gpx"} {
		act, err := format.Read(filepath.Join("..", "..", "testdata", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		acts = append(acts, act)
	}
	return acts
}

func mergeInputs(t *testing.T) merge.Result {
	t.Helper()
	res, err := merge.Merge(testInputs(t), merge.Options{
		Sort: true, Overlap: merge.OverlapError, Stats: stats.DefaultOptions(),
	})
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// TestRoundTripEachFormat merges the sample inputs, writes the result to each
// output format, re-reads it, and checks the headline totals survive. This is
// the guarantee that matters: the merged file describes the combined track.
func TestRoundTripEachFormat(t *testing.T) {
	res := mergeInputs(t)
	want := res.Summary

	for _, kind := range []format.Kind{format.GPX, format.FIT, format.TCX} {
		t.Run(string(kind), func(t *testing.T) {
			out := filepath.Join(t.TempDir(), "merged."+string(kind))
			if err := format.Write(out, kind, res.Activity, res.Summary); err != nil {
				t.Fatal(err)
			}
			back, err := format.Read(out)
			if err != nil {
				t.Fatal(err)
			}
			got := stats.Compute(back.Records, stats.DefaultOptions())

			if len(back.Records) != len(res.Activity.Records) {
				t.Errorf("records = %d, want %d", len(back.Records), len(res.Activity.Records))
			}
			if math.Abs(got.TotalDistance-want.TotalDistance) > 1.0 {
				t.Errorf("distance = %.2f, want %.2f", got.TotalDistance, want.TotalDistance)
			}
			if got.TotalAscent != want.TotalAscent {
				t.Errorf("ascent = %.0f, want %.0f", got.TotalAscent, want.TotalAscent)
			}
			if got.TotalMoving != want.TotalMoving {
				t.Errorf("moving = %s, want %s", got.TotalMoving, want.TotalMoving)
			}
			if got.TotalElapsed != want.TotalElapsed {
				t.Errorf("elapsed = %s, want %s", got.TotalElapsed, want.TotalElapsed)
			}
			if got.MaxHR != want.MaxHR {
				t.Errorf("maxHR = %d, want %d", got.MaxHR, want.MaxHR)
			}
		})
	}
}

// TestDecodeEncodeBytes exercises the in-memory byte API that the WebAssembly
// build calls (no filesystem), mirroring the file round-trip above.
func TestDecodeEncodeBytes(t *testing.T) {
	res := mergeInputs(t)
	want := res.Summary

	for _, kind := range []format.Kind{format.GPX, format.FIT, format.TCX} {
		t.Run(string(kind), func(t *testing.T) {
			data, err := format.Encode(kind, res.Activity, res.Summary)
			if err != nil {
				t.Fatal(err)
			}
			if len(data) == 0 {
				t.Fatal("Encode returned no bytes")
			}
			back, err := format.Decode(data, kind)
			if err != nil {
				t.Fatal(err)
			}
			got := stats.Compute(back.Records, stats.DefaultOptions())
			if math.Abs(got.TotalDistance-want.TotalDistance) > 1.0 {
				t.Errorf("distance = %.2f, want %.2f", got.TotalDistance, want.TotalDistance)
			}
			if got.TotalAscent != want.TotalAscent {
				t.Errorf("ascent = %.0f, want %.0f", got.TotalAscent, want.TotalAscent)
			}
			if got.MaxHR != want.MaxHR {
				t.Errorf("maxHR = %d, want %d", got.MaxHR, want.MaxHR)
			}
		})
	}
}

// TestGapPreservedAsElapsedNotMoving guards the core concatenation semantic:
// the 10-minute gap between the two sample files counts as elapsed time but not
// moving time or distance.
func TestGapPreservedAsElapsedNotMoving(t *testing.T) {
	res := mergeInputs(t)
	if res.Summary.TotalElapsed != 10*time.Minute+40*time.Second {
		t.Errorf("elapsed = %s, want 10m40s", res.Summary.TotalElapsed)
	}
	if res.Summary.TotalMoving != 80*time.Second {
		t.Errorf("moving = %s, want 1m20s (gap excluded)", res.Summary.TotalMoving)
	}
}
