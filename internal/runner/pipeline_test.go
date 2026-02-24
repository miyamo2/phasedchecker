package runner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/miyamo2/phasedchecker/internal/severity"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
)

// setupTestModule creates a temporary Go module directory with the given files.
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

// warnAnalyzer reports a diagnostic with category "warn" for every declaration.
var warnAnalyzer = &analysis.Analyzer{
	Name: "warnanalyzer",
	Doc:  "reports a warning for every declaration",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			for _, decl := range f.Decls {
				pass.Report(analysis.Diagnostic{
					Pos:      decl.Pos(),
					Message:  "function warning",
					Category: "warn",
				})
			}
		}
		return nil, nil
	},
}

// errorAnalyzer reports a diagnostic with category "error" for every declaration.
var errorAnalyzer = &analysis.Analyzer{
	Name: "erroranalyzer",
	Doc:  "reports an error diagnostic for every declaration",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			for _, decl := range f.Decls {
				pass.Report(analysis.Diagnostic{
					Pos:      decl.Pos(),
					Message:  "error issue",
					Category: "error",
				})
			}
		}
		return nil, nil
	},
}

// criticalAnalyzer reports a diagnostic with category "critical" for every declaration.
var criticalAnalyzer = &analysis.Analyzer{
	Name: "criticalanalyzer",
	Doc:  "reports a critical diagnostic for every declaration",
	Run: func(pass *analysis.Pass) (any, error) {
		for _, f := range pass.Files {
			for _, decl := range f.Decls {
				pass.Report(analysis.Diagnostic{
					Pos:      decl.Pos(),
					Message:  "critical issue",
					Category: "critical",
				})
			}
		}
		return nil, nil
	},
}

// noopAnalyzer does nothing.
var noopAnalyzer = &analysis.Analyzer{
	Name: "noopanalyzer",
	Doc:  "does nothing",
	Run: func(pass *analysis.Pass) (any, error) {
		return nil, nil
	},
}

// criticalPolicy maps category "critical" to SeverityCritical; everything else is Info.
var criticalPolicy = severity.DiagnosticPolicy{
	Rules: []severity.CategoryRule{
		{Category: "critical", Severity: severity.SeverityCritical},
	},
	DefaultSeverity: severity.SeverityInfo,
}

// warnPolicy maps category "warn" to SeverityWarn; everything else is Info.
var warnPolicy = severity.DiagnosticPolicy{
	Rules: []severity.CategoryRule{
		{Category: "warn", Severity: severity.SeverityWarn},
	},
	DefaultSeverity: severity.SeverityInfo,
}

// errorPolicy maps category "error" to SeverityError; everything else is Info.
var errorPolicy = severity.DiagnosticPolicy{
	Rules: []severity.CategoryRule{
		{Category: "error", Severity: severity.SeverityError},
	},
	DefaultSeverity: severity.SeverityInfo,
}

func TestRunPipeline_EmptyPipeline(t *testing.T) {
	t.Parallel()
	cfg := Config{
		Pipeline: Pipeline{Phases: nil},
	}
	var results []*PhaseResult
	for result, err := range RunPipeline(cfg, nil, nil) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		results = append(results, result)
	}
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestRunPipeline_SinglePhase(t *testing.T) {
	t.Parallel()
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})
	pkgs, err := LoadPackages(dir, false, []string{"./..."})
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "phase1",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
				},
			},
		},
	}

	var results []*PhaseResult
	for result, err := range RunPipeline(cfg, pkgs, nil) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		results = append(results, result)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Phase != "phase1" {
		t.Errorf("expected phase name %q, got %q", "phase1", results[0].Phase)
	}
	if results[0].Graph == nil {
		t.Errorf("expected non-nil Graph")
	}
}

