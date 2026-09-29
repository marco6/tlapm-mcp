# Preface

This document describes the scope of the `internal/prover/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the core TLA⁺ prover logic.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The `prover/` package is the heart of tlapm-mcp. It orchestrates tlapm binary invocations, parses structured output (timing, obligations, success/failure), handles fingerprint caching, and resolves step notation (nested paths and DFS ranges). The package uses `fs.FS` for filesystem abstraction, enabling `os.DirFS(".")` in production and `embed.FS` in tests.


# Architecture

The prover follows a layered approach:

1. **Invocation**: `buildProveCmd()` constructs the `exec.Cmd` with solver, timing, fingerprint, and line flags.
2. **Parsing**: `parseResult()` parses tlapm's console output into `Result` structs with timing breakdowns and obligation details.
3. **Fingerprinting**: `FPMode` enum (`FPDefault`, `FPNo`, `FPCheck`) controls cache behavior via `--usefp`, `--nofp`, and `--safefp` flags.
4. **Step resolution**: `ParseStep()` handles nested paths (`<1>/<2>`) and DFS ranges (`<1>..<3>`) via `buildDFSTree()`.


# Important

- **Timing**: All timing values are in seconds (float64), never milliseconds.
- **Fingerprints**: Default mode (`FPDefault`) uses cached results; `FPNo` ignores cache; `FPCheck` validates versions before using cache.
- **Error handling**: `TLAPMError` struct provides typed errors with codes (`ErrModuleNotFound`, `ErrSolverUnavailable`, etc.).
- **Solvers**: `raceSolvers()` auto-detects available solvers from tlapm's `--config` output; distinguishes "solver not available" (executable missing, `obligations_failed = -1`) from "solver failed" (tlapm ran but returned non-zero).
- **FS abstraction**: Production code uses `os.DirFS(".")` directly; tests use `go:embed` with `embed.FS`.


# Directory

- `prover.go` - `Prover` struct, argument types, `Prove()`, `Race()`, `ListTheorems()`, `ResolveRange()`, and `buildProveCmd()`.
- `result.go` - Output types (`Result`, `RaceResult`, `ListResult`), `parseResult()`, `raceSolvers()`, `listTheorems()`.
- `step.go` - `ParseStep()`, `Step` struct, `StepKind` enum, `buildDFSTree()`, `ResolveStepRange()`.
- `errors.go` - `TLAPMError` struct, error codes, `WrapToolError()`, `DetectErrorClass()`, `LogError()`.
- `step_test.go` - Unit tests for step parsing.
- `integration_test.go` - Integration tests covering prove, race, list_theorems, DFS resolution, and fingerprint caching.
- `testdata/` - TLA⁺ example files embedded by integration tests (arithmetic, failing, folding, hard_proofs, sequences). All derived from https://github.com/tlaplus/CommunityModules/ and abide to its MIT license terms.

