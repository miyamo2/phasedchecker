package checkertest

import (
	"errors"
	"fmt"
	"go/ast"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/miyamo2/phasedchecker"
	"golang.org/x/tools/go/analysis"
	gochecker "golang.org/x/tools/go/analysis/checker"
)

// --- Test analyzers ---

// diagAnalyzer reports a diagnostic on the first token of every file.
var diagAnalyzer = &analysis.Analyzer{
	Name: "diag",
	Doc:  "reports a test diagnostic on var declarations",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			ast.Inspect(
				f, func(n ast.Node) bool {
					spec, ok := n.(*ast.ValueSpec)
					if !ok {
						return true
					}
					for _, name := range spec.Names {
						pass.Report(
							analysis.Diagnostic{
								Pos:     name.Pos(),
								Message: "test diagnostic",
							},
						)
					}
					return true
				},
			)
		}
		return nil, nil
	},
}

// renameAnalyzer renames "bar" to "baz" via SuggestedFix.
var renameAnalyzer = &analysis.Analyzer{
	Name: "rename",
	Doc:  "renames bar to baz",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			ast.Inspect(
				f, func(n ast.Node) bool {
					ident, ok := n.(*ast.Ident)
					if !ok || ident.Name != "bar" {
						return true
					}
					msg := fmt.Sprintf("renaming %q to %q", "bar", "baz")
					pass.Report(
						analysis.Diagnostic{
							Pos:     ident.Pos(),
							End:     ident.End(),
							Message: msg,
							SuggestedFixes: []analysis.SuggestedFix{
								{
									Message: msg,
									TextEdits: []analysis.TextEdit{
										{
											Pos:     ident.Pos(),
											End:     ident.End(),
											NewText: []byte("baz"),
										},
									},
								},
							},
						},
					)
					return true
				},
			)
		}
		return nil, nil
	},
}

func testdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "testdata")
}

// --- parseExpectations unit tests ---

func TestParseExpectations(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		text       string
		wantDelta  int
		wantCount  int
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:      "single string",
			text:      ` "hello"`,
			wantDelta: 0,
			wantCount: 1,
		},
		{
			name:      "multiple strings",
			text:      ` "foo" "bar"`,
			wantDelta: 0,
			wantCount: 2,
		},
		{
			name:      "line offset",
			text:      ` +3 "pattern"`,
			wantDelta: 3,
			wantCount: 1,
		},
		{
			name:      "raw string",
			text:      " `raw.*pattern`",
			wantDelta: 0,
			wantCount: 1,
		},
		{
			name:      "empty",
			text:      "",
			wantDelta: 0,
			wantCount: 0,
		},
		{
			name:       "invalid regex",
			text:       ` "[invalid"`,
			wantErr:    true,
			wantErrMsg: "invalid regex",
		},
		{
			name:       "fact form rejected",
			text:       ` foo:"pattern"`,
			wantErr:    true,
			wantErrMsg: "fact expectations",
		},
		{
			name:       "plus without number",
			text:       ` + "pattern"`,
			wantErr:    true,
			wantErrMsg: "expected line number",
		},
		{
			name:       "unterminated string",
			text:       ` "unterminated`,
			wantErr:    true,
			wantErrMsg: "literal not terminated",
		},
		{
			name:       "unexpected token",
			text:       ` ;`,
			wantErr:    true,
			wantErrMsg: "unexpected token",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				delta, expects, err := parseExpectations(tt.text)
				if tt.wantErr {
					if err == nil {
						t.Fatalf("expected error containing %q, got nil", tt.wantErrMsg)
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if delta != tt.wantDelta {
					t.Errorf("lineDelta = %d, want %d", delta, tt.wantDelta)
				}
				if len(expects) != tt.wantCount {
					t.Errorf("len(expects) = %d, want %d", len(expects), tt.wantCount)
				}
			},
		)
	}
}

// --- Integration tests ---

func TestRun_BlockCommentWant(t *testing.T) {
	dir := filepath.Join(testdataDir(), "blockcomment")

	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "test",
					Analyzers: []*analysis.Analyzer{diagAnalyzer},
				},
			},
		},
	}

	results := Run(t, dir, cfg, "./...")

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
}