func TestRunPipeline_CriticalAbortsPipeline(t *testing.T) {
	t.Parallel()
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})
	pkgs, err := LoadPackages(dir, false, []string{"./..."})
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	afterPhaseCalled := false
	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "critical-phase",
					Analyzers: []*analysis.Analyzer{criticalAnalyzer},
					AfterPhase: func(graph *checker.Graph) error {
						afterPhaseCalled = true
						return nil
					},
				},
				{
					Name:      "should-not-run",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
				},
			},
		},
		DiagnosticPolicy: criticalPolicy,
	}

	var results []*PhaseResult
	var pipelineErr error
	for result, err := range RunPipeline(cfg, pkgs, nil) {
		if result != nil {
			results = append(results, result)
		}
		if err != nil {
			pipelineErr = err
			break
		}
	}
	if pipelineErr == nil {
		t.Fatal("expected error for critical diagnostic, got nil")
	}
	if !strings.Contains(pipelineErr.Error(), "critical diagnostic") {
		t.Errorf("error should mention critical diagnostic, got: %v", pipelineErr)
	}
	// Critical yields both result and error.
	if len(results) != 1 {
		t.Fatalf("expected 1 result (critical phase), got %d", len(results))
	}
	if results[0].Phase != "critical-phase" {
		t.Errorf("expected phase %q, got %q", "critical-phase", results[0].Phase)
	}
	if results[0].HasError {
		t.Errorf("expected HasError=false for critical phase")
	}
	if afterPhaseCalled {
		t.Errorf("AfterPhase should NOT be called when Critical is detected")
	}
}

func TestRunPipeline_AfterPhaseError(t *testing.T) {
	t.Parallel()
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})
	pkgs, err := LoadPackages(dir, false, []string{"./..."})
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "failing-phase",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
					AfterPhase: func(graph *checker.Graph) error {
						return errors.New("after-phase failed")
					},
				},
				{
					Name:      "should-not-run",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
				},
			},
		},
	}

	var results []*PhaseResult
	var pipelineErr error
	for result, err := range RunPipeline(cfg, pkgs, nil) {
		if err != nil {
			pipelineErr = err
			break
		}
		results = append(results, result)
	}
	// AfterPhase runs before yield; on error the phase result is NOT yielded.
	if len(results) != 0 {
		t.Fatalf("expected 0 results (AfterPhase error prevents yield), got %d", len(results))
	}
	if pipelineErr == nil {
		t.Fatal("expected error from AfterPhase, got nil")
	}
	if !strings.Contains(pipelineErr.Error(), "after-phase callback") {
		t.Errorf("error should mention after-phase callback, got: %v", pipelineErr)
	}
	if !strings.Contains(pipelineErr.Error(), "after-phase failed") {
		t.Errorf("error should contain original message, got: %v", pipelineErr)
	}
}

func TestRunPipeline_MultiPhaseOrdering(t *testing.T) {
	t.Parallel()
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})
	pkgs, err := LoadPackages(dir, false, []string{"./..."})
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	var order []string
	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "first",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
					AfterPhase: func(graph *checker.Graph) error {
						order = append(order, "first")
						return nil
					},
				},
				{
					Name:      "second",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
					AfterPhase: func(graph *checker.Graph) error {
						order = append(order, "second")
						return nil
					},
				},
				{
					Name:      "third",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
				},
			},
		},
	}

	var results []*PhaseResult
	for result, err := range RunPipeline(cfg, pkgs, nil) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		results = append(results, result)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, name := range []string{"first", "second", "third"} {
		if results[i].Phase != name {
			t.Errorf("results[%d].Phase = %q, want %q", i, results[i].Phase, name)
		}
	}
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("AfterPhase call order = %v, want [first second]", order)
	}
}

func TestRunPipeline_DiagnosticsWithoutCritical(t *testing.T) {
	t.Parallel()
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})
	pkgs, err := LoadPackages(dir, false, []string{"./..."})
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "warn-phase",
					Analyzers: []*analysis.Analyzer{warnAnalyzer},
				},
			},
		},
		DiagnosticPolicy: warnPolicy,
	}

	var results []*PhaseResult
	for result, err := range RunPipeline(cfg, pkgs, nil) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		results = append(results, result)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].HasWarn {
		t.Errorf("expected HasWarn=true for warn diagnostics")
	}
}

