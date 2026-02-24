// Package checkertest provides testing utilities for phase-based analysis pipelines.
// It is analogous to [analysistest] but designed for [phasedchecker.Config] pipelines,
// verifying diagnostics against // want directives and optionally checking suggested
// fixes against .golden files.
//
// [analysistest]: golang.org/x/tools/go/analysis/analysistest
package checkertest

import (
	"errors"
	"testing"

	"github.com/miyamo2/phasedchecker"
	"github.com/miyamo2/phasedchecker/checkertest/internal"
	"github.com/miyamo2/phasedchecker/internal/runner"
	gochecker "golang.org/x/tools/go/analysis/checker"
)

// Result holds the result of executing a single phase.
type Result struct {
	// Phase is the name of the executed phase.
	Phase string
	// Graph is the analysis result graph produced by [checker.Analyze] for this phase.
	Graph *gochecker.Graph
}

// Run executes the checker Config's Pipeline against the packages matched by
// patterns in dir, and verifies that all diagnostics from all phases match
// the // want directives in the source files.
func Run(t *testing.T, dir string, cfg phasedchecker.Config, patterns ...string) []*Result {
	t.Helper()
	return runPipeline(t, dir, cfg, false, patterns)
}

// RunWithSuggestedFixes is like Run but additionally applies SuggestedFixes
// and compares the results against .golden files.
func RunWithSuggestedFixes(t *testing.T, dir string, cfg phasedchecker.Config, patterns ...string) []*Result {
	t.Helper()
	return runPipeline(t, dir, cfg, true, patterns)
}

func runPipeline(t internal.T, dir string, cfg phasedchecker.Config, checkGolden bool, patterns []string) []*Result {
	t.Helper()

	if len(cfg.Pipeline.Phases) == 0 {
		t.Fatal("pipeline has no phases")
	}

	pkgs, err := runner.LoadPackages(dir, true, patterns)
	if err != nil {
		t.Fatalf("loading packages: %v", err)
	}

	// Collect // want expectations from source files.
	wants := collectExpectations(t, pkgs)

	// Optionally capture original file content for golden comparison.
	var gf *goldenFiles
	if checkGolden {
		gf = newGoldenFiles()
		gf.capture(pkgs)
	}

	var (
		results []*Result
		graphs  []*gochecker.Graph
		aborted bool
	)

	defer func() {
		// Report unmatched expectations.
		reportUnmatched(t, wants)

		// Compare golden files if requested.
		if !aborted && checkGolden {
			compareGolden(t, graphs, gf)
		}
	}()

	for pr, err := range runner.RunPipeline(cfg, pkgs, nil) {
		if pr != nil {
			matchDiagnostics(t, pr.Graph, wants)
			results = append(results, &Result{Phase: pr.Phase, Graph: pr.Graph})
			graphs = append(graphs, pr.Graph)
		}
		if err != nil {
			aborted = true
			if errors.Is(err, runner.ErrAfterPhase) {
				t.Fatal(err)
			}
			break
		}
	}

	return results
}

// matchDiagnostics checks all diagnostics from the graph against expectations.
func matchDiagnostics(
	t internal.T, graph *gochecker.Graph, wants map[expectKey][]*expectation,
) {
	t.Helper()
	for act := range graph.All() {
		if act.Err != nil {
			continue
		}
		if !act.IsRoot {
			continue
		}
		for _, d := range act.Diagnostics {
			posn := act.Package.Fset.Position(d.Pos)
			checkDiagnostics(t, wants, posn, d.Message)
		}
	}
}
