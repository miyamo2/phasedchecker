package phasedchecker

import (
	"os"

	"golang.org/x/tools/go/analysis/checker"

	"github.com/miyamo2/phasedchecker/internal/x/tools/driverutil"
)

// applyFixes collects all diagnostics with SuggestedFixes from the graph
// and applies the first fix for each diagnostic to the source files.
func applyFixes(graph *checker.Graph, printDiff, verbose bool) error {
	var actions []driverutil.FixAction

	for act := range graph.All() {
		if !act.IsRoot || len(act.Diagnostics) == 0 {
			continue
		}

		action := driverutil.FixAction{
			Name:    act.String(),
			Pkg:     act.Package.Types,
			Files:   act.Package.Syntax,
			FileSet: act.Package.Fset,
			ReadFileFunc: func(filename string) ([]byte, error) {
				return os.ReadFile(filename)
			},
			Diagnostics: act.Diagnostics,
		}
		actions = append(actions, action)
	}

	if len(actions) == 0 {
		return nil
	}

	writeFile := func(filename string, content []byte) error {
		perm := os.FileMode(0644)
		if fi, err := os.Stat(filename); err == nil {
			perm = fi.Mode().Perm()
		}
		return os.WriteFile(filename, content, perm)
	}

	return driverutil.ApplyFixes(actions, writeFile, printDiff, verbose)
}
