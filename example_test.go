package phasedchecker_test

import (
	"fmt"
	"log"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"

	"github.com/miyamo2/phasedchecker"
)

// lintAnalyzer is a placeholder analyzer for demonstration purposes.
var lintAnalyzer = &analysis.Analyzer{
	Name: "lint",
	Doc:  "reports common style issues",
	Run: func(pass *analysis.Pass) (any, error) {
		return nil, nil
	},
}

// compatAnalyzer is a placeholder analyzer for demonstration purposes.
var compatAnalyzer = &analysis.Analyzer{
	Name: "compat",
	Doc:  "checks API compatibility",
	Run: func(pass *analysis.Pass) (any, error) {
		return nil, nil
	},
}

func ExampleRun() {
	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "lint",
					Analyzers: []*analysis.Analyzer{lintAnalyzer},
				},
				{
					Name:      "compat",
					Analyzers: []*analysis.Analyzer{compatAnalyzer},
					AfterPhase: func(graph *checker.Graph) error {
						fmt.Println("compat phase completed")
						return nil
					},
				},
			},
		},
		DiagnosticPolicy: phasedchecker.DiagnosticPolicy{
			Rules: []phasedchecker.CategoryRule{
				{Category: "deprecation", Severity: phasedchecker.SeverityWarn},
				{Category: "security", Severity: phasedchecker.SeverityCritical},
			},
			DefaultSeverity: phasedchecker.SeverityError,
		},
	}

	exitCode, err := phasedchecker.Run(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if exitCode != 0 {
		log.Fatalf("exit code: %d", exitCode)
	}
}
