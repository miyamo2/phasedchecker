// Package checkertest provides testing utilities for the checker package.
//
// It is an alternative to golang.org/x/tools/go/analysis/analysistest
// designed for checker's phase-based pipeline model. Like analysistest,
// it supports // want directives for diagnostic matching and .golden files
// for SuggestedFix verification.
//
// Usage:
//
//	func TestMyAnalyzer(t *testing.T) {
//	    checkertest.Run(t, "testdata/mycase", checker.Config{...}, "./...")
//	}
//
//	func TestMyAnalyzerFixes(t *testing.T) {
//	    checkertest.RunWithSuggestedFixes(t, "testdata/mycase", checker.Config{...}, "./...")
//	}
package phasedchecker

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miyamo2/phasedchecker/severity"
	"golang.org/x/tools/go/analysis"
	gochecker "golang.org/x/tools/go/analysis/checker"
)

// --- Test analyzers ---

var noopAnalyzer = &analysis.Analyzer{
	Name: "noop",
	Doc:  "does nothing",
	Run: func(pass *analysis.Pass) (any, error) {
		return nil, nil
	},
}

var failAnalyzer = &analysis.Analyzer{
	Name: "fail",
	Doc:  "always fails",
	Run: func(pass *analysis.Pass) (any, error) {
		return nil, fmt.Errorf("analysis failed")
	},
}

func newDiagAnalyzer(category string) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: "diag_" + category,
		Doc:  "reports diagnostic with category " + category,
		Run: func(pass *analysis.Pass) (any, error) {
			if len(pass.Files) > 0 {
				pass.Report(
					analysis.Diagnostic{
						Pos:      pass.Files[0].Pos(),
						Message:  "test diagnostic",
						Category: category,
					},
				)
			}
			return nil, nil
		},
	}
}

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

// --- Helpers ---

func setupTestModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()

	gomod := "module example.com/test\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatal(err)
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	return dir
}

const minimalMain = `package main

func main() {}
`

// --- Tests ---

func Test_run_EmptyPipeline(t *testing.T) {
	code, err := run(
		Config{}, &argument{
			Patterns: []string{"./..."},
		},
	)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "pipeline has no phases") {
		t.Errorf("error = %v, want containing %q", err, "pipeline has no phases")
	}
}

func Test_run_NilArgs(t *testing.T) {
	code, err := run(Config{}, nil)
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "args cannot be nil") {
		t.Errorf("error = %v, want containing %q", err, "args cannot be nil")
	}
}

