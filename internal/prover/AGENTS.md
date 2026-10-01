# Preface

This document describes the scope of the `internal/prover/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the core TLA⁺ prover logic.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The `prover/` package invokes tlapm for numeric source-line targets, parses tlapm output, and handles fingerprint caching. It never reads or parses module contents; a single line maps to `--line`, and an inclusive range maps to `--toolbox`.


# Architecture

The prover follows a layered approach:

1. **Invocation**: `buildProveCmd()` and `raceSolvers()` construct tlapm commands with solver, timing, fingerprint, and target flags.
2. **Output parsing**: `parseResult()` parses tlapm console output into `Result` fields for success/failure, timing, and obligations.
3. **Fingerprinting**: `FPMode` enum (`FPDefault`, `FPNo`, `FPCheck`) controls cache behavior via `--usefp`, `--nofp`, and `--safefp` flags.
4. **Targeting**: `LineTarget` maps one-based lines to `--line N` and inclusive ranges to `--toolbox START END`.


# Important

- **Timing**: All timing values are in seconds (float64), never milliseconds.
- **Fingerprints**: Default mode (`FPDefault`) uses cached results; `FPNo` ignores cache; `FPCheck` validates versions before using cache.
- **Error handling**: `TLAPMError` struct provides typed errors with codes (`ErrModuleNotFound`, `ErrSolverUnavailable`, etc.).
- **Solvers**: `raceSolvers()` auto-detects available solvers from tlapm's `--config` output; distinguishes "solver not available" (executable missing, `obligations_failed = -1`) from "solver failed" (tlapm ran but returned non-zero).
- **FS abstraction**: Production code uses `os.DirFS(".")` directly; tests use `go:embed` with `embed.FS`.


# Directory

- `prover.go` - `Prover`, line/range argument types, `Prove()`, `Race()`, filesystem path checks, and `buildProveCmd()`.
- `result.go` - Output types (`Result`, `RaceResult`), `parseResult()`, and `raceSolvers()`.
- `errors.go` - `TLAPMError` struct, error codes, `WrapToolError()`, `DetectErrorClass()`, `LogError()`, and module path checks.
- `integration_test.go` - Integration coverage for line/range prove and race, output parsing, and fingerprint caching.
- `testdata/` - TLA⁺ example modules embedded by integration tests (arithmetic, failing, folding, hard_proofs, sequences). All derived from https://github.com/tlaplus/CommunityModules/ and abide to its MIT license terms.

