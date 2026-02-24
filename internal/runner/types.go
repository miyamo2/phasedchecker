package runner

import (
	"github.com/miyamo2/phasedchecker/internal/severity"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
)

// Phase represents a group of analyzers that run together.
// All analyzers in a phase complete on ALL packages before the next phase starts.
type Phase struct {
	// Name is a human-readable label for logging/debugging.
	Name string
	// Analyzers are the analysis passes to run in this phase.
	// Analyzers within a phase may run concurrently across packages.
	Analyzers []*analysis.Analyzer
	// AfterPhase is an optional callback invoked after all analyzers in this phase
	// have completed on all packages and fixes have been applied.
	// It receives the resulting Graph, enabling callers to extract per-package
	// Action.Result values and aggregate them for consumption by subsequent phases.
	AfterPhase func(graph *checker.Graph) error
}

// Pipeline defines an ordered sequence of phases.
type Pipeline struct {
	// Phases are executed in order. Each phase completes fully before the next begins.
	Phases []Phase
}

// Config controls the behavior of the checker.
type Config struct {
	// Pipeline defines the phase-ordered analyzer execution plan.
	Pipeline Pipeline
	// DiagnosticPolicy determines how diagnostic categories map to severity levels and exit codes.
	DiagnosticPolicy severity.DiagnosticPolicy
}

// PhaseResult holds the result of executing a single phase.
type PhaseResult struct {
	// Phase is the name of the executed phase.
	Phase string
	// Graph is the analysis result graph produced by [checker.Analyze] for this phase.
	Graph *checker.Graph
	// HasError is true if any diagnostics with severity Error were detected in this phase.
	HasError bool
	// HasWarn is true if any diagnostics with severity Warn were detected in this phase.
	HasWarn bool
}
