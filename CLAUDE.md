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

# Sync vendored x/tools internal packages (after updating x/tools version in go.mod)
make sync-x-tools
```

## Architecture

### Core (root package `phasedchecker`)

- **`checker.go`** — Main entry point. `Main()` parses CLI args, loads packages, and executes the pipeline. Each `Phase` runs `checker.Analyze()` on its analyzers, processes diagnostics by severity, optionally applies fixes, then calls the `AfterPhase` callback. Exit codes: 0=clean, 1=error/critical, 3=warnings only (no fix mode). In JSON mode (`-json`), exit is always 0 (diagnostics are emitted as JSON to stdout) unless a Critical diagnostic triggers early termination.
- **`severity.go`** — Severity levels (`SeverityInfo`, `SeverityWarn`, `SeverityError`, `SeverityCritical`) and `DiagnosticPolicy` (category-to-severity rules with first-match-wins semantics and a default). These types are in the root package, not a separate `severity/` package.
- **`flags.go`** — CLI argument parsing (`-fix`, `-diff`, `-json`, `-test`, `-debug`). Debug flags are a subset of `"fpstv"`: `f`=fact logging, `p`=sequential (no parallelism), `s`=sanity check, `t`=timing, `v`=verbose.
- **`fix.go`** — Fix application via vendored `driverutil.ApplyFixes()`. Uses `reflect` + `unsafe` to extract the unexported `pass` field from `checker.Action` — this couples tightly to the `checker.Action` struct layout in `x/tools`.

### Key Types

- `Phase` — name + analyzers + optional `AfterPhase` callback receiving `*checker.Graph`
- `Pipeline` — ordered sequence of `Phase`s
- `Config` — pipeline + `DiagnosticPolicy` for severity mapping

### `checkertest/` package

Testing framework analogous to `analysistest`, but for phase-based pipelines:
- `Run()` — runs pipeline and verifies `// want` directives in source
- `RunWithSuggestedFixes()` — additionally checks `.golden` files (plain or txtar archive format)
- `expect.go` — parses `// want "regex"` and `// want +N "regex"` comment directives
- `golden.go` — golden file comparison with diff/merge support
- `internal/testing.go` — minimal `T` interface to enable unit testing of the test framework itself

### `internal/x/tools/` — Vendored internals

Copies of `golang.org/x/tools` internal packages (`diff`, `driverutil`, `free`) with rewritten import paths. Managed by `make sync-x-tools`. Do not edit these files directly.

## Conventions

- Go 1.25 required
- Tests use `setupTestModule()` to create temporary Go modules with specific source files
- `checkertest` uses `phasedchecker.Config` directly (no intermediate config package)
