package phasedchecker

import (
	"flag"
	"fmt"
	"strings"
)

// argument holds the runtime arguments that control how the checker executes.
// These are typically parsed from command-line flags via parseArgs.
type argument struct {
	// Fix enables automatic application of SuggestedFixes.
	Fix bool
	// PrintDiff, when used with Fix, prints unified diffs instead of updating files.
	PrintDiff bool
	// Debug holds debug flags, any subset of "fpstv".
	Debug string
	// Patterns are the package patterns to analyze (e.g., "./...").
	Patterns []string
}

// dbg reports whether the debug flag b is set.
func (a *argument) dbg(b byte) bool {
	return strings.IndexByte(a.Debug, b) >= 0
}

// parseArgs parses command-line arguments and returns argument.
func parseArgs(programName string, args []string) (*argument, error) {
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)

	var (
		fix       bool
		printDiff bool
		debug     string
	)
	fs.BoolVar(&fix, "fix", false, "apply suggested fixes")
	fs.BoolVar(&printDiff, "diff", false, "with -fix, don't update the files, but print a unified diff")
	fs.StringVar(&debug, "debug", "", `debug flags, any subset of "fpstv"`)

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	patterns := fs.Args()
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no packages specified")
	}

	return &argument{
		Fix:       fix,
		PrintDiff: printDiff,
		Debug:     debug,
		Patterns:  patterns,
	}, nil
}
