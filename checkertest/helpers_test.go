package checkertest

import (
	"fmt"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/miyamo2/phasedchecker/config"
	"github.com/miyamo2/phasedchecker/internal/x/tools/diff"
	"golang.org/x/tools/go/analysis"
	gochecker "golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/txtar"
)

// --- hasEdits tests ---

func TestHasEdits(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		diag []analysis.Diagnostic
		want bool
	}{
		{
			name: "nil input",
			diag: nil,
			want: false,
		},
		{
			name: "empty slice",
			diag: []analysis.Diagnostic{},
			want: false,
		},
		{
			name: "diagnostic without fixes",
			diag: []analysis.Diagnostic{
				{Message: "no fix"},
			},
			want: false,
		},
		{
			name: "fix without TextEdits",
			diag: []analysis.Diagnostic{
				{
					Message: "has fix",
					SuggestedFixes: []analysis.SuggestedFix{
						{Message: "fix", TextEdits: nil},
					},
				},
			},
			want: false,
		},
		{
			name: "fix with TextEdits",
			diag: []analysis.Diagnostic{
				{
					Message: "has fix",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "fix",
							TextEdits: []analysis.TextEdit{
								{NewText: []byte("new")},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "second diagnostic has edits",
			diag: []analysis.Diagnostic{
				{Message: "no fix"},
				{
					Message: "has fix",
					SuggestedFixes: []analysis.SuggestedFix{
						{
							Message: "fix",
							TextEdits: []analysis.TextEdit{
								{NewText: []byte("new")},
							},
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := hasEdits(tt.diag); got != tt.want {
				t.Errorf("hasEdits() = %v, want %v", got, tt.want)
			}
		})
	}
}

// --- checkDiagnostics tests ---

func TestCheckDiagnostics(t *testing.T) {
	t.Parallel()

	posn := token.Position{Filename: "test.go", Line: 10}
	key := expectKey{file: "test.go", line: 10}

	t.Run("match removes last expectation and deletes key", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		wants := map[expectKey][]*expectation{
			key: {{rx: regexp.MustCompile("hello")}},
		}

		checkDiagnostics(mt, wants, posn, "hello world")

		if len(mt.errors) != 0 {
			t.Errorf("unexpected errors: %v", mt.errors)
		}
		if _, ok := wants[key]; ok {
			t.Errorf("key should have been deleted from wants map")
		}
	})

	t.Run("match removes one of multiple expectations", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		wants := map[expectKey][]*expectation{
			key: {
				{rx: regexp.MustCompile("first")},
				{rx: regexp.MustCompile("second")},
			},
		}

		checkDiagnostics(mt, wants, posn, "second message")

		if len(mt.errors) != 0 {
			t.Errorf("unexpected errors: %v", mt.errors)
		}
		remaining := wants[key]
		if len(remaining) != 1 {
			t.Fatalf("expected 1 remaining expectation, got %d", len(remaining))
		}
		if remaining[0].rx.String() != "first" {
			t.Errorf("remaining expectation = %q, want %q", remaining[0].rx.String(), "first")
		}
	})

	t.Run("unexpected diagnostic when no expectations at position", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		wants := map[expectKey][]*expectation{}

		checkDiagnostics(mt, wants, posn, "surprise")

		if len(mt.errors) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(mt.errors), mt.errors)
		}
		if !strings.Contains(mt.errors[0], "unexpected diagnostic") {
			t.Errorf("error = %q, want it to contain %q", mt.errors[0], "unexpected diagnostic")
		}
	})

	t.Run("no match when expectations exist but none match", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		wants := map[expectKey][]*expectation{
			key: {{rx: regexp.MustCompile("^abc$")}},
		}

		checkDiagnostics(mt, wants, posn, "xyz")

		if len(mt.errors) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(mt.errors), mt.errors)
		}
		if !strings.Contains(mt.errors[0], "does not match any of") {
			t.Errorf("error = %q, want it to contain %q", mt.errors[0], "does not match any of")
		}
		// expectations should remain
		if len(wants[key]) != 1 {
			t.Errorf("expectations should remain unchanged, got %d", len(wants[key]))
		}
	})
}

// --- reportUnmatched tests ---

