# phasedchecker

[![Go Reference](https://pkg.go.dev/badge/github.com/miyamo2/phasedchecker.svg)](https://pkg.go.dev/github.com/miyamo2/phasedchecker)
[![GitHub go.mod Go version (subdirectory of monorepo)](https://img.shields.io/github/go-mod/go-version/miyamo2/phasedchecker?logo=go)](https://img.shields.io/github/go-mod/go-version/miyamo2/phasedchecker?logo=go)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/miyamo2/phasedchecker)](https://img.shields.io/github/v/release/miyamo2/phasedchecker)
![Coverage](https://github.com/miyamo2/phasedchecker/blob/main/.assets/test_cov.svg?raw=true)
![Code to Test Ratio](https://github.com/miyamo2/phasedchecker/blob/main/.assets/ratio.svg?raw=true)
![Test Execution Time](https://github.com/miyamo2/phasedchecker/blob/main/.assets/time.svg?raw=true)
[![Go Report Card](https://goreportcard.com/badge/github.com/miyamo2/phasedchecker)](https://goreportcard.com/report/github.com/miyamo2/phasedchecker)
[![GitHub License](https://img.shields.io/github/license/miyamo2/phasedchecker?&color=blue)](https://img.shields.io/github/license/miyamo2/phasedchecker?&color=blue)

A phase-aware driver for Go static analysis ([`golang.org/x/tools/go/analysis`](https://pkg.go.dev/golang.org/x/tools/go/analysis)).

Unlike `multichecker`, which runs all analyzers per-package with no phase ordering, phasedchecker executes analyzers in **sequential phases** — each phase completes for **all packages** before the next phase starts. This enables inter-phase communication where later phases can consume results produced by earlier ones.

## Requirements

- Go 1.25+

## Installation

```bash
go get github.com/miyamo2/phasedchecker
```

## Quick Start

```go
package main

import (
	"fmt"
	"os"

	"golang.org/x/tools/go/analysis"
	"github.com/miyamo2/phasedchecker"
)

var myAnalyzer = &analysis.Analyzer{
	Name: "example",
	Doc:  "reports something",
	Run: func(pass *analysis.Pass) (any, error) {
		// your analysis logic here
		return nil, nil
	},
}

func main() {
	cfg := phasedchecker.Config{
		Pipeline: phasedchecker.Pipeline{
			Phases: []phasedchecker.Phase{
				{
					Name:      "lint",
					Analyzers: []*analysis.Analyzer{myAnalyzer},
				},
			},
		},
		DiagnosticPolicy: phasedchecker.DiagnosticPolicy{
			DefaultSeverity: phasedchecker.SeverityWarn,
		},
	}
	phasedchecker.Main(cfg)
}
```

```bash
go run . ./...
```

## Core Concepts

### Phase / Pipeline / Config

- **`Phase`** — A named group of analyzers that run together. All analyzers in a phase complete on all packages before the next phase starts.
- **`Pipeline`** — An ordered sequence of `Phase`s executed sequentially.
- **`Config`** — Combines a `Pipeline` with a `DiagnosticPolicy` to control the checker's behavior.

### DiagnosticPolicy and Severity

`DiagnosticPolicy` maps diagnostic categories to severity levels using an ordered list of `CategoryRule`s. The first matching rule wins. If no rule matches, `DefaultSeverity` is applied.

```go
phasedchecker.DiagnosticPolicy{
    Rules: []phasedchecker.CategoryRule{
        {Category: "todo", Severity: phasedchecker.SeverityInfo},
        {Category: "panic", Severity: phasedchecker.SeverityError},
    },
    DefaultSeverity: phasedchecker.SeverityWarn,
}
```

### AfterPhase Callbacks

Each `Phase` can have an optional `AfterPhase` callback that receives the `*checker.Graph` after the phase completes. This enables inter-phase communication — for example, Phase 1 can collect data via `Action.Result` values, and Phase 2 can use that data.

```go
phasedchecker.Phase{
    Name:      "scan",
    Analyzers: []*analysis.Analyzer{scanAnalyzer},
    AfterPhase: func(graph *checker.Graph) error {
        for act := range graph.All() {
            if act.Analyzer == scanAnalyzer && act.Err == nil {
                // extract and aggregate act.Result
            }
        }
        return nil
    },
}
```

See the [multiphase example](.examples/multiphase/main.go) for a complete demonstration.

## Severity Levels

| Severity | Reported | Exit Code | Pipeline |
|---|---|---|---|
| `SeverityInfo` | No | No effect | Continues |
| `SeverityWarn` | Yes (stderr) | 3 (if no errors and fix mode disabled) | Continues |
| `SeverityError` | Yes (stderr) | 1 | Continues |
| `SeverityCritical` | Yes (stderr) | 1 | **Aborts at the current phase** |

## CLI Flags

The binary built with `phasedchecker.Main` accepts the following flags:

| Flag | Default | Description |
|---|---|---|
| `-fix` | `false` | Apply suggested fixes |
| `-diff` | `false` | With `-fix`, print unified diffs instead of updating files |
| `-json` | `false` | Emit JSON diagnostics to stdout |
| `-test` | `true` | Include test files in analysis |
| `-debug` | `""` | Debug flags, any subset of `"fpstv"` |

### Debug Flags

| Flag | Description |
|---|---|
| `f` | Log analysis facts to stderr |
| `p` | Sequential execution (disable parallelism) |
| `s` | Enable sanity checks |
| `t` | Print timing information |
| `v` | Verbose logging |

## Exit Codes

| Code | Meaning |
|---|---|
| 0 | Clean (no diagnostics) or JSON mode |
| 1 | Error/Critical diagnostics or internal error |
| 3 | Warnings only (no fix mode) |

## Testing with `checkertest`

The `checkertest` package provides testing utilities analogous to [`analysistest`](https://pkg.go.dev/golang.org/x/tools/go/analysis/analysistest), designed for phase-based pipelines.

### Run

`Run` executes the pipeline against test packages and verifies that diagnostics match `// want` directives in source files.

```go
func TestMyPipeline(t *testing.T) {
    cfg := phasedchecker.Config{
        Pipeline: phasedchecker.Pipeline{
            Phases: []phasedchecker.Phase{
                {Name: "lint", Analyzers: []*analysis.Analyzer{myAnalyzer}},
            },
        },
        DiagnosticPolicy: phasedchecker.DiagnosticPolicy{
            DefaultSeverity: phasedchecker.SeverityWarn,
        },
    }
    checkertest.Run(t, "testdata", cfg, "./...")
}
```

### RunWithSuggestedFixes

Like `Run`, but additionally applies `SuggestedFixes` and compares the results against `.golden` files.

```go
checkertest.RunWithSuggestedFixes(t, "testdata", cfg, "./...")
```

### `// want` Directives

Place `// want` comments in test source files to declare expected diagnostics:

```go
var x = 1 // want "diagnostic message regex"
```

Multiple expectations on the same line:

```go
var x = 1 // want "first pattern" "second pattern"
```

Expect a diagnostic on a line offset from the comment:

```go
// want +2 "pattern on line two below"

var x = 1
```

### Golden Files

For `RunWithSuggestedFixes`, place `.golden` files alongside test source files. The file should contain the expected source after fixes are applied. Both plain files and txtar archive format are supported.

## Examples

The [`.examples/`](.examples/) directory contains runnable examples:

| Example | Description |
|---|---|
| [`basic`](.examples/basic/main.go) | Single-phase naming convention checker |
| [`multiphase`](.examples/multiphase/main.go) | Inter-phase communication via `AfterPhase` callbacks |
| [`severity`](.examples/severity/main.go) | Per-category severity control with `DiagnosticPolicy` |

## License

[BSD 3-Clause](LICENSE)
