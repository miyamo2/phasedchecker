package phasedchecker

import (
	"os"
	"reflect"
	"unsafe"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"

	"github.com/miyamo2/phasedchecker/internal/x/tools/driverutil"
)

// actionPass extracts the unexported pass field from a checker.Action using reflect.
func actionPass(act *checker.Action) *analysis.Pass {
	v := reflect.ValueOf(act).Elem()
	f := v.FieldByName("pass")
	if !f.IsValid() || f.IsNil() {
		return nil
	}
	return (*analysis.Pass)(unsafe.Pointer(f.Pointer()))
}

// applyFixes collects all diagnostics with SuggestedFixes from the graph
// and applies the first fix for each diagnostic to the source files.
func applyFixes(graph *checker.Graph, printDiff, verbose bool) error {
	var actions []driverutil.FixAction

	for act := range graph.All() {
		if !act.IsRoot || len(act.Diagnostics) == 0 {
			continue
		}

		readFile := func(filename string) ([]byte, error) {
			return os.ReadFile(filename)
		}
		if pass := actionPass(act); pass != nil && pass.ReadFile != nil {
			readFile = pass.ReadFile
		}

		action := driverutil.FixAction{
			Name:         act.String(),
			Pkg:          act.Package.Types,
			Files:        act.Package.Syntax,
			FileSet:      act.Package.Fset,
			ReadFileFunc: readFile,
			Diagnostics:  act.Diagnostics,
		}
		actions = append(actions, action)
	}

	if len(actions) == 0 {
		return nil
	}

	writeFile := func(filename string, content []byte) error {
		return os.WriteFile(filename, content, 0644)
	}

	return driverutil.ApplyFixes(actions, writeFile, printDiff, verbose)
}
