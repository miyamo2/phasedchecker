// Command basic demonstrates a single-phase naming convention checker.
//
// It detects exported ALL_CAPS variable names and reports them as warnings.
//
// Usage:
//
//	go run . ./target/...
package main

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"

	"github.com/miyamo2/phasedchecker"
)

var namingAnalyzer = &analysis.Analyzer{
	Name: "naming",
	Doc:  "reports exported ALL_CAPS variable names",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				vs, ok := n.(*ast.ValueSpec)
				if !ok {
					return true
				}
				for _, name := range vs.Names {
					if name.IsExported() && isAllCaps(name.Name) {
						pass.Report(analysis.Diagnostic{
							Pos:      name.Pos(),
							Message:  fmt.Sprintf("variable %q uses ALL_CAPS naming", name.Name),
							Category: "naming",
						})
					}
				}
				return true
			})
		}
		return nil, nil
	},
}

// isAllCaps reports whether s consists of at least two characters,
// all of which are uppercase ASCII letters, digits, or underscores.
func isAllCaps(s string) bool {
	if len(s) < 2 {
		return false
	}
	for _, r := range s {
		if r != '_' && (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}

func main() {
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
	phasedchecker.Main(cfg)
}
