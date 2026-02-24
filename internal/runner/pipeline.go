package runner

import (
	"errors"
	"fmt"
	"iter"

	"github.com/miyamo2/phasedchecker/internal/severity"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"
)

// LoadPackages loads Go packages from the given directory using the specified patterns.
// When dir is non-empty it is set as the working directory for the package loader;
// production callers pass "" so that the current working directory is used,
// while checkertest passes the testdata directory.
func LoadPackages(dir string, test bool, patterns []string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode:  packages.LoadSyntax | packages.NeedModule,
		Tests: test,
	}
	if dir != "" {
		cfg.Dir = dir
	}

	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, fmt.Errorf("loading packages: %w", err)
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
		return nil, fmt.Errorf("package loading errors: %w", errors.Join(loadErrors...))
	}

	return pkgs, nil
}

// RunPipeline returns an iterator that executes each phase in the config's
// pipeline sequentially, yielding a [*PhaseResult] per phase.
//
// For each phase it runs [checker.Analyze], then scans root diagnostics using
// the config's [severity.DiagnosticPolicy]. Diagnostics classified as
// [severity.SeverityError] or [severity.SeverityWarn] set the corresponding
// HasError/HasWarn flags on the [PhaseResult].
//
// If a [severity.SeverityCritical] diagnostic is found, the result and an
// error are yielded together and iteration stops immediately (the AfterPhase
// callback is NOT invoked and subsequent phases are skipped).
//
// For non-critical phases the AfterPhase callback runs before the result is
// yielded. If AfterPhase returns an error, a nil result and the wrapped error
// are yielded and iteration stops (the phase result is NOT yielded).
// If the caller breaks out of the loop, the AfterPhase callback for the
// current phase is skipped.
//
// The opts parameter is passed through to [checker.Analyze]; production callers
// set SanityCheck/Sequential/FactLog while checkertest passes nil.
func RunPipeline(cfg Config, pkgs []*packages.Package, opts *checker.Options) iter.Seq2[*PhaseResult, error] {
	return func(yield func(*PhaseResult, error) bool) {
		for _, phase := range cfg.Pipeline.Phases {
			graph, err := checker.Analyze(phase.Analyzers, pkgs, opts)
			if err != nil {
				yield(nil, fmt.Errorf("phase %q: %w", phase.Name, err))
				return
			}
			result := &PhaseResult{Phase: phase.Name, Graph: graph}
			for act := range graph.All() {
				if act.Err != nil {
					result.HasError = true
					continue
				}
				if !act.IsRoot {
					continue
				}
				for _, d := range act.Diagnostics {
					sv := ResolveSeverity(d.Category, cfg.DiagnosticPolicy)
					switch sv {
					case severity.SeverityCritical:
						yield(result, fmt.Errorf("critical diagnostic: %s", d.Message))
						return
					case severity.SeverityError:
						result.HasError = true
					case severity.SeverityWarn:
						result.HasWarn = true
					}
				}
			}
			if phase.AfterPhase != nil {
				if err := phase.AfterPhase(graph); err != nil {
					yield(nil, fmt.Errorf("phase %q after-phase callback: %w", phase.Name, err))
					return
				}
			}
			if !yield(result, nil) {
				return
			}
		}
	}
}
