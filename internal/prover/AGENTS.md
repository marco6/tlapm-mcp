# Preface

This document describes the scope of the `internal/prover/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the core TLA⁺ prover logic.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The prover invokes TLAPM for parse/elaboration checks and numeric source-line proof targets, parses TLAPM output conservatively, and handles fingerprint caching for proofs. `check` uses no-backend summary mode; `race` tries a fixed list of TLAPM methods. The MCP never reads or parses module contents.


# Architecture

The prover follows a layered approach:

1. **Invocation**: `buildCheckCmd()` runs summary mode with backend verifiers disabled; `buildProveCmd()` and `raceSolvers()` consume TLAPM Toolbox events. `raceSolvers()` selects each fixed TLAPM method with `--method`; race concurrency is capped at two invocations.
2. **Output parsing**: `Check()` extracts summary obligation counts and best-effort diagnostics. `parseToolboxObligations()` merges repeated Toolbox events by internal obligation ID; results expose only unresolved obligations with source line and status. `parseResult()` only marks a proof complete on an aggregate `[INFO]: All N obligations proved.` line where `N > 0` and there are no `[ERROR]:` lines; callers also require a zero process exit status. A zero-obligation summary is classified as `NO_OBLIGATIONS`.
3. **Fingerprinting**: `FPMode` enum (`FPDefault`, `FPNo`, `FPCheck`) controls cache behavior via `--usefp`, `--nofp`, and `--safefp` flags. The MCP exposes only `cached: true|false`.
4. **Targeting**: `check` accepts no target or a one-based `line` / inclusive `from`-`to` target. `prove` and `race` require one of those targets. Targets pass as `--line N` or `--toolbox FROM TO`.


# Important

- **Timing**: All timing values are in seconds (float64), never milliseconds.
- **Cache behavior**: The MCP's `cached` option defaults to true (TLAPM's default); false ignores cached proofs. `FPCheck` remains an internal prover mode.
- **Error handling**: `TLAPMError` classifies invalid targets, TLAPM parse/proof/backend failures, fingerprint corruption, MCP cancellation, and exits without diagnostics.
- **Obligation records**: toolbox status events are merged by run-local obligation ID; only unresolved results are returned. `being proved` and `interrupted` map to `timeout`; obligations without a terminal result map to `backend-error`.
- **Methods**: `Prover.Race` uses a fixed method list with `--method`; it does not inspect `tlapm --config`. Missing TLAPM is returned as `TLAPM_NOT_FOUND`; a method-level failure is not proof success.
- **FS abstraction**: Relative paths use `os.DirFS(".")` in production and `embed.FS` in tests; absolute paths are checked with `os.Stat`.


# Directory

- `prover.go` - `Prover`, line/range argument types, `Prove()`, `Race()`, filesystem path checks, and proof command construction.
- `check.go` - Parse/elaboration-only `Check()` operation, its result and diagnostic types, and TLAPM summary parsing.
- `result.go` - Proof output types (`Result`, `RaceResult`), `parseResult()`, and `raceSolvers()`.
- `obligations.go` - Compact Toolbox event parsing and unresolved-obligation filtering.
- `errors.go` - `TLAPMError` struct, error codes, `WrapToolError()`, `DetectErrorClass()`, `LogError()`, and module path checks.
- `integration_test.go` - Integration coverage for line/range prove and race, structured obligations, output parsing, and fingerprint caching.
- `obligations_test.go` - Toolbox status parsing and compact-response tests.
- `check_test.go` - Check command, summary/diagnostic parsing, and parse/elaboration integration coverage.
- `testdata/` - TLA⁺ example modules embedded by integration tests (arithmetic, failing, folding, hard_proofs, sequences). All derived from https://github.com/tlaplus/CommunityModules/ and abide to its MIT license terms.
