// Command fitmerge merges two or more GPX/FIT activity files into a single
// GPX or FIT file, recomputing distance, ascent, moving time and speed so the
// output's totals faithfully describe the combined track.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/darinkes/fit-merger/internal/format"
	"github.com/darinkes/fit-merger/internal/merge"
	"github.com/darinkes/fit-merger/internal/model"
	"github.com/darinkes/fit-merger/internal/stats"
)

// version is overridable at build time with
// -ldflags "-X main.version=v1.2.3"; otherwise it falls back to the module
// version embedded by the Go toolchain.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "fitmerge:", err)
		os.Exit(1)
	}
}

// run dispatches to a subcommand. Merging is the default verb, so existing
// invocations (`fitmerge in1 in2 -o out`) keep working without a subcommand.
func run(args []string) error {
	if len(args) > 0 && args[0] == "dump" {
		return runDump(args[1:])
	}
	return runMerge(args)
}

type config struct {
	output          string
	formatName      string
	sort            bool
	overlap         string
	ascentThreshold float64
	movingThreshold float64
	use3D           bool
	sport           string
	dryRun          bool
	verbose         bool
}

func runMerge(args []string) error {
	fs := flag.NewFlagSet("fitmerge", flag.ContinueOnError)
	var c config
	fs.StringVar(&c.output, "o", "", "output file (.gpx, .fit or .tcx); format inferred from extension")
	fs.StringVar(&c.formatName, "format", "", "override output format: gpx|fit|tcx")
	fs.BoolVar(&c.sort, "sort", true, "order inputs by their first timestamp")
	fs.StringVar(&c.overlap, "overlap", "error", "when inputs overlap in time: error|keep|trim")
	fs.Float64Var(&c.ascentThreshold, "ascent-threshold", 3.0, "min sustained elevation change counted as climb (m)")
	fs.Float64Var(&c.movingThreshold, "moving-threshold", 0.5, "speed below which time is a pause (m/s)")
	fs.BoolVar(&c.use3D, "3d", false, "include elevation in distance measurement")
	fs.StringVar(&c.sport, "sport", "", "override sport tag on the output")
	fs.BoolVar(&c.dryRun, "dry-run", false, "compute and print the merged summary without writing output")
	fs.BoolVar(&c.verbose, "v", false, "verbose output")
	var showVersion bool
	fs.BoolVar(&showVersion, "version", false, "print version and exit")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: fitmerge [flags] <input1> <input2> [input3...]\n"+
			"       fitmerge dump [flags] <file>\n\n"+
			"Merge GPX/FIT/TCX files into one, recomputing all summary figures.\n"+
			"Use `fitmerge dump <file>` to inspect a single file.\n\nFlags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if showVersion {
		fmt.Println("fitmerge", buildVersion())
		return nil
	}

	inputs := fs.Args()
	if len(inputs) == 0 {
		fs.Usage()
		return fmt.Errorf("no input files given")
	}
	if !c.dryRun && c.output == "" {
		return fmt.Errorf("no output file: pass -o <file> or use -dry-run")
	}

	overlap := merge.OverlapStrategy(c.overlap)
	switch overlap {
	case merge.OverlapError, merge.OverlapKeep, merge.OverlapTrim:
	default:
		return fmt.Errorf("invalid -overlap %q (want error|keep|trim)", c.overlap)
	}

	// Decode all inputs.
	acts := make([]model.Activity, 0, len(inputs))
	for _, in := range inputs {
		act, err := format.Read(in)
		if err != nil {
			return fmt.Errorf("reading %s: %w", in, err)
		}
		if c.verbose {
			first, last := act.TimeBounds()
			fmt.Fprintf(os.Stderr, "read %s: %d records, %s .. %s\n",
				in, len(act.Records), fmtTime(first), fmtTime(last))
		}
		acts = append(acts, act)
	}

	statOpts := stats.Options{
		AscentThreshold:      c.ascentThreshold,
		MovingSpeedThreshold: c.movingThreshold,
		Use3D:                c.use3D,
	}
	res, err := merge.Merge(acts, merge.Options{Sort: c.sort, Overlap: overlap, Stats: statOpts})
	if err != nil {
		return err
	}
	if c.sport != "" {
		res.Activity.Sport = c.sport
	}

	printSummary(os.Stdout, res)

	if c.dryRun {
		fmt.Fprintln(os.Stdout, "\n(dry run: no output written)")
		return nil
	}

	kind, err := outputKind(c)
	if err != nil {
		return err
	}
	if err := format.Write(c.output, kind, res.Activity, res.Summary); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "\nwrote %s (%s, %d records)\n", c.output, kind, len(res.Activity.Records))
	return nil
}

// buildVersion prefers an explicit -ldflags version, then the module version
// the Go toolchain embeds for `go install`ed builds.
func buildVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func outputKind(c config) (format.Kind, error) {
	if c.formatName != "" {
		switch k := format.Kind(strings.ToLower(c.formatName)); k {
		case format.GPX, format.FIT, format.TCX:
			return k, nil
		default:
			return "", fmt.Errorf("invalid -format %q (want gpx|fit|tcx)", c.formatName)
		}
	}
	return format.Detect(c.output)
}