func TestReportUnmatched(t *testing.T) {
	t.Parallel()

	t.Run("empty map produces no errors", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}

		reportUnmatched(mt, map[expectKey][]*expectation{})

		if len(mt.errors) != 0 {
			t.Errorf("expected no errors, got %d: %v", len(mt.errors), mt.errors)
		}
	})

	t.Run("one key one expectation produces one error", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		wants := map[expectKey][]*expectation{
			{file: "a.go", line: 1}: {{rx: regexp.MustCompile("pattern")}},
		}

		reportUnmatched(mt, wants)

		if len(mt.errors) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(mt.errors), mt.errors)
		}
		if !strings.Contains(mt.errors[0], "no diagnostic was reported matching") {
			t.Errorf("error = %q, want it to contain %q", mt.errors[0], "no diagnostic was reported matching")
		}
	})

	t.Run("multiple keys and expectations produce matching error count", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		wants := map[expectKey][]*expectation{
			{file: "a.go", line: 1}: {
				{rx: regexp.MustCompile("p1")},
				{rx: regexp.MustCompile("p2")},
			},
			{file: "b.go", line: 5}: {
				{rx: regexp.MustCompile("p3")},
			},
		}

		reportUnmatched(mt, wants)

		if len(mt.errors) != 3 {
			t.Errorf("expected 3 errors, got %d: %v", len(mt.errors), mt.errors)
		}
	})
}

// --- compareAllFixesGolden tests ---

func TestCompareAllFixesGolden(t *testing.T) {
	t.Parallel()

	const src = "package main\n\nvar bar = 1\n"
	barStart := strings.Index(src, "bar")
	barEnd := barStart + len("bar")
	pkg := types.NewPackage("example.com/test", "main")

	t.Run("single fix matches expected", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		ffe := &fileEditsInfo{
			pkg: pkg,
			byMsg: map[string][]diff.Edit{
				"rename": {{Start: barStart, End: barEnd, New: "baz"}},
			},
		}
		expected := []byte("package main\n\nvar baz = 1\n")

		compareAllFixesGolden(mt, "test.go", []byte(src), ffe, expected)

		if len(mt.errors) != 0 {
			t.Errorf("unexpected errors: %v", mt.errors)
		}
	})

	t.Run("single fix does not match expected", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		ffe := &fileEditsInfo{
			pkg: pkg,
			byMsg: map[string][]diff.Edit{
				"rename": {{Start: barStart, End: barEnd, New: "baz"}},
			},
		}
		expected := []byte("package main\n\nvar qux = 1\n")

		compareAllFixesGolden(mt, "test.go", []byte(src), ffe, expected)

		if len(mt.errors) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(mt.errors), mt.errors)
		}
		if !strings.Contains(mt.errors[0], "golden file mismatch") {
			t.Errorf("error = %q, want it to contain %q", mt.errors[0], "golden file mismatch")
		}
	})

	t.Run("conflicting edits", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		ffe := &fileEditsInfo{
			pkg: pkg,
			byMsg: map[string][]diff.Edit{
				"fix-a": {{Start: barStart, End: barEnd, New: "baz"}},
				"fix-b": {{Start: barStart, End: barEnd, New: "qux"}},
			},
		}

		compareAllFixesGolden(mt, "test.go", []byte(src), ffe, []byte("unused"))

		if len(mt.errors) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(mt.errors), mt.errors)
		}
		if !strings.Contains(mt.errors[0], "conflicting edits") {
			t.Errorf("error = %q, want it to contain %q", mt.errors[0], "conflicting edits")
		}
	})

	t.Run("multiple non-conflicting fixes merge successfully", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		// "var bar = 1\n" has "bar" at index barStart..barEnd
		// "= 1" has "1" at a later position; change it to "2"
		oneStart := strings.Index(src, "1")
		oneEnd := oneStart + 1
		ffe := &fileEditsInfo{
			pkg: pkg,
			byMsg: map[string][]diff.Edit{
				"fix-a": {{Start: barStart, End: barEnd, New: "baz"}},
				"fix-b": {{Start: oneStart, End: oneEnd, New: "2"}},
			},
		}
		expected := []byte("package main\n\nvar baz = 2\n")

		compareAllFixesGolden(mt, "test.go", []byte(src), ffe, expected)

		if len(mt.errors) != 0 {
			t.Errorf("unexpected errors: %v", mt.errors)
		}
	})

	t.Run("apply error with out-of-bounds edit", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		ffe := &fileEditsInfo{
			pkg: pkg,
			byMsg: map[string][]diff.Edit{
				"bad": {{Start: 9999, End: 10000, New: "x"}},
			},
		}

		compareAllFixesGolden(mt, "test.go", []byte(src), ffe, []byte("unused"))

		if len(mt.errors) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(mt.errors), mt.errors)
		}
		if !strings.Contains(mt.errors[0], "applying edits") {
			t.Errorf("error = %q, want it to contain %q", mt.errors[0], "applying edits")
		}
	})
}

// --- compareArchiveGolden tests ---

