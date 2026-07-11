// Package fit decodes and encodes FIT files to and from the canonical activity
// model. Unlike GPX, FIT stores summary messages (session/lap) and cumulative
// distance directly, so encoding must recompute those to stay consistent with
// the merged record stream.
//
// Phase 2 implements this; for now the entry points report that FIT support is
// pending so the rest of the pipeline can be built and tested against GPX.
package fit

import (
	"errors"

	"github.com/rinkes/fit-merger/internal/model"
)

// ErrNotImplemented is returned until FIT support lands in Phase 2.
var ErrNotImplemented = errors.New("fit support not implemented yet (phase 2)")

// Read decodes a FIT file into an Activity.
func Read(path string) (model.Activity, error) {
	return model.Activity{}, ErrNotImplemented
}

// Write encodes an Activity as a FIT file.
func Write(path string, act model.Activity, summary model.Summary) error {
	return ErrNotImplemented
}