func Test_run_ExitCodes(t *testing.T) {
	dir := setupTestModule(
		t, map[string]string{
			"main.go": minimalMain,
		},
	)
	t.Chdir(dir)

	tests := []struct {
		name       string
		cfg        Config
		args       *argument
		wantCode   int
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "noop clean",
			cfg: Config{
				Pipeline: Pipeline{
					Phases: []Phase{
						{
							Name:      "test",
							Analyzers: []*analysis.Analyzer{noopAnalyzer},
						},
					},
				},
				DiagnosticPolicy: severity.DiagnosticPolicy{DefaultSeverity: severity.SeverityInfo},
			},
			args:     &argument{Patterns: []string{"./..."}},
			wantCode: 0,
		},
		{
			name: "fail error",
			cfg: Config{
				Pipeline: Pipeline{
					Phases: []Phase{
						{
							Name:      "test",
							Analyzers: []*analysis.Analyzer{failAnalyzer},
						},
					},
				},
			},
			args:     &argument{Patterns: []string{"./..."}},
			wantCode: 1,
		},
		{
			name: "diag SeverityError",
			cfg: Config{
				Pipeline: Pipeline{
					Phases: []Phase{
						{
							Name:      "test",
							Analyzers: []*analysis.Analyzer{newDiagAnalyzer("err")},
						},
					},
				},
				DiagnosticPolicy: severity.DiagnosticPolicy{
					Rules: []severity.CategoryRule{{Category: "err", Severity: severity.SeverityError}},
				},
			},
			args:     &argument{Patterns: []string{"./..."}},
			wantCode: 1,
		},
		{
			name: "diag SeverityWarn",
			cfg: Config{
				Pipeline: Pipeline{
					Phases: []Phase{
						{
							Name:      "test",
							Analyzers: []*analysis.Analyzer{newDiagAnalyzer("warn")},
						},
					},
				},
				DiagnosticPolicy: severity.DiagnosticPolicy{
					Rules: []severity.CategoryRule{{Category: "warn", Severity: severity.SeverityWarn}},
				},
			},
			args:     &argument{Patterns: []string{"./..."}},
			wantCode: 3,
		},
		{
			name: "diag SeverityInfo",
			cfg: Config{
				Pipeline: Pipeline{
					Phases: []Phase{
						{
							Name:      "test",
							Analyzers: []*analysis.Analyzer{newDiagAnalyzer("info")},
						},
					},
				},
				DiagnosticPolicy: severity.DiagnosticPolicy{DefaultSeverity: severity.SeverityInfo},
			},
			args:     &argument{Patterns: []string{"./..."}},
			wantCode: 0,
		},
		{
			name: "warn with fix",
			cfg: Config{
				Pipeline: Pipeline{
					Phases: []Phase{
						{
							Name:      "test",
							Analyzers: []*analysis.Analyzer{newDiagAnalyzer("warn")},
						},
					},
				},
				DiagnosticPolicy: severity.DiagnosticPolicy{
					Rules: []severity.CategoryRule{{Category: "warn", Severity: severity.SeverityWarn}},
				},
			},
			args:     &argument{Fix: true, Patterns: []string{"./..."}},
			wantCode: 0,
		},
		{
			name: "error takes precedence over warn",
			cfg: Config{
				Pipeline: Pipeline{
					Phases: []Phase{
						{
							Name: "test",
							Analyzers: []*analysis.Analyzer{
								newDiagAnalyzer("err"),
								newDiagAnalyzer("warn"),
							},
						},
					},
				},
				DiagnosticPolicy: severity.DiagnosticPolicy{
					Rules: []severity.CategoryRule{
						{Category: "err", Severity: severity.SeverityError},
						{Category: "warn", Severity: severity.SeverityWarn},
					},
				},
			},
			args:     &argument{Patterns: []string{"./..."}},
			wantCode: 1,
		},
		{
			name: "diag SeverityCritical",
			cfg: Config{
				Pipeline: Pipeline{
					Phases: []Phase{
						{
							Name:      "test",
							Analyzers: []*analysis.Analyzer{newDiagAnalyzer("crit")},
						},
					},
				},
				DiagnosticPolicy: severity.DiagnosticPolicy{
					Rules: []severity.CategoryRule{{Category: "crit", Severity: severity.SeverityCritical}},
				},
			},
			args:       &argument{Patterns: []string{"./..."}},
			wantCode:   1,
			wantErr:    true,
			wantErrMsg: "critical diagnostic",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				code, err := run(tt.cfg, tt.args)
				if tt.wantErr && err == nil {
					t.Fatal("expected error, got nil")
				}
				if !tt.wantErr && err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tt.wantErrMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.wantErrMsg)) {
					t.Errorf("error = %v, want containing %q", err, tt.wantErrMsg)
				}
				if code != tt.wantCode {
					t.Errorf("exit code = %d, want %d", code, tt.wantCode)
				}
			},
		)
	}
}

func Test_run_MultiPhase(t *testing.T) {
	dir := setupTestModule(
		t, map[string]string{
			"main.go": minimalMain,
		},
	)
	t.Chdir(dir)

	var phases []string
	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "phase1",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
					AfterPhase: func(_ *gochecker.Graph) error {
						phases = append(phases, "phase1")
						return nil
					},
				},
				{
					Name:      "phase2",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
					AfterPhase: func(_ *gochecker.Graph) error {
						phases = append(phases, "phase2")
						return nil
					},
				},
			},
		},
	}

	code, err := run(cfg, &argument{Patterns: []string{"./..."}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if len(phases) != 2 || phases[0] != "phase1" || phases[1] != "phase2" {
		t.Errorf("phases = %v, want [phase1 phase2]", phases)
	}
}

func Test_run_AfterPhaseError(t *testing.T) {
	dir := setupTestModule(
		t, map[string]string{
			"main.go": minimalMain,
		},
	)
	t.Chdir(dir)

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "phase1",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
					AfterPhase: func(_ *gochecker.Graph) error {
						return fmt.Errorf("callback error")
					},
				},
			},
		},
	}

	code, err := run(cfg, &argument{Patterns: []string{"./..."}})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if err == nil || !strings.Contains(err.Error(), "callback error") {
		t.Errorf("error = %v, want containing %q", err, "callback error")
	}
}

