//go:build !js || !wasm

// This stub keeps the package buildable on non-wasm platforms (so `go build
// ./...` and CI don't choke on the constraint-excluded main). The real entry
// point lives in main.go behind a js && wasm build tag.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "fitmerge-wasm is only built for GOOS=js GOARCH=wasm; use ./cmd/fitmerge for the CLI")
	os.Exit(1)
}
