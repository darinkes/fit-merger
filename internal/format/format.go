// Package format routes reading and writing to the right codec based on file
// extension, keeping the CLI oblivious to concrete formats.
package format

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/rinkes/fit-merger/internal/fit"
	"github.com/rinkes/fit-merger/internal/gpx"
	"github.com/rinkes/fit-merger/internal/model"
)

// Kind identifies a supported activity file format.
type Kind string

const (
	GPX Kind = "gpx"
	FIT Kind = "fit"
)

// Detect infers the format from a path's extension.
func Detect(path string) (Kind, error) {
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".gpx":
		return GPX, nil
	case ".fit":
		return FIT, nil
	default:
		return "", fmt.Errorf("cannot determine format from extension %q (want .gpx or .fit)", ext)
	}
}

// Read decodes any supported file into the canonical model.
func Read(path string) (model.Activity, error) {
	kind, err := Detect(path)
	if err != nil {
		return model.Activity{}, err
	}
	switch kind {
	case GPX:
		return gpx.Read(path)
	case FIT:
		return fit.Read(path)
	default:
		return model.Activity{}, fmt.Errorf("unsupported format %q", kind)
	}
}

// Write encodes an activity to path in the format given by kind. The summary is
// used by formats (FIT) that store aggregate messages; GPX ignores it.
func Write(path string, kind Kind, act model.Activity, summary model.Summary) error {
	switch kind {
	case GPX:
		return gpx.Write(path, act)
	case FIT:
		return fit.Write(path, act, summary)
	default:
		return fmt.Errorf("unsupported output format %q", kind)
	}
}
