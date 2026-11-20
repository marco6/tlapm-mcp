# Preface

This document describes the scope of the `internal/prover/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the core TLA⁺ prover logic.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The prover invokes TLAPM for parse/elaboration checks and numeric source-line proof targets, parses TLAPM output conservatively, and handles fingerprint caching for proofs. `check` uses no-backend summary mode; `prove` runs the proof for the selected target. The MCP never reads or parses module contents.


# Architecture

The prover follows a layered approach:

1. **Invocation**: `buildCheckCmd()` runs summary mode with backend verifiers disabled; `buildProveCmd()` runs `tlapm --timing --strict` with `--line N` or `--toolbox FROM TO` targets, requiring a `tlapm` release that supports `--strict` (1.6.0-pre or newer).
2. **Output parsing**: `Check()` extracts summary obligation counts and best-effort diagnostics. `parseToolboxObligations()` merges repeated Toolbox events by internal obligation ID and records each backend attempt; results expose only unresolved obligations with source line and status. `parseResult()` only marks a proof complete on an aggregate `[INFO]: All N obligations proved.` line with no `[ERROR]:` lines; callers also require a zero process exit status. Under `--strict`, an explicit target that selects no obligation (exit 12) is classified as `NO_OBLIGATIONS`, and a proof with missing or omitted steps (exit 11) as `PROOF_INCOMPLETE`. Successful proofs whose obligations needed backend fallback also carry compact `warnings` with the proof-step line and a `what` hint naming the missing `BY` annotation.
3. **Fingerprinting**: `FPMode` enum (`FPDefault`, `FPNo`, `FPCheck`) controls cache behavior via `--usefp`, `--nofp`, and `--safefp` flags. The MCP exposes only `cached: true|false`.
4. **Targeting**: `check` always checks the whole module and takes no target. `prove` requires a one-based `line` or inclusive `from`-`to` target, passed as `--line N` or `--toolbox FROM TO`.
5. **Concurrency**: `Prove()` and `Check()` acquire a context-aware advisory `flock` keyed by the module's cache name (the file base name without `.tla`) before launching TLAPM. The lock file is a sidecar under `<TLAPM_CACHE_DIR>/.tlapm-mcp-locks/` (default `.tlacache/.tlapm-mcp-locks/`). Calls for different modules use different sidecars and run concurrently; calls sharing a module key, including `check`/`prove` pairs, contend on one sidecar and serialize across server processes. The sidecar is not the fingerprint file: TLAPM replaces that path by rename, invalidating locks held on the old inode. `--summary` still initializes fingerprints, so `check` takes the same lock.


# Important

- **Timing**: All timing values are in seconds (float64), never milliseconds.
- **Cache behavior**: The MCP's `cached` option defaults to true (TLAPM's default); false ignores cached proofs. `FPCheck` remains an internal prover mode.
- **Error handling**: `TLAPMError` classifies invalid targets, TLAPM parse/proof/backend failures, strict empty targets (`NO_OBLIGATIONS`) and incomplete proofs (`PROOF_INCOMPLETE`), fingerprint corruption, MCP cancellation, exits without diagnostics, and module-lock/cache `IO_ERROR`s.
- **Obligation records**: toolbox status events are merged by run-local obligation ID; only unresolved results are returned. `being proved` and `interrupted` map to `timeout`; obligations without a terminal result map to `backend-error`.
- **Warnings**: only successful proofs produce them. A warning is emitted when a known backend failed before another backend proved the obligation; non-backend failures such as `expandcdot` and in-progress events are ignored. Duplicate line/backend warnings are dropped.
- **FS abstraction**: Relative paths use `os.DirFS(".")` in production and `embed.FS` in tests; absolute paths are checked with `os.Stat`.
- **Concurrency safety**: Never launch TLAPM for a module without `acquireModuleLock()`. The gate is keyed by `moduleCacheKey()`, which maps path aliases to TLAPM's cache directory name; the MCP does not inspect module contents, so a declared module name that differs from the file base name is not detected. The advisory sidecar is cooperative: TLAPM itself does not lock it, so direct `tlapm` runs bypass the gate. Platforms without `flock` run without a module lock.


# Directory

- `prover.go` - `Prover`, line/range argument types, `Prove()`, filesystem path checks, and proof command construction.
- `check.go` - Parse/elaboration-only `Check()` operation, its result and diagnostic types, and TLAPM summary parsing.
- `result.go` - Proof output types (`Result`, `Warning`), `parseResult()`, and compact backend-fallback warnings.
- `obligations.go` - Compact Toolbox event parsing, backend-attempt tracking, and unresolved-obligation filtering.
- `errors.go` - `TLAPMError` struct, error codes, `WrapToolError()`, `DetectErrorClass()`, `LogError()`, and module path checks.
- `locks.go` - Per-module TLAPM invocation gate, cache-key and sidecar-path normalization, and context-aware waiting.
- `locks_flock.go` - Unix advisory `flock` primitive with non-blocking polling and context cancellation.
- `locks_noflock.go` - No-op sidecar lock on platforms without advisory `flock`.
- `locks_test.go` - Cache-key/path, same-module serialization, cross-tool locking, different-module concurrency, and cancellation tests.
- `locks_flock_test.go` - Cross-process advisory lock coverage using a helper process.
- `integration_test.go` - Integration coverage for line/range prove, structured obligations, output parsing, and fingerprint caching.
- `obligations_test.go` - Toolbox status parsing, compact-response, and backend-warning tests.
- `check_test.go` - Check command, summary/diagnostic parsing, and parse/elaboration integration coverage.
- `testdata/` - TLA⁺ example modules embedded by integration tests (arithmetic, failing, folding, hard_proofs, sequences). All derived from https://github.com/tlaplus/CommunityModules/ and abide to its MIT license terms.
