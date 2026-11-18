---
name: tlapm-mcp
description: Use when checking, proving, debugging, or completing TLA+ proofs. Covers the tlapm-mcp tool `prove` (target selection, result and warning interpretation, error recovery, fingerprint caching, and the proof-repair loop) and when to use the TLA+ LSP instead of the MCP.
---

# tlapm-mcp

`tlapm-mcp` exposes the TLA+ Proof Manager (`tlapm`) over MCP with a single tool: `prove`, which runs TLAPM proof backends for selected source lines. It does **not** read or edit TLA+ source, and it does **not** target theorems by name.

Current interface: `prove`.

## Division of labor with the TLA+ LSP

The tlapm standard LSP (run as `tlapm lsp`) is the tool for everything that is not proving:

- **TLA+ LSP**: syntax and elaboration checking of a whole module, live diagnostics, and standard language actions such as navigation and renaming.
- **`prove` (this MCP)**: running proof backends for selected source lines.

Rules of thumb:

- Before proving, get module diagnostics from the TLA+ LSP and fix reported errors first; it is cheaper than a backend run and pinpoints locations.
- Use the LSP for renames and other structural edits, then re-prove the affected lines with `prove`.
- `prove` still fails with `TLAPM_PARSE_ERROR` when the module does not parse, so a broken module cannot slip through; the LSP just reports it earlier and with better locations.
- The MCP used to have a `check` tool for whole-module parse/elaboration. It was removed: use the TLA+ LSP for that.

## Tool contract

| Tool | Target | Purpose |
|------|--------|---------|
| `prove` | Required: exactly one of `line`, or both `from` and `to` | Run TLAPM proof backends for the selected source line(s). |

Argument rules:

- `module` is required. It may be absolute or relative to the **server's working directory**, not necessarily your shell's working directory. When in doubt, pass an absolute path.
- Lines are one-based source lines. Never combine `line` with `from`/`to`.
- `from` and `to` are inclusive, positive integers, and `to >= from`.
- `cached` is boolean and defaults to `true`.
- A single `line` becomes `tlapm --line N`; a range becomes `tlapm --toolbox FROM TO`. TLAPM, not the MCP, decides what those source lines mean.

## Recommended workflow

1. **Check with the TLA+ LSP first.** Parsing and elaboration errors are reported by the LSP without paying for backends. Fix diagnostics in the module before proving.
2. **Read the actual source.** Locate the `THEOREM`, `LEMMA`, proof step, and `BY` clause with line numbers. Do not guess targets from theorem names.
3. **Prove the smallest useful range.** Include the theorem statement and its proof. If a single line yields `NO_OBLIGATIONS`, widen to the inclusive range covering the step.
4. **Interpret the result.** `success: true` requires an aggregate `All N obligations proved` summary with `N > 0`; a per-obligation success message is not enough.
5. **Repair from unresolved obligations.** `obligations` lists only failures, each with a source `line`, `status`, and usually `failure_reason`. Read that line, then fix the statement, add intermediate steps, or expand definitions with `BY DEF`.
6. **Use warnings only after success.** A `warning` such as `missing \`BY Zenon\`` is a non-fatal hint: adding that `BY` tactic to the indicated step avoids a failed backend on the next run.
7. **Bypass cache after source changes.** Keep the default `cached: true` for iteration; pass `cached: false` after editing definitions or when a result looks stale.

## Prove target selection

```jsonc
// Prove one proof step line
{ "module": "/abs/path/Spec.tla", "line": 35 }

// Prove an inclusive range containing a theorem and its proof
{ "module": "/abs/path/Spec.tla", "from": 35, "to": 42 }

// Ignore cached fingerprints and recompute
{ "module": "/abs/path/Spec.tla", "from": 35, "to": 42, "cached": false }
```

A range can cover several obligations. Prefer it when the exact step line is unclear or when a single-line target reports zero obligations.

## Result interpretation

### `prove`

```jsonc
{
  "success": false,
  "error_code": "PROOF_FAILED",
  "module": "Spec",
  "range": { "start": 28, "end": 34 },
  "total_time_seconds": 0.234,
  "timing": { "interaction": 0.21 },
  "obligation_count": 1,
  "obligations": [
    { "line": 8, "status": "failed", "failure_reason": "false" }
  ],
  "proof_text": "[ERROR]: 1/1 obligation failed."
}
```

- `success: true` only when TLAPM exits zero, emits `[INFO]: All N obligations proved.` with `N > 0`, and emits no `[ERROR]:` lines.
- `success: true` covers **only the selected target**, not the whole module. Claims about other theorems need their own `prove` calls.
- `obligation_count` is the total generated count.
- `obligations` contains **only unresolved** obligations. Do not treat it as the full obligation list. Statuses are `failed`, `timeout`, `parse-error`, or `backend-error`; `failure_reason` is present when TLAPM supplies one.
- `timing` and `total_time_seconds` are seconds; never milliseconds.
- `warnings[]` is emitted only for successful proofs. Each entry has the proof-step `line` and a `what` hint naming the missing `BY` tactic.
- A proof failure is normally a successful tool call whose result has `success: false` and an `error_code`, not an MCP transport error.