func TestRun_BasicDiagnostic(t *testing.T) {
	dir := filepath.Join(testdataDir(), "basic")

	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "test",
					Analyzers: []*analysis.Analyzer{diagAnalyzer},
				},
			},
		},
	}

	results := Run(t, dir, cfg, "./...")

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Phase != "test" {
		t.Errorf("phase = %q, want %q", results[0].Phase, "test")
	}
}

func TestRunWithSuggestedFixes_Golden(t *testing.T) {
	dir := filepath.Join(testdataDir(), "golden")

	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "test",
					Analyzers: []*analysis.Analyzer{renameAnalyzer},
				},
			},
		},
	}

	results := RunWithSuggestedFixes(t, dir, cfg, "./...")

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
}

// criticalDiagAnalyzer reports a diagnostic with category "crit" on var declarations.
var criticalDiagAnalyzer = &analysis.Analyzer{
	Name: "critdiag",
	Doc:  "reports a diagnostic with category crit on var declarations",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			ast.Inspect(
				f, func(n ast.Node) bool {
					spec, ok := n.(*ast.ValueSpec)
					if !ok {
						return true
					}
					for _, name := range spec.Names {
						pass.Report(
							analysis.Diagnostic{
								Pos:      name.Pos(),
								Message:  "test diagnostic",
								Category: "crit",
							},
						)
					}
					return true
				},
			)
		}
		return nil, nil
	},
}

func TestRun_SeverityCritical_SkipsSubsequentPhases(t *testing.T) {
	dir := filepath.Join(testdataDir(), "critical")

	var afterPhaseCalled []string
	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "phase1",
					Analyzers: []*analysis.Analyzer{criticalDiagAnalyzer},
					AfterPhase: func(_ *gochecker.Graph) error {
						afterPhaseCalled = append(afterPhaseCalled, "phase1")
						return nil
					},
				},
				{
					Name:      "phase2",
					Analyzers: []*analysis.Analyzer{diagAnalyzer},
					AfterPhase: func(_ *gochecker.Graph) error {
						afterPhaseCalled = append(afterPhaseCalled, "phase2")
						return nil
					},
				},
			},
		},
		DiagnosticPolicy: phasedchecker.DiagnosticPolicy{
			Rules: []phasedchecker.CategoryRule{
				{Category: "crit", Severity: phasedchecker.SeverityCritical},
			},
		},
	}

	results := Run(t, dir, cfg, "./...")

	// Only phase1 should produce a result; phase2 should be skipped.
	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
	if results[0].Phase != "phase1" {
		t.Errorf("phase = %q, want %q", results[0].Phase, "phase1")
	}

	// AfterPhase should NOT be called for the critical phase (nor for skipped phases).
	if len(afterPhaseCalled) != 0 {
		t.Errorf("afterPhaseCalled = %v, want empty (critical phase should skip AfterPhase)", afterPhaseCalled)
	}
}

func TestRun_SeverityCritical_DiagnosticsMatch(t *testing.T) {
	dir := filepath.Join(testdataDir(), "critical")

	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "test",
					Analyzers: []*analysis.Analyzer{criticalDiagAnalyzer},
				},
			},
		},
		DiagnosticPolicy: phasedchecker.DiagnosticPolicy{
			Rules: []phasedchecker.CategoryRule{
				{Category: "crit", Severity: phasedchecker.SeverityCritical},
			},
		},
	}

	// The // want "test diagnostic" directive in critical/main.go should match
	// even though the diagnostic is SeverityCritical.
	results := Run(t, dir, cfg, "./...")

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
}

func TestRun_AnalyzerError(t *testing.T) {
	dir := filepath.Join(testdataDir(), "noexpect")

	errAnalyzer := &analysis.Analyzer{
		Name: "errana",
		Doc:  "always returns an error",
		Run: func(pass *analysis.Pass) (any, error) {
			return nil, errors.New("analysis failed")
		},
	}

	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "test",
					Analyzers: []*analysis.Analyzer{errAnalyzer},
				},
			},
		},
	}

	// The analyzer errors; matchDiagnostics should skip actions with Err != nil.
	// No // want directives in noexpect, so no unmatched expectations either.
	results := Run(t, dir, cfg, "./...")

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
}