func Test_run_MultiPhase_ErrorStopsEarly(t *testing.T) {
	dir := setupTestModule(
		t, map[string]string{
			"main.go": minimalMain,
		},
	)
	t.Chdir(dir)

	var phases []string
	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "phase1",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
					AfterPhase: func(_ *gochecker.Graph) error {
						phases = append(phases, "phase1")
						return fmt.Errorf("phase1 failed")
					},
				},
				{
					Name:      "phase2",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
					AfterPhase: func(_ *gochecker.Graph) error {
						phases = append(phases, "phase2")
						return nil
					},
				},
			},
		},
	}

	code, err := run(cfg, &argument{Patterns: []string{"./..."}})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(phases) != 1 || phases[0] != "phase1" {
		t.Errorf("phases = %v, want [phase1]", phases)
	}
}

func Test_run_NonRootActionsSkipped(t *testing.T) {
	dir := setupTestModule(
		t, map[string]string{
			"main.go": minimalMain,
		},
	)
	t.Chdir(dir)

	// depAnalyzer is required by rootAnalyzer; its actions will be non-root.
	depAnalyzer := &analysis.Analyzer{
		Name: "dep",
		Doc:  "dependency analyzer that reports a diagnostic",
		Run: func(pass *analysis.Pass) (any, error) {
			if len(pass.Files) > 0 {
				pass.Report(analysis.Diagnostic{
					Pos:     pass.Files[0].Pos(),
					Message: "dep diagnostic",
				})
			}
			return nil, nil
		},
	}
	rootAnalyzer := &analysis.Analyzer{
		Name:     "root",
		Doc:      "root analyzer with dependency",
		Requires: []*analysis.Analyzer{depAnalyzer},
		Run: func(pass *analysis.Pass) (any, error) {
			return nil, nil
		},
	}

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "test",
					Analyzers: []*analysis.Analyzer{rootAnalyzer},
				},
			},
		},
		DiagnosticPolicy: severity.DiagnosticPolicy{
			DefaultSeverity: severity.SeverityError,
		},
	}

	// depAnalyzer reports a diagnostic, but it's non-root so should be skipped.
	// If it weren't skipped, the exit code would be 1 (Error).
	code, err := run(cfg, &argument{Patterns: []string{"./..."}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (non-root diagnostics should be skipped)", code)
	}
}

func Test_run_DiagAccumulation_AcrossPhases(t *testing.T) {
	dir := setupTestModule(
		t, map[string]string{
			"main.go": minimalMain,
		},
	)
	t.Chdir(dir)

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "phase1",
					Analyzers: []*analysis.Analyzer{newDiagAnalyzer("warn")},
				},
				{
					Name:      "phase2",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
				},
			},
		},
		DiagnosticPolicy: severity.DiagnosticPolicy{
			Rules: []severity.CategoryRule{{Category: "warn", Severity: severity.SeverityWarn}},
		},
	}

	code, err := run(cfg, &argument{Patterns: []string{"./..."}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 3 {
		t.Errorf("exit code = %d, want 3", code)
	}
}

func Test_run_FixApplication(t *testing.T) {
	dir := setupTestModule(
		t, map[string]string{
			"main.go": `package main

var bar = 1

func main() {
	_ = bar
}
`,
		},
	)
	t.Chdir(dir)

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "test",
					Analyzers: []*analysis.Analyzer{renameAnalyzer},
				},
			},
		},
	}

	code, err := run(cfg, &argument{Fix: true, Patterns: []string{"./..."}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	content, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(content), "bar") {
		t.Errorf("file still contains 'bar' after fix:\n%s", content)
	}
	if !strings.Contains(string(content), "baz") {
		t.Errorf("file does not contain 'baz' after fix:\n%s", content)
	}
}