func TestCompareArchiveGolden(t *testing.T) {
	t.Parallel()

	const src = "package main\n\nvar bar = 1\n"
	barStart := strings.Index(src, "bar")
	barEnd := barStart + len("bar")
	pkg := types.NewPackage("example.com/test", "main")

	t.Run("section matches fix and content matches", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		ffe := &fileEditsInfo{
			pkg: pkg,
			byMsg: map[string][]diff.Edit{
				"rename bar to baz": {{Start: barStart, End: barEnd, New: "baz"}},
			},
		}
		ar := &txtar.Archive{
			Files: []txtar.File{
				{Name: "rename bar to baz", Data: []byte("package main\n\nvar baz = 1\n")},
			},
		}

		compareArchiveGolden(mt, "test.go", []byte(src), ffe, ar)

		if len(mt.errors) != 0 {
			t.Errorf("unexpected errors: %v", mt.errors)
		}
	})

	t.Run("section matches fix but content differs", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		ffe := &fileEditsInfo{
			pkg: pkg,
			byMsg: map[string][]diff.Edit{
				"rename bar to baz": {{Start: barStart, End: barEnd, New: "baz"}},
			},
		}
		ar := &txtar.Archive{
			Files: []txtar.File{
				{Name: "rename bar to baz", Data: []byte("package main\n\nvar qux = 1\n")},
			},
		}

		compareArchiveGolden(mt, "test.go", []byte(src), ffe, ar)

		if len(mt.errors) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(mt.errors), mt.errors)
		}
		if !strings.Contains(mt.errors[0], "mismatch") {
			t.Errorf("error = %q, want it to contain %q", mt.errors[0], "mismatch")
		}
	})

	t.Run("section name has no matching fix", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		ffe := &fileEditsInfo{
			pkg: pkg,
			byMsg: map[string][]diff.Edit{
				"rename bar to baz": {{Start: barStart, End: barEnd, New: "baz"}},
			},
		}
		ar := &txtar.Archive{
			Files: []txtar.File{
				{Name: "nonexistent fix", Data: []byte("whatever")},
			},
		}

		compareArchiveGolden(mt, "test.go", []byte(src), ffe, ar)

		if len(mt.errors) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(mt.errors), mt.errors)
		}
		if !strings.Contains(mt.errors[0], "has no matching fix") {
			t.Errorf("error = %q, want it to contain %q", mt.errors[0], "has no matching fix")
		}
	})

	t.Run("apply error with invalid edits", func(t *testing.T) {
		t.Parallel()
		mt := &mockT{}
		ffe := &fileEditsInfo{
			pkg: pkg,
			byMsg: map[string][]diff.Edit{
				"bad fix": {{Start: 9999, End: 10000, New: "x"}},
			},
		}
		ar := &txtar.Archive{
			Files: []txtar.File{
				{Name: "bad fix", Data: []byte("whatever")},
			},
		}

		compareArchiveGolden(mt, "test.go", []byte(src), ffe, ar)

		if len(mt.errors) != 1 {
			t.Fatalf("expected 1 error, got %d: %v", len(mt.errors), mt.errors)
		}
		if !strings.Contains(mt.errors[0], "applying edits for fix") {
			t.Errorf("error = %q, want it to contain %q", mt.errors[0], "applying edits for fix")
		}
	})
}

// --- runPipeline tests ---

func TestRunPipeline_AfterPhaseError(t *testing.T) {
	t.Parallel()

	noopAnalyzer := &analysis.Analyzer{
		Name: "noop",
		Doc:  "does nothing",
		Run: func(pass *analysis.Pass) (any, error) {
			return nil, nil
		},
	}

	dir := filepath.Join(testdataDir(), "basic")

	cfg := config.Config{
		Pipeline: config.Pipeline{
			Phases: []config.Phase{
				{
					Name:      "phase1",
					Analyzers: []*analysis.Analyzer{noopAnalyzer},
					AfterPhase: func(_ *gochecker.Graph) error {
						return fmt.Errorf("after-phase callback error")
					},
				},
			},
		},
	}

	mt := &mockT{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		runPipeline(mt, dir, cfg, false, []string{"./..."})
	}()
	<-done

	if len(mt.fatals) == 0 {
		t.Fatal("expected Fatalf to be called, but it was not")
	}
	if !strings.Contains(mt.fatals[0], "after-phase callback") {
		t.Errorf("fatal = %q, want containing %q", mt.fatals[0], "after-phase callback")
	}
}

// --- loadPackages tests ---

func TestLoadPackages_LoadErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gomod := "module example.com/broken\n\ngo 1.25\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644); err != nil {
		t.Fatal(err)
	}
	// Write a Go file with a syntax error.
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	mt := &mockT{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		loadPackages(mt, dir, []string{"./..."})
	}()
	<-done

	if len(mt.fatals) == 0 {
		t.Fatal("expected Fatalf to be called for load errors")
	}
	got := mt.fatals[0]
	if !strings.Contains(got, "package loading errors") && !strings.Contains(got, "loading packages") {
		t.Errorf("fatal = %q, want containing load error message", got)
	}
}

