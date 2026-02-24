# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

phasedchecker is a phase-aware driver for Go static analysis (`golang.org/x/tools/go/analysis`). Unlike `multichecker` which runs all analyzers per-package with no phase ordering, phasedchecker executes analyzers in sequential phases — each phase completes for ALL packages before the next phase starts. It uses `golang.org/x/tools/go/analysis/checker.Analyze()` internally.

## Commands

```bash
# Run all tests with race detection
go test -race ./...

# Run a single test
go test -run TestName ./...

# Run tests for a specific package
go test ./checkertest/...

# Run example tests (each example has its own go.mod)
cd .examples/basic && go test -v ./...

# Sync vendored x/tools internal packages (after updating x/tools version in go.mod)
make sync-x-tools
```

## Architecture

### Layering

The root package (`phasedchecker`) is a thin public API layer that re-exports types from internal packages as type aliases:
- `Phase`, `Pipeline`, `Config` → `internal/runner`
- `Severity`, `CategoryRule`, `DiagnosticPolicy` → `internal/severity`

The actual implementation lives in `internal/` sub-packages.

### Core (root package `phasedchecker`)

- **`checker.go`** — `Main()` parses CLI args via `arg.ParseArgs()`, then delegates to internal `run()`. `run()` loads packages via `runner.LoadPackages()`, iterates `runner.RunPipeline()`, processes diagnostics by severity, optionally applies fixes, then returns the exit code. Handles JSON output via `driverutil.JSONTree` and timing output for the slowest 90% of analyzers.
- **`severity.go`** — Type aliases re-exporting severity types from `internal/severity`. Severity iota values have reserved gaps for future levels (debug, notice, fatal, emergency) — do not renumber.
- **`fix.go`** — Fix application via vendored `driverutil.ApplyFixes()`. Uses `reflect` + `unsafe` to extract the unexported `pass` field from `checker.Action` — this couples tightly to the `checker.Action` struct layout in `x/tools`.

### `internal/runner/` — Pipeline Execution Engine

- **`types.go`** — `Phase` (name + analyzers + optional `AfterPhase` callback), `Pipeline` (ordered phases), `Config` (pipeline + `DiagnosticPolicy`), `PhaseResult` (per-phase result with `Graph`, `HasError`, `HasWarn`)
- **`pipeline.go`** — `LoadPackages()` loads packages via `packages.Load()`. `RunPipeline()` returns a Go iterator (`iter.Seq2[*PhaseResult, error]`) that yields results per phase. On `SeverityCritical`, yields `ErrCriticalDiagnostic` and stops. On `AfterPhase` error, yields `ErrAfterPhase` and stops.
- **`resolve.go`** — `ResolveSeverity()` maps diagnostic category to severity via first-match-wins linear search through `DiagnosticPolicy.Rules`.

### `internal/arg/` — CLI Argument Parsing

- **`flags.go`** — `ParseArgs()` parses CLI flags (`-fix`, `-diff`, `-json`, `-test`, `-debug`, `-V`). `Argument` struct holds parsed results. `Dbg(b byte) bool` checks individual debug flags. Debug flags are a subset of `"fpstv"`: `f`=fact logging, `p`=sequential (no parallelism), `s`=sanity check, `t`=timing, `v`=verbose. The `-V=full` flag implements the `go vet` version protocol.

### `internal/testutil/` — Shared Test Utilities

- **`module.go`** — `SetupTestModule()` creates a temporary Go module directory with auto-generated `go.mod` and given files.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Clean, or JSON mode (unless Critical) |
| 1 | Error/Critical diagnostics or internal error |
| 3 | Warnings only (no fix mode) |

`SeverityCritical` aborts the pipeline during the current phase, skipping all subsequent phases.

### `checkertest/` package

Testing framework analogous to `analysistest`, but for phase-based pipelines:
- `Run()` — runs pipeline and verifies `// want` directives in source
- `RunWithSuggestedFixes()` — additionally checks `.golden` files (plain or txtar archive format)
- `expect.go` — parses `// want "regex"`, `// want "a" "b"` (multiple), and `// want +N "regex"` (line offset) comment directives. Fact expectations (`name:"pattern"`) are explicitly rejected — phasedchecker does not use analysis facts.
- `golden.go` — golden file comparison with diff/merge support
- `internal/testing.go` — minimal `T` interface to enable unit testing of the test framework itself

`collectExpectations` only scans root packages (not transitive dependencies like stdlib) to avoid false matches from unrelated `// want` comments in third-party source.

### `internal/x/tools/` — Vendored internals

Copies of `golang.org/x/tools` internal packages (`diff`, `driverutil`, `free`) with rewritten import paths. Managed by `make sync-x-tools`. Do not edit these files directly.

### `.examples/` — Runnable examples

Three examples (`basic`, `severity`, `multiphase`), each with its own `go.mod` using `replace` directive pointing to `../..`. Each has:
- `main.go` — checker implementation using `phasedchecker.Main()`
- `target/` — sample Go source for `go run . ./target/...`
- `testdata/` + `main_test.go` — demonstrates `checkertest.Run()` usage

## Conventions

- Go 1.25 required
- Tests use `testutil.SetupTestModule()` to create temporary Go modules, then `t.Chdir(dir)` to switch to the temp directory before calling `run()`
- `checkertest` uses `phasedchecker.Config` directly (no intermediate config package)
- `Severity` iota has intentional gaps (reserved values for future debug/notice/fatal/emergency levels) — do not renumber
- Coverage threshold is 70% (`.octocov.yml`). `internal/x/tools/` and `.examples/` are excluded from coverage and code-to-test ratio metrics
