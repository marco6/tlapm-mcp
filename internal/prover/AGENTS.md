# Preface

This document describes the scope of the `internal/prover/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the core TLA⁺ prover logic.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The prover invokes tlapm for numeric source-line targets, parses tlapm output conservatively, and handles fingerprint caching. `race` tries a fixed list of TLAPM methods; it never reads or parses module contents.


# Architecture

The prover follows a layered approach:

1. **Invocation**: `buildProveCmd()` selects an optional SMT solver with `--solver`; `raceSolvers()` selects each fixed TLAPM method with `--method` and adds timing, fingerprint, and target flags.
2. **Output parsing**: `parseResult()` only marks a proof complete on an aggregate `[INFO]: All N obligations proved.` line with no `[ERROR]:` lines; callers also require a zero process exit status.
3. **Fingerprinting**: `FPMode` enum (`FPDefault`, `FPNo`, `FPCheck`) controls cache behavior via `--usefp`, `--nofp`, and `--safefp` flags.
4. **Targeting**: `LineTarget` maps one-based lines to `--line N` and inclusive ranges to `--toolbox START END`.


# Important

- **Timing**: All timing values are in seconds (float64), never milliseconds.
- **Fingerprints**: Default mode (`FPDefault`) uses cached results; `FPNo` ignores cache; `FPCheck` validates versions before using cache.
- **Error handling**: `TLAPMError` struct provides typed errors with codes (`ErrModuleNotFound`, `ErrSolverUnavailable`, etc.).
- **Methods**: `Prover.Race` uses a fixed method list with `--method`; it does not inspect `tlapm --config`. Missing TLAPM is returned as `TLAPM_NOT_FOUND`; a method-level failure is not proof success.
- **FS abstraction**: Relative paths use `os.DirFS(".")` in production and `embed.FS` in tests; absolute paths are checked with `os.Stat`.


# Directory

- `prover.go` - `Prover`, line/range argument types, `Prove()`, `Race()`, filesystem path checks, and `buildProveCmd()`.
- `result.go` - Output types (`Result`, `RaceResult`), `parseResult()`, and `raceSolvers()`.
- `errors.go` - `TLAPMError` struct, error codes, `WrapToolError()`, `DetectErrorClass()`, `LogError()`, and module path checks.
- `integration_test.go` - Integration coverage for line/range prove and race, output parsing, and fingerprint caching.
- `testdata/` - TLA⁺ example modules embedded by integration tests (arithmetic, failing, folding, hard_proofs, sequences). All derived from https://github.com/tlaplus/CommunityModules/ and abide to its MIT license terms.