## Two error layers

1. **Tool error**: the MCP call itself is marked as an error and returns JSON such as `{"tool":"prove","code":"INVALID_RANGE","message":"..."}`. This covers bad arguments, a missing module, a missing `tlapm` binary, and cancellation. Fix the request or environment.
2. **Result `error_code`**: TLAPM ran but the outcome was a proof failure, timeout, parse failure, or incomplete output. Read the result fields and repair the module.

| Code | Meaning | Action |
|------|---------|--------|
| `INVALID_MODULE` | Module path is empty or does not exist from the server working directory. | Use an absolute path and confirm the file exists. |
| `INVALID_RANGE` | Missing, conflicting, or non-positive target; `to < from`. | Supply exactly one of `line` or `from`+`to`, with positive ordered integers. |
| `INVALID_REQUEST` | Arguments could not be decoded; `cached` was not boolean. | Send valid JSON with the documented types. |
| `TLAPM_PARSE_ERROR` | TLAPM could not parse or elaborate. | Fix the reported syntax/definition error; TLA+ LSP diagnostics pinpoint it. |
| `PROOF_FAILED` | One or more obligations failed. | Inspect `obligations[].line` and `failure_reason`; fix the step or proof. |
| `BACKEND_TIMEOUT` | A proof backend timed out or was interrupted. | Narrow the target, split the proof, and simplify before retrying. |
| `BACKEND_FAILURE` | A backend errored for a reason other than an ordinary failed obligation. | Narrow/split the proof; add an explicit `BY` tactic when one backend is known to work. |
| `FINGERPRINT_CORRUPTED` | TLAPM's fingerprint cache is corrupt. | Retry with `cached: false`; if it persists, remove only the affected module's `.tlacache` entry. |
| `NO_OBLIGATIONS` | The target generated zero obligations, so nothing was proved. | Widen the range to include the theorem/proof step; do not report success. |
| `MCP_REQUEST_TIMEOUT` / `MCP_REQUEST_CANCELLED` | The caller timed out or cancelled. | Retry with a narrower target. |
| `TLAPM_EXIT_NO_DIAGNOSTICS` | TLAPM exited abnormally without a diagnostic. | Inspect `stderr` or rerun with a narrower target. |
| `TLAPM_FAILURE` / `TLAPM_OUTPUT_INCOMPLETE` | TLAPM failed or produced incomplete output. | Rerun, then narrow/split; inspect `proof_text` and stderr. |
| `TLAPM_NOT_FOUND` | The `tlapm` executable is not on the server's `PATH`. | Environment problem; do not retry the same call. |
| `SOLVER_UNAVAILABLE` | A required solver is missing or incompatible. | Environment problem; install the solver or change the proof's tactic. |

## Caching

- `cached: true` (default) uses `.tlacache/` fingerprints and skips already-proved obligations.
- `cached: false` passes `--nofp`, ignores existing fingerprints, and recomputes.
- Keep caching on during normal iteration. Turn it off after changing definitions, hypotheses, or proof steps, or when a cached result appears inconsistent.
- Do not edit or delete cache files as a first response to a failure. Only address the cache after `FINGERPRINT_CORRUPTED`, and only for the affected module.

## Repairing a failed proof

Use the unresolved obligation's `line` as the anchor:

1. Read the source at that line and the enclosing theorem/lemma.
2. `failure_reason: "false"` usually means the obligation is false as stated or is missing a hypothesis/invariant. Re-check the claim before adding tactics.
3. Expand definitions with `BY DEF Name1, Name2`, or add intermediate `<1>...` steps whose facts feed the failing step.
4. If a backend failed before another proved the obligation, the successful result carries a `warning` naming the `BY` tactic to add (for example `BY Zenon` or `BY SMT`). Apply it to the indicated step.
5. Re-prove the same target. Use the TLA+ LSP to confirm the module is still clean after edits.

## Guardrails

- Never claim success from an individual obligation message. Require `success: true` and the aggregate count.
- Never claim success when `obligation_count` is zero or `error_code` is `NO_OBLIGATIONS`.
- Never generalize a successful `prove` to the whole module; it covers only the selected target.
- Do not use the MCP to parse, elaborate, rename, or navigate TLA+ source; use the TLA+ LSP. The MCP only proves.
- Do not pass theorem names, `--line`, `--toolbox`, `--nofp`, or other TLAPM flags in `module`; use `line`/`from`/`to` only.
- Do not expect the MCP to modify the module. Edit the `.tla` file with the normal file tools, then call `prove` again.
- Do not assume the module's working directory: relative paths are resolved by the server process.

## Repository examples

These fixtures are representative targets when the server runs with this repository as its working directory:

```jsonc
// Prove the theorem and its BY clause
{ "module": "testcases/arithmetic_theorem.tla", "from": 6, "to": 8 }

// A fixture with a deliberately failing theorem at lines 45-50
{ "module": "testcases/arithmetic.tla", "from": 45, "to": 50 }
```
