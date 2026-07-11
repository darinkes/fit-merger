package gpx

import (
	"path/filepath"
	"testing"
)

func TestInspect(t *testing.T) {
	kv, err := Inspect(filepath.Join("..", "..", "testdata", "part1.gpx"))
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]string{}
	for _, p := range kv {
		m[p[0]] = p[1]
	}
	if m["GPX version"] != "1.1" {
		t.Errorf("GPX version = %q, want 1.1", m["GPX version"])
	}
	if m["Tracks"] != "1" {
		t.Errorf("Tracks = %q, want 1", m["Tracks"])
	}
	if m["Track points"] != "5" {
		t.Errorf("Track points = %q, want 5", m["Track points"])
	}
}