func TestRunPipeline_ErrorDiagnostics(t *testing.T) {
	t.Parallel()
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})
	pkgs, err := LoadPackages(dir, false, []string{"./..."})
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "error-phase",
					Analyzers: []*analysis.Analyzer{errorAnalyzer},
				},
			},
		},
		DiagnosticPolicy: errorPolicy,
	}

	var results []*PhaseResult
	for result, err := range RunPipeline(cfg, pkgs, nil) {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		results = append(results, result)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].HasError {
		t.Errorf("expected HasError=true for error diagnostics")
	}
}

func TestRunPipeline_CriticalInSecondPhase(t *testing.T) {
	t.Parallel()
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})
	pkgs, err := LoadPackages(dir, false, []string{"./..."})
	if err != nil {
		t.Fatalf("LoadPackages: %v", err)
	}

	firstAfterCalled := false
	secondAfterCalled := false
	cfg := Config{
		Pipeline: Pipeline{
			Phases: []Phase{
				{
					Name:      "first",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
					AfterPhase: func(graph *checker.Graph) error {
						firstAfterCalled = true
						return nil
					},
				},
				{
					Name:      "critical",
					Analyzers: []*analysis.Analyzer{criticalAnalyzer},
					AfterPhase: func(graph *checker.Graph) error {
						secondAfterCalled = true
						return nil
					},
				},
				{
					Name:      "third",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
				},
			},
		},
		DiagnosticPolicy: criticalPolicy,
	}

	var results []*PhaseResult
	var pipelineErr error
	for result, err := range RunPipeline(cfg, pkgs, nil) {
		if result != nil {
			results = append(results, result)
		}
		if err != nil {
			pipelineErr = err
			break
		}
	}
	// First phase completes normally, second yields critical with result.
	if len(results) != 2 {
		t.Fatalf("expected 2 results (first + critical), got %d", len(results))
	}
	if results[0].Phase != "first" {
		t.Errorf("results[0].Phase = %q, want %q", results[0].Phase, "first")
	}
	if results[1].Phase != "critical" {
		t.Errorf("results[1].Phase = %q, want %q", results[1].Phase, "critical")
	}
	// Second phase yields critical error.
	if pipelineErr == nil {
		t.Fatal("expected error for critical diagnostic in second phase, got nil")
	}
	if !strings.Contains(pipelineErr.Error(), "critical diagnostic") {
		t.Errorf("error should mention critical diagnostic, got: %v", pipelineErr)
	}
	if !firstAfterCalled {
		t.Errorf("first phase AfterPhase should have been called")
	}
	if secondAfterCalled {
		t.Errorf("critical phase AfterPhase should NOT have been called")
	}
}

func TestLoadPackages_Success(t *testing.T) {
	t.Parallel()
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})

	pkgs, err := LoadPackages(dir, false, []string{"./..."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("expected at least one package")
	}
}

func TestLoadPackages_LoadErrors(t *testing.T) {
	t.Parallel()
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {
	// This is a syntax error: missing closing brace
`,
	})

	_, err := LoadPackages(dir, false, []string{"./..."})
	if err == nil {
		t.Fatal("expected error for syntax error in Go file, got nil")
	}
	if !strings.Contains(err.Error(), "package loading errors") {
		t.Errorf("expected 'package loading errors' in message, got: %v", err)
	}
}

func TestLoadPackages_EmptyDir(t *testing.T) {
	// When dir is empty, LoadPackages should not set cfg.Dir and rely on the CWD.
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
	})
	t.Chdir(dir)

	pkgs, err := LoadPackages("", false, []string{"./..."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatal("expected at least one package")
	}
}

func TestLoadPackages_WithTestFlag(t *testing.T) {
	t.Parallel()
	dir := setupTestModule(t, map[string]string{
		"main.go": `package main

func main() {}
`,
		"main_test.go": `package main

import "testing"

func TestFoo(t *testing.T) {}
`,
	})

	pkgs, err := LoadPackages(dir, true, []string{"./..."})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// With Tests: true, packages.Load returns additional test packages.
	if len(pkgs) == 0 {
		t.Fatal("expected at least one package with test flag")
	}
}