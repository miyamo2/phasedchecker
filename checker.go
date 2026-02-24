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
	"fmt"
	"io"
	"log"
	"os"
	"sort"
	"time"

	"github.com/miyamo2/phasedchecker/internal/arg"
	"github.com/miyamo2/phasedchecker/internal/runner"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"

	"github.com/miyamo2/phasedchecker/internal/x/tools/driverutil"
)

// Phase represents a group of analyzers that run together.
// All analyzers in a phase complete on ALL packages before the next phase starts.
type Phase = runner.Phase

// Pipeline defines an ordered sequence of phases.
type Pipeline = runner.Pipeline

// Config controls the behavior of the checker.
type Config = runner.Config

// Main executes the full pipeline: load packages, run phases, apply fixes, and returns the exit code.
//
// Analyzer errors within a phase set exit code 1 but do not halt the pipeline;
// subsequent phases still execute. Only SeverityCritical diagnostics cause immediate termination.
func Main(cfg Config) {
	args, err := arg.ParseArgs(os.Args[0], os.Args[1:])
	if err != nil {
		log.Printf("Error parsing arguments: %v", err)
	}
	exitCode, err := run(cfg, args)
	if err != nil {
		log.Printf("Error: %v", err)
	}
	os.Exit(exitCode)
}

// run is the internal entry point that accepts pre-parsed arguments.
func run(cfg Config, args *arg.Argument) (int, error) {
	if args == nil {
		return 1, fmt.Errorf("args cannot be nil")
	}
	if len(cfg.Pipeline.Phases) == 0 {
		return 1, fmt.Errorf("pipeline has no phases")
	}

	if args.Dbg('v') {
		log.SetPrefix("")
		log.SetFlags(log.Lmicroseconds)
		log.Printf("load %s", args.Patterns)
	}

	pkgs, err := runner.LoadPackages("", args.Test, args.Patterns)
	if err != nil {
		return 1, err
	}

	var factLog io.Writer
	if args.Dbg('f') {
		factLog = os.Stderr
	}

	opts := &checker.Options{
		SanityCheck: args.Dbg('s'),
		Sequential:  args.Dbg('p'),
		FactLog:     factLog,
	}

	var (
		results  = make([]*runner.PhaseResult, 0, len(cfg.Pipeline.Phases))
		hasError bool
		hasWarn  bool
	)

	for result, err := range runner.RunPipeline(cfg, pkgs, opts) {
		if err != nil {
			return 1, err
		}
		results = append(results, result)
		switch {
		case result.HasError:
			hasError = true
		case result.HasWarn:
			hasWarn = true
		}
	}

	if args.Dbg('t') {
		if !args.Dbg('p') {
			log.Println("Warning: times are mostly GC/scheduler noise; use -debug=tp to disable parallelism")
		}
		var list []*checker.Action
		var total time.Duration
		for _, r := range results {
			for act := range r.Graph.All() {
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
			for act := range r.Graph.All() {
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
			if err := applyFixes(r.Graph, args.PrintDiff, args.Dbg('v')); err != nil {
				return 1, fmt.Errorf("applying fixes for phase %q: %w", r.Phase, err)
			}
		} else {
			_ = r.Graph.PrintText(os.Stderr, -1)
		}
	}

	if hasError {
		return 1, nil
	}
	if !args.Fix && hasWarn {
		return 3, nil
	}
	return 0, nil
}
