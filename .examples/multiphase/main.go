// Command multiphase demonstrates inter-phase communication via AfterPhase callbacks.
//
// Phase 1 ("scan") collects deprecated functions from doc comments.
// The AfterPhase callback merges per-package results into a global set.
// Phase 2 ("lint") detects calls to those deprecated functions.
//
// Usage:
//
//	go run . ./target/...
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"reflect"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"

	"github.com/miyamo2/phasedchecker"
)

// deprecatedFuncs holds the set of deprecated function keys ("pkg/path.FuncName").
// Populated by the AfterPhase callback of the "scan" phase,
// then read by the "lint" phase.
var deprecatedFuncs = make(map[string]bool)

var scanAnalyzer = &analysis.Analyzer{
	Name:       "scan",
	Doc:        "collects deprecated functions from doc comments",
	ResultType: reflect.TypeOf(map[string]bool{}),
	Run: func(pass *analysis.Pass) (any, error) {
		result := make(map[string]bool)
		for _, f := range pass.Files {
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Doc == nil {
					continue
				}
				for _, c := range fn.Doc.List {
					if strings.Contains(c.Text, "Deprecated:") {
						key := pass.Pkg.Path() + "." + fn.Name.Name
						result[key] = true
						break
					}
				}
			}
		}
		return result, nil
	},
}

var checkAnalyzer = &analysis.Analyzer{
	Name: "check",
	Doc:  "reports calls to deprecated functions",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var fn *types.Func
				switch fun := call.Fun.(type) {
				case *ast.SelectorExpr:
					if use, ok := pass.TypesInfo.Uses[fun.Sel]; ok {
						if f, ok := use.(*types.Func); ok {
							fn = f
						}
					}
				case *ast.Ident:
					if use, ok := pass.TypesInfo.Uses[fun]; ok {
						if f, ok := use.(*types.Func); ok {
							fn = f
						}
					}
				}
				if fn != nil {
					key := fn.Pkg().Path() + "." + fn.Name()
					if deprecatedFuncs[key] {
						pass.Report(analysis.Diagnostic{
							Pos:      call.Pos(),
							Message:  fmt.Sprintf("call to deprecated function %s", fn.Name()),
							Category: "deprecated",
						})
					}
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
	phasedchecker.Main(cfg)
}
