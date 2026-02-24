package arg

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

// Argument holds the runtime arguments that control how the checker executes.
// These are typically parsed from command-line flags via ParseArgs.
type Argument struct {
	// Fix enables automatic application of SuggestedFixes.
	Fix bool
	// PrintDiff, when used with Fix, prints unified diffs instead of updating files.
	PrintDiff bool
	// JSON enables JSON output of diagnostics to stdout.
	JSON bool
	// Test indicates whether test files should be analyzed.
	Test bool
	// Debug holds debug flags, any subset of "fpstv".
	Debug string
	// Patterns are the package patterns to analyze (e.g., "./...").
	Patterns []string
}

// Dbg reports whether the debug flag b is set.
func (a *Argument) Dbg(b byte) bool {
	return strings.IndexByte(a.Debug, b) >= 0
}

// versionFlag minimally complies with the -V protocol required by "go vet".
type versionFlag struct{}

func (versionFlag) IsBoolFlag() bool { return true }
func (versionFlag) Get() any         { return nil }
func (versionFlag) String() string   { return "" }
func (versionFlag) Set(s string) error {
	if s != "full" {
		log.Fatalf("unsupported flag value: -V=%s (use -V=full)", s)
	}
	progname, err := os.Executable()
	if err != nil {
		return err
	}
	f, err := os.Open(progname)
	if err != nil {
		log.Fatal(err)
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}
	f.Close()
	fmt.Printf(
		"%s version devel comments-go-here buildID=%02x\n",
		progname, string(h.Sum(nil)),
	)
	os.Exit(0)
	return nil
}

// ParseArgs parses command-line arguments and returns Argument.
func ParseArgs(programName string, args []string) (*Argument, error) {
	fs := flag.NewFlagSet(programName, flag.ContinueOnError)

	var (
		fix       bool
		printDiff bool
		jsonMode  bool
		test      bool
		debug     string
	)
	fs.BoolVar(&fix, "fix", false, "apply suggested fixes")
	fs.BoolVar(&printDiff, "diff", false, "with -fix, don't update the files, but print a unified diff")
	fs.BoolVar(&jsonMode, "json", false, "emit JSON output to stdout")
	fs.BoolVar(&test, "test", true, "indicates whether test files should be analyzed")
	fs.StringVar(&debug, "debug", "", `debug flags, any subset of "fpstv"`)
	fs.Var(versionFlag{}, "V", "print version and exit")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}

	patterns := fs.Args()
	if len(patterns) == 0 {
		return nil, fmt.Errorf("no packages specified")
	}

	return &Argument{
		Fix:       fix,
		PrintDiff: printDiff,
		JSON:      jsonMode,
		Test:      test,
		Debug:     debug,
		Patterns:  patterns,
	}, nil
}
