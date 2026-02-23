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

### Core (root package `phasedchecker`)

- **`checker.go`** — Main entry point. `Main()` parses CLI args, loads packages via `packages.LoadSyntax | packages.NeedModule`, and executes the pipeline. Each `Phase` runs `checker.Analyze()` on its analyzers, processes diagnostics by severity, optionally applies fixes, then calls the `AfterPhase` callback.
- **`severity.go`** — Severity levels and `DiagnosticPolicy` (category-to-severity rules with first-match-wins semantics and a default). Types are in the root package, not a separate sub-package. Severity iota values have reserved gaps for future levels (debug, notice, fatal, emergency).
- **`flags.go`** — CLI argument parsing (`-fix`, `-diff`, `-json`, `-test`, `-debug`, `-V`). Debug flags are a subset of `"fpstv"`: `f`=fact logging, `p`=sequential (no parallelism), `s`=sanity check, `t`=timing, `v`=verbose. The `-V=full` flag implements the `go vet` version protocol (prints executable name + SHA256 hash).
- **`fix.go`** — Fix application via vendored `driverutil.ApplyFixes()`. Uses `reflect` + `unsafe` to extract the unexported `pass` field from `checker.Action` — this couples tightly to the `checker.Action` struct layout in `x/tools`.

### Key Types

- `Phase` — name + analyzers + optional `AfterPhase` callback receiving `*checker.Graph`
- `Pipeline` — ordered sequence of `Phase`s
- `Config` — pipeline + `DiagnosticPolicy` for severity mapping

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
- Tests use `setupTestModule()` to create temporary Go modules, then `t.Chdir(dir)` to switch to the temp directory before calling `run()`
- `checkertest` uses `phasedchecker.Config` directly (no intermediate config package)
- `Severity` iota has intentional gaps (reserved values for future debug/notice/fatal/emergency levels) — do not renumber
- Coverage threshold is 70% (`.octocov.yml`). `internal/x/tools/` and `.examples/` are excluded from coverage and code-to-test ratio metrics
