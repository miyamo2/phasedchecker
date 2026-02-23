// Command severity demonstrates DiagnosticPolicy for per-category severity control.
//
// Two analyzers report diagnostics with different categories:
//   - "todo"  → SeverityInfo  (not reported, no effect on exit code)
//   - "panic" → SeverityError (exit code 1)
//
// Usage:
//
//	go run . ./target/...
package main

import (
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/miyamo2/phasedchecker"
)

var todoAnalyzer = &analysis.Analyzer{
	Name: "todo",
	Doc:  "reports TODO comments",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			for _, cg := range f.Comments {
				for _, c := range cg.List {
					if strings.Contains(c.Text, "TODO") {
						pass.Report(analysis.Diagnostic{
							Pos:      c.Pos(),
							Message:  "TODO comment found",
							Category: "todo",
						})
					}
				}
			}
		}
		return nil, nil
	},
}

var panicCallAnalyzer = &analysis.Analyzer{
	Name: "paniccall",
	Doc:  "reports calls to panic()",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "panic" {
					pass.Report(analysis.Diagnostic{
						Pos:      call.Pos(),
						Message:  "call to panic()",
						Category: "panic",
					})
				}
				return true
			})
		}
		return nil, nil
	},
}

func main() {
	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "lint",
					Analyzers: []*analysis.Analyzer{todoAnalyzer, panicCallAnalyzer},
				},
			},
		},
		DiagnosticPolicy: phasedchecker.DiagnosticPolicy{
			Rules: []phasedchecker.CategoryRule{
				{Category: "todo", Severity: phasedchecker.SeverityInfo},
				{Category: "panic", Severity: phasedchecker.SeverityError},
			},
			DefaultSeverity: phasedchecker.SeverityWarn,
		},
	}
	phasedchecker.Main(cfg)
}
