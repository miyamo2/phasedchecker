package main

import (
	"path/filepath"
	"runtime"
	"testing"

	"golang.org/x/tools/go/analysis"

	"github.com/miyamo2/phasedchecker"
	"github.com/miyamo2/phasedchecker/checkertest"
)

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

func TestNamingAnalyzer(t *testing.T) {
	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "naming",
					Analyzers: []*analysis.Analyzer{namingAnalyzer},
				},
			},
		},
		DiagnosticPolicy: phasedchecker.DiagnosticPolicy{
			DefaultSeverity: phasedchecker.SeverityWarn,
		},
	}
	checkertest.Run(t, testdataDir(), cfg, "./...")
}
