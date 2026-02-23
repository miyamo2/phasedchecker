package main

import (
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"

	"github.com/miyamo2/phasedchecker"
	"github.com/miyamo2/phasedchecker/checkertest"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

func TestMultiPhaseDeprecation(t *testing.T) {
	deprecatedFuncs = make(map[string]bool) // reset global state
	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "scan",
					Analyzers: []*analysis.Analyzer{scanAnalyzer},
					AfterPhase: func(graph *checker.Graph) error {
						for act := range graph.All() {
							if act.Analyzer != scanAnalyzer || act.Err != nil {
								continue
							}
							if result, ok := act.Result.(map[string]bool); ok {
								for k := range result {
									deprecatedFuncs[k] = true
								}
							}
						}
						return nil
					},
				},
				{
					Name:      "lint",
					Analyzers: []*analysis.Analyzer{checkAnalyzer},
				},
			},
		},
		DiagnosticPolicy: phasedchecker.DiagnosticPolicy{
			DefaultSeverity: phasedchecker.SeverityWarn,
		},
	}
	checkertest.Run(t, testdataDir(), cfg, "./...")
}
