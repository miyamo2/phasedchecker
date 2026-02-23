// Package phasedchecker implements a phase-based analysis driver.
//
// Unlike multichecker, which runs all analyzers per-package with no phase ordering,
// this checker supports executing analyzers in sequential phases. Each phase completes
// for ALL packages before the next phase starts.
//
// The checker uses the public golang.org/x/tools/go/analysis/checker.Analyze() API
// to run analyzers within each phase.
package phasedchecker

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"time"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/miyamo2/phasedchecker/internal/x/tools/driverutil"
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
	DiagnosticPolicy DiagnosticPolicy
}

// Run executes the full pipeline: load packages, run phases, apply fixes, and returns the exit code.
//
// Analyzer errors within a phase set exit code 1 but do not halt the pipeline;
// subsequent phases still execute. Only SeverityCritical diagnostics cause immediate termination.
func Run(cfg Config) (int, error) {
	args, err := parseArgs(os.Args[0], os.Args[1:])
	if err != nil {
		return 1, fmt.Errorf("parsing arguments: %w", err)
	}
	return run(cfg, args)
}

// run is the internal entry point that accepts pre-parsed arguments.
func run(cfg Config, args *argument) (int, error) {
	if args == nil {
		return 1, fmt.Errorf("args cannot be nil")
	}
	if len(cfg.Pipeline.Phases) == 0 {
		return 1, fmt.Errorf("pipeline has no phases")
	}

	if args.dbg('v') {
		log.SetPrefix("")
		log.SetFlags(log.Lmicroseconds)
		log.Printf("load %s", args.Patterns)
	}

	pkgs, err := packages.Load(&packages.Config{Mode: packages.LoadAllSyntax}, args.Patterns...)
	if err != nil {
		return 1, fmt.Errorf("loading packages: %w", err)
	}

	var loadErrors []error
	packages.Visit(
		pkgs, nil, func(pkg *packages.Package) {
			for _, err := range pkg.Errors {
				loadErrors = append(loadErrors, err)
			}
		},
	)
	if len(loadErrors) > 0 {
		return 1, fmt.Errorf("package loading errors: %w", errors.Join(loadErrors...))
	}

	var factLog io.Writer
	if args.dbg('f') {
		factLog = os.Stderr
	}

	type phaseResult struct {
		name  string
		graph *checker.Graph
	}
	var results []phaseResult

	hasError := false
	hasDiagnostics := false
	for _, phase := range cfg.Pipeline.Phases {
		if args.dbg('v') {
			log.Printf("phase %q: building graph of analysis passes", phase.Name)
		}
		graph, err := checker.Analyze(
			phase.Analyzers, pkgs, &checker.Options{
				SanityCheck: args.dbg('s'),
				Sequential:  args.dbg('p'),
				FactLog:     factLog,
			},
		)
		if err != nil {
			return 1, fmt.Errorf("phase %q: %w", phase.Name, err)
		}

		for act := range graph.All() {
			if act.Err != nil {
				hasError = true
				continue
			}
			if !act.IsRoot {
				continue
			}
			for _, d := range act.Diagnostics {
				sv := resolveSeverity(d.Category, cfg.DiagnosticPolicy)
				switch sv {
				case SeverityCritical:
					return 1, fmt.Errorf("critical diagnostic: %s", d.Message)
				case SeverityError:
					hasError = true
				case SeverityWarn:
					hasDiagnostics = true
				}
			}
		}

		results = append(results, phaseResult{phase.Name, graph})

		if phase.AfterPhase != nil {
			if err := phase.AfterPhase(graph); err != nil {
				return 1, fmt.Errorf("phase %q after-phase callback: %w", phase.Name, err)
			}
		}
	}

	if args.dbg('t') {
		if !args.dbg('p') {
			log.Println("Warning: times are mostly GC/scheduler noise; use -debug=tp to disable parallelism")
		}
		var list []*checker.Action
		var total time.Duration
		for _, r := range results {
			for act := range r.graph.All() {
				list = append(list, act)
				total += act.Duration
			}
		}
		sort.Slice(
			list, func(i, j int) bool {
				return list[i].Duration > list[j].Duration
			},
		)
		var sum time.Duration
		for _, act := range list {
			fmt.Fprintf(os.Stderr, "%s\t%s\n", act.Duration, act)
			sum += act.Duration
			if sum >= total*9/10 {
				break
			}
		}
		if total > sum {
			fmt.Fprintf(os.Stderr, "%s\tall others\n", total-sum)
		}
	}

	if !args.Fix && args.JSON {
		tree := make(driverutil.JSONTree)
		for _, r := range results {
			for act := range r.graph.All() {
				var diags []analysis.Diagnostic
				if act.IsRoot {
					diags = act.Diagnostics
				}
				tree.Add(act.Package.Fset, act.Package.ID, act.Analyzer.Name, diags, act.Err)
			}
		}
		if err := tree.Print(os.Stdout); err != nil {
			return 1, fmt.Errorf("printing JSON diagnostics: %w", err)
		}
		return 0, nil
	}

	for _, r := range results {
		if args.Fix {
			if err := applyFixes(r.graph, args.PrintDiff, args.dbg('v')); err != nil {
				return 1, fmt.Errorf("applying fixes for phase %q: %w", r.name, err)
			}
		} else {
			_ = r.graph.PrintText(os.Stderr, -1)
		}
	}

	if hasError {
		return 1, nil
	}
	if !args.Fix && hasDiagnostics {
		return 3, nil
	}
	return 0, nil
}

// resolveSeverity finds the severity for a given category.
func resolveSeverity(category string, policy DiagnosticPolicy) Severity {
	for _, rule := range policy.Rules {
		if rule.Category == category {
			return rule.Severity
		}
	}
	return policy.DefaultSeverity
}
