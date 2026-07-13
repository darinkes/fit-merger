package format_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/darinkes/fit-merger/internal/format"
)

// update regenerates the golden files instead of comparing against them:
//
//	go test ./internal/format -update
//
// Run it after an intentional change to either codec's output, then review the
// resulting diff in testdata/golden/ before committing.
var update = flag.Bool("update", false, "regenerate golden files")

// TestGoldenOutput pins the exact bytes produced when the two sample inputs are
// merged and encoded to each format. The semantic round-trip tests already prove
// the totals survive; this guards against *unintended* changes to the wire
// output (field ordering, extra messages, encoder settings) that a round-trip
// wouldn't catch. Both encoders are deterministic and carry no timestamp or
// version, so the bytes are stable across runs and platforms.
func TestGoldenOutput(t *testing.T) {
	res := mergeInputs(t)

	for _, kind := range []format.Kind{format.GPX, format.FIT, format.TCX} {
		t.Run(string(kind), func(t *testing.T) {
			data, err := format.Encode(kind, res.Activity, res.Summary)
			if err != nil {
				t.Fatal(err)
			}
			golden := filepath.Join("..", "..", "testdata", "golden", "merged."+string(kind))

			if *update {
				if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, data, 0o644); err != nil {
					t.Fatal(err)
				}
				t.Logf("wrote %s (%d bytes)", golden, len(data))
				return
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (create it with `go test ./internal/format -update`): %v", err)
			}
			if !bytes.Equal(want, data) {
				t.Errorf("%s output differs from golden: got %d bytes, want %d. "+
					"If this change is intentional, run `go test ./internal/format -update` and review the diff.",
					kind, len(data), len(want))
			}
		})
	}
}
