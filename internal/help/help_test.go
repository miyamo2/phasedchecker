package help

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/miyamo2/phasedchecker/internal/runner"
	"golang.org/x/tools/go/analysis"
)

// testAnalyzerWithFlags creates an analyzer that has custom flags.
func testAnalyzerWithFlags() *analysis.Analyzer {
	a := &analysis.Analyzer{
		Name: "flagged",
		Doc:  "analyzer with flags\n\nThis analyzer accepts custom flags for configuration.",
	}
	a.Flags.Init("flagged", flag.ContinueOnError)
	a.Flags.String("threshold", "10", "set the warning threshold")
	a.Flags.Bool("strict", false, "enable strict mode")
	return a
}

var (
	simpleAnalyzer = &analysis.Analyzer{
		Name: "simple",
		Doc:  "a simple analyzer for testing",
	}
	multiDocAnalyzer = &analysis.Analyzer{
		Name: "multidoc",
		Doc:  "multi-paragraph doc analyzer\n\nThis is the second paragraph with more detail.\n\nAnd a third paragraph.",
	}
	anotherAnalyzer = &analysis.Analyzer{
		Name: "another",
		Doc:  "another test analyzer",
	}
)

func testPipeline() runner.Pipeline {
	return runner.Pipeline{
		Phases: []runner.Phase{
			{
				Name:      "lint",
				Analyzers: []*analysis.Analyzer{simpleAnalyzer, multiDocAnalyzer},
			},
			{
				Name:      "security",
				Analyzers: []*analysis.Analyzer{anotherAnalyzer, testAnalyzerWithFlags()},
			},
		},
	}
}

func TestHelp_Overview(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pipeline := testPipeline()

	Help(&buf, "mytool", pipeline, nil)

	output := buf.String()

	// Verify progname appears in description.
	if !strings.Contains(output, "mytool is a tool for phase-based static analysis") {
		t.Error("overview should contain progname in description")
	}
	if !strings.Contains(output, "mytool runs analyzers in sequential phases") {
		t.Error("overview should describe sequential phase execution")
	}

	// Verify phases section.
	if !strings.Contains(output, "Phases:") {
		t.Error("overview should contain Phases header")
	}
	if !strings.Contains(output, "lint:") {
		t.Error("overview should list lint phase")
	}
	if !strings.Contains(output, "security:") {
		t.Error("overview should list security phase")
	}

	// Verify analyzers appear under their phases.
	if !strings.Contains(output, "simple") {
		t.Error("overview should list simple analyzer")
	}
	if !strings.Contains(output, "multidoc") {
		t.Error("overview should list multidoc analyzer")
	}
	if !strings.Contains(output, "another") {
		t.Error("overview should list another analyzer")
	}
	if !strings.Contains(output, "flagged") {
		t.Error("overview should list flagged analyzer")
	}

	// Verify core flags section.
	if !strings.Contains(output, "Core flags:") {
		t.Error("overview should contain Core flags header")
	}
	for _, f := range []string{"-fix", "-diff", "-json", "-test", "-debug"} {
		if !strings.Contains(output, f) {
			t.Errorf("overview should list %s flag", f)
		}
	}

	// Verify footer.
	if !strings.Contains(output, "mytool help phase <name>") {
		t.Error("overview should contain help phase footer")
	}
	if !strings.Contains(output, "mytool help analyzer <name>") {
		t.Error("overview should contain help analyzer footer")
	}
}

func TestHelp_Overview_PhaseOrdering(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pipeline := testPipeline()

	Help(&buf, "mytool", pipeline, nil)

	output := buf.String()

	// Verify phase order: lint appears before security.
	lintIdx := strings.Index(output, "lint:")
	secIdx := strings.Index(output, "security:")
	if lintIdx < 0 || secIdx < 0 {
		t.Fatal("both lint and security phases should appear")
	}
	if lintIdx >= secIdx {
		t.Error("lint phase should appear before security phase")
	}

	// Verify analyzer order within lint phase: simple before multidoc.
	simpleIdx := strings.Index(output, "simple")
	multidocIdx := strings.Index(output, "multidoc")
	if simpleIdx >= multidocIdx {
		t.Error("simple analyzer should appear before multidoc (definition order)")
	}
}