func Test_run_PrintDiff(t *testing.T) {
	const src = `package main

var bar = 1

func main() {
	_ = bar
}
`
	dir := setupTestModule(
		t, map[string]string{
			"main.go": src,
		},
	)
	t.Chdir(dir)

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "test",
					Analyzers: []*analysis.Analyzer{renameAnalyzer},
				},
			},
		},
	}

	code, err := run(cfg, &argument{Fix: true, PrintDiff: true, Patterns: []string{"./..."}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}

	content, err := os.ReadFile(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != src {
		t.Errorf("file was modified in print-diff mode:\ngot:\n%s\nwant:\n%s", content, src)
	}
}

func Test_run_Sequential(t *testing.T) {
	dir := setupTestModule(
		t, map[string]string{
			"main.go": minimalMain,
		},
	)
	t.Chdir(dir)

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "test",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
				},
			},
		},
	}

	code, err := run(cfg, &argument{Sequential: true, Patterns: []string{"./..."}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func Test_run_PackageLoadError(t *testing.T) {
	dir := setupTestModule(
		t, map[string]string{
			"main.go": minimalMain,
		},
	)
	t.Chdir(dir)

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "test",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
				},
			},
		},
	}

	code, err := run(cfg, &argument{Patterns: []string{"./nonexistent"}})
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func Test_resolveSeverity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		policy   severity.DiagnosticPolicy
		category string
		want     severity.Severity
	}{
		{
			name:     "no rules returns DefaultSeverity",
			policy:   severity.DiagnosticPolicy{DefaultSeverity: severity.SeverityInfo},
			category: "anything",
			want:     severity.SeverityInfo,
		},
		{
			name: "unmatched category returns DefaultSeverity",
			policy: severity.DiagnosticPolicy{
				Rules:           []severity.CategoryRule{{Category: "other", Severity: severity.SeverityError}},
				DefaultSeverity: severity.SeverityWarn,
			},
			category: "nomatch",
			want:     severity.SeverityWarn,
		},
		{
			name: "exact match returns Error",
			policy: severity.DiagnosticPolicy{
				Rules: []severity.CategoryRule{{Category: "err", Severity: severity.SeverityError}},
			},
			category: "err",
			want:     severity.SeverityError,
		},
		{
			name: "exact match returns Warn",
			policy: severity.DiagnosticPolicy{
				Rules: []severity.CategoryRule{{Category: "warn", Severity: severity.SeverityWarn}},
			},
			category: "warn",
			want:     severity.SeverityWarn,
		},
		{
			name: "exact match returns Info",
			policy: severity.DiagnosticPolicy{
				Rules:           []severity.CategoryRule{{Category: "info", Severity: severity.SeverityInfo}},
				DefaultSeverity: severity.SeverityError,
			},
			category: "info",
			want:     severity.SeverityInfo,
		},
		{
			name: "first matching rule wins",
			policy: severity.DiagnosticPolicy{
				Rules: []severity.CategoryRule{
					{Category: "cat", Severity: severity.SeverityWarn},
					{Category: "cat", Severity: severity.SeverityError},
				},
			},
			category: "cat",
			want:     severity.SeverityWarn,
		},
		{
			name: "empty category matches",
			policy: severity.DiagnosticPolicy{
				Rules:           []severity.CategoryRule{{Category: "", Severity: severity.SeverityError}},
				DefaultSeverity: severity.SeverityInfo,
			},
			category: "",
			want:     severity.SeverityError,
		},
		{
			name:     "zero value Policy returns SeverityInfo",
			policy:   severity.DiagnosticPolicy{},
			category: "anything",
			want:     severity.SeverityInfo,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				t.Parallel()
				got := resolveSeverity(tt.category, tt.policy)
				if got != tt.want {
					t.Errorf("resolveSeverity(%q) = %d, want %d", tt.category, got, tt.want)
				}
			},
		)
	}
}
