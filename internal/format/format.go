// Package format routes reading and writing to the right codec based on file
// extension, keeping callers oblivious to concrete formats. It offers both
// path-based helpers (used by the CLI) and byte-slice helpers (used by the
// WebAssembly build, which has no filesystem).
package format

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/darinkes/fit-merger/internal/fit"
	"github.com/darinkes/fit-merger/internal/gpx"
	"github.com/darinkes/fit-merger/internal/model"
)

// Kind identifies a supported activity file format.
type Kind string

const (
	GPX Kind = "gpx"
	FIT Kind = "fit"
)

// Detect infers the format from a path or file name's extension.
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

// Read decodes a supported file at path into the canonical model.
func Read(path string) (model.Activity, error) {
	kind, err := Detect(path)
	if err != nil {
		return model.Activity{}, err
	}
	switch kind {
	case GPX:
		return gpx.ReadFile(path)
	case FIT:
		return fit.ReadFile(path)
	default:
		return model.Activity{}, fmt.Errorf("unsupported format %q", kind)
	}
}

// Write encodes an activity to path in the format given by kind. The summary is
// used by formats (FIT) that store aggregate messages; GPX ignores it.
func Write(path string, kind Kind, act model.Activity, summary model.Summary) error {
	switch kind {
	case GPX:
		return gpx.WriteFile(path, act)
	case FIT:
		return fit.WriteFile(path, act, summary)
	default:
		return fmt.Errorf("unsupported output format %q", kind)
	}
}

// Decode parses in-memory bytes of the given kind into the canonical model.
// The caller sets Sources (e.g. to the uploaded file name).
func Decode(data []byte, kind Kind) (model.Activity, error) {
	switch kind {
	case GPX:
		return gpx.Read(bytes.NewReader(data))
	case FIT:
		return fit.Read(bytes.NewReader(data))
	default:
		return model.Activity{}, fmt.Errorf("unsupported format %q", kind)
	}
}

// Encode serializes an activity to bytes in the given kind.
func Encode(kind Kind, act model.Activity, summary model.Summary) ([]byte, error) {
	var buf bytes.Buffer
	switch kind {
	case GPX:
		if err := gpx.Write(&buf, act); err != nil {
			return nil, err
		}
	case FIT:
		if err := fit.Write(&buf, act, summary); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported output format %q", kind)
	}
	return buf.Bytes(), nil
}