func TestHelp_Overview_AnalyzerDocFirstParagraph(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pipeline := testPipeline()

	Help(&buf, "mytool", pipeline, nil)

	output := buf.String()

	// The overview should show only the first paragraph of the doc.
	if !strings.Contains(output, "multi-paragraph doc analyzer") {
		t.Error("overview should contain first paragraph of multidoc doc")
	}
	// The second paragraph should NOT appear in the overview.
	if strings.Contains(output, "second paragraph") {
		t.Error("overview should NOT contain subsequent paragraphs of doc")
	}
}

func TestHelp_PhaseDetail(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pipeline := testPipeline()

	Help(&buf, "mytool", pipeline, []string{"phase", "lint"})

	output := buf.String()

	if !strings.Contains(output, `Phase "lint"`) {
		t.Error("phase detail should contain phase name in quotes")
	}
	if !strings.Contains(output, "Analyzers:") {
		t.Error("phase detail should contain Analyzers header")
	}
	if !strings.Contains(output, "simple") {
		t.Error("phase detail should list simple analyzer")
	}
	if !strings.Contains(output, "multidoc") {
		t.Error("phase detail should list multidoc analyzer")
	}
	// Should NOT contain analyzers from other phases.
	if strings.Contains(output, "another") {
		t.Error("phase detail should NOT list analyzers from other phases")
	}
}

func TestHelp_PhaseDetail_SecurityPhase(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pipeline := testPipeline()

	Help(&buf, "mytool", pipeline, []string{"phase", "security"})

	output := buf.String()

	if !strings.Contains(output, `Phase "security"`) {
		t.Error("phase detail should contain security phase name")
	}
	if !strings.Contains(output, "another") {
		t.Error("security phase should list another analyzer")
	}
	if !strings.Contains(output, "flagged") {
		t.Error("security phase should list flagged analyzer")
	}
}

func TestHelp_AnalyzerDetail_WithFlags(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pipeline := testPipeline()

	Help(&buf, "mytool", pipeline, []string{"analyzer", "flagged"})

	output := buf.String()

	// Title line.
	if !strings.Contains(output, "flagged: analyzer with flags") {
		t.Error("analyzer detail should contain name: first paragraph")
	}

	// Analyzer flags section.
	if !strings.Contains(output, "Analyzer flags:") {
		t.Error("analyzer detail should contain Analyzer flags header when flags exist")
	}
	if !strings.Contains(output, "threshold") {
		t.Error("analyzer detail should list threshold flag")
	}
	if !strings.Contains(output, "strict") {
		t.Error("analyzer detail should list strict flag")
	}

	// Remaining doc paragraphs.
	if !strings.Contains(output, "This analyzer accepts custom flags for configuration.") {
		t.Error("analyzer detail should contain remaining doc paragraphs")
	}
}

func TestHelp_AnalyzerDetail_WithoutFlags(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pipeline := testPipeline()

	Help(&buf, "mytool", pipeline, []string{"analyzer", "simple"})

	output := buf.String()

	// Title line.
	if !strings.Contains(output, "simple: a simple analyzer for testing") {
		t.Error("analyzer detail should contain name: doc")
	}

	// No flags section.
	if strings.Contains(output, "Analyzer flags:") {
		t.Error("analyzer detail should NOT contain Analyzer flags header when no flags")
	}
}

func TestHelp_AnalyzerDetail_MultiParagraphDoc(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pipeline := testPipeline()

	Help(&buf, "mytool", pipeline, []string{"analyzer", "multidoc"})

	output := buf.String()

	// Title should be the first paragraph.
	if !strings.Contains(output, "multidoc: multi-paragraph doc analyzer") {
		t.Error("analyzer detail should show first paragraph as title")
	}

	// Remaining paragraphs should appear.
	if !strings.Contains(output, "This is the second paragraph with more detail.") {
		t.Error("analyzer detail should show second paragraph")
	}
	if !strings.Contains(output, "And a third paragraph.") {
		t.Error("analyzer detail should show third paragraph")
	}
}

func TestHelp_AnalyzerDetail_FlagPrefix(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	pipeline := testPipeline()

	Help(&buf, "mytool", pipeline, []string{"analyzer", "flagged"})

	output := buf.String()

	// Flags should be prefixed with analyzer name (multichecker convention).
	if !strings.Contains(output, "flagged.threshold") {
		t.Error("flags should be prefixed with analyzer name")
	}
	if !strings.Contains(output, "flagged.strict") {
		t.Error("flags should be prefixed with analyzer name")
	}
}
