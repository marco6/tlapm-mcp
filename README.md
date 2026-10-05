# tlapm-mcp

An MCP server that wraps the [TLA⁺ Proof Manager (tlapm)](https://github.com/tlaplus/tlapm) to let LLMs reason about TLA⁺ proofs interactively.

## Quick start

Prove a specific source line, or an inclusive line range, in a TLA+ module:

- **Line 28** — runs `tlapm --line 28`.
- **Lines 28–34** — runs `tlapm --toolbox 28 34`.
- **Prove with solver `z3`** — selects Z3 as the SMT backend instead of the configured default.
- **Prove without cache** — disables fingerprint/caching.
- **Race methods** — tries the supported TLAPM methods in parallel and returns the fastest proved result, or the fastest failure if none proves the target.

The MCP does not parse TLA+ source or resolve theorem paths. `module` accepts an absolute filesystem path or a path relative to the server working directory. The server passes the requested line or range directly to `tlapm` and parses only tool output.

## MCP Tools

### `prove`

Prove one source line or an inclusive source line range. Provide exactly one of `line` or `range`.

```jsonc
{
  "module": "/path/to/Spec.tla",       // required: absolute or server-working-directory-relative module path
  "line": 28,                           // one-based source line; alternatively use range
  "solver": "z3",                      // optional: SMT backend passed to tlapm --solver
  "use_fingerprints": true,             // optional: cache mode (default)
  "threads": 1                          // optional: positive integer; one is the default
}
```

`solver` selects the SMT backend and is passed to TLAPM as `--solver`. It does not select a proof method such as `smt`, `zenon`, or `blast`; omit it to use TLAPM's configured default.

For a range, replace `line` with an inclusive `range` object:

```jsonc
{
  "module": "/path/to/Spec.tla",       // absolute or server-working-directory-relative path
  "range": { "start": 28, "end": 34 }
}
```

Single lines use `--line N`; ranges use `--toolbox START END`.

**Returns:**

```jsonc
{
  "success": true,
  "module": "Spec",
  "line": 28,                         // or "range": { "start": 28, "end": 34 }
  "solver": "z3",
  "total_time_seconds": 0.234,
  "timing": { "interaction": 0.210 },
  "proof_text": "[INFO]: All 37 obligations proved.",
  "fingerprints_used": "use"
}
```

The server reports `success: true` only when `tlapm` exits successfully and emits an aggregate `[INFO]: All N obligations proved.` line. A message that only says an individual obligation was proved is not sufficient. `obligations` contains parsed `[ERROR]:` messages; source lines are not extracted by the current parser.

### `race`

Try each method in the fixed method list documented below on one source line or an inclusive line range. Each candidate is passed to TLAPM with `--method`; calls run in parallel up to `threads`, and each method reports its own result, including unavailable methods.

```jsonc
{
  "module": "/path/to/Spec.tla",
  "line": 28,                           // alternatively: "range": { "start": 28, "end": 34 }
  "use_fingerprints": true,
  "threads": 2                          // positive integer; max parallel prover invocations
}
```

`line` is passed as `--line N`; `range` is passed as `--toolbox START END`.

**Returns:**

```jsonc
{
  "fastest": {
    "solver": "smt",
    "success": true,
    "total_time_seconds": 0.087
  },
  "results": [
    { "solver": "smt",   "success": true,  "total_time_seconds": 0.087, "obligations_failed": 0 },
    { "solver": "zenon", "success": true,  "total_time_seconds": 0.152, "obligations_failed": 0 },
    { "solver": "z3",    "success": false, "total_time_seconds": 1.203, "obligations_failed": -1, "error": "TLAPM output did not confirm that all obligations were proved" }
  ]
}
```

`obligations_failed` is `-1` when the wrapper cannot establish a count. If no method proves the target, `fastest` contains the fastest failed result; it is not a successful proof.

The response keeps `solver` as the result-field name for compatibility; its value identifies the attempted TLAPM method.

## Line and range targeting

Targets are one-based source line numbers, not theorem names or proof-step paths. Supply exactly one target:

```jsonc
{ "module": "Spec.tla", "line": 35 }
{ "module": "Spec.tla", "range": { "start": 35, "end": 42 } }
```

- `line` selects one source line and becomes `tlapm --line N`.
- `range` selects inclusive start/end lines and becomes `tlapm --toolbox START END`.

Absolute module paths and paths relative to the server working directory are accepted. The MCP does not inspect module contents; `tlapm` interprets the source and target.

## Race Method Reference
| Method | Description |
|--------|-------------|
| `smt` | TLAPM's default SMT method (usually Z3 via SMTLIB) |
| `z3` | Z3 method |
| `zenon` | Zenon tableau prover |
| `auto` | Isabelle with "auto" tactic |
| `blast` | Isabelle with "blast" tactic |
| `force` | Isabelle with "force" tactic |
| `cvc4` | CVC4 |
| `yices` | Yices |
| `verit` | VeriT |
| `spass` | SPASS |
| `zipper` | Zipperposition |
| `ls4` | LS4 temporal logic decision procedure |
| `fail` | Dummy — always fails (useful for testing) |

## Fingerprint / Cache Control

tlapm caches proof results in fingerprint files (`.tlacache/`). The server honors these by default:

- **`use_fingerprints: true`** (default) — loads cached results; skips proven obligations.
- **`use_fingerprints: false`** — ignores existing fingerprints, recomputes. Useful after definition changes.
- **`use_fingerprints: "check"`** — loads fingerprints but validates tlapm/zenon/Isabelle versions (`--safefp`).

## Failing over to a model-readable error

The `obligations` array contains messages parsed from `[ERROR]:` output lines. The current parser does not extract source locations or reconstruct full proof obligations, so these messages are diagnostic text rather than a complete obligation listing.

## Example interactions

```
> Prove the theorem beginning on line 28 in `hard_proofs.tla`.

< {"module": "hard_proofs.tla", "line": 28, "solver": "z3", "use_fingerprints": true}

< {"success": true, "module": "hard_proofs", "line": 28, "solver": "z3", "total_time_seconds": 0.019}

> Prove source lines 28 through 34 without cache.

< {"module": "hard_proofs.tla", "range": {"start": 28, "end": 34}, "use_fingerprints": false}

< {"success": true, "module": "hard_proofs", "range": {"start": 28, "end": 34}, "solver": "smt", "total_time_seconds": 0.018}

> Race all provers on line 28 of `hard_proofs.tla`.

< {"module": "hard_proofs.tla", "line": 28, "use_fingerprints": true}

< {"fastest": {"solver": "smt", "success": true, "total_time_seconds": 0.087}, "results": [ ... ]}
```

## Build & Editor Setup

### Building

```bash
# Ensure Go 1.24+ is available
go version

# Build the binary (outputs to ./bin/tlapm-mcp)
go build -o bin/tlapm-mcp ./cmd/tlapm-mcp/

# Or run directly without building
go run ./cmd/tlapm-mcp/
```

The binary communicates over MCP stdio as a long-lived process. Every `prove` and `race` call requires exactly one of `line` or inclusive `range`; solver selection and fingerprinting remain per-command options.

### Opencode

Add `tlapm-mcp` as an MCP server in your Opencode configuration:

```jsonc
// .opencode/config.jsonc (or your Opencode config file)
{
  "mcpServers": {
    "tlapm": {
      "command": "go",
      "args": ["run", "./cmd/tlapm-mcp/"],
      "cwd": "/path/to/tlapm-mcp"
    }
  }
}
```

Or with the compiled binary:

```jsonc
{
  "mcpServers": {
    "tlapm": {
      "command": "/path/to/tlapm-mcp/bin/tlapm-mcp"
    }
  }
}
```

The caller provides exactly one of `line` or `range` in each `prove` / `race` tool call:

```jsonc
// One line -> tlapm --line N
{ "module": "Spec.tla", "line": 35, "solver": "z3", "use_fingerprints": false }

// Inclusive range -> tlapm --toolbox START END
{ "module": "Spec.tla", "range": { "start": 35, "end": 42 } }
```

### OMP (Oh My Pi)

Connect via OMP's MCP server config:

```jsonc
// ~/.omp/config.jsonc (or via `omp init`)
{
  "mcp": {
    "servers": {
      "tlapm": {
        "command": "go",
        "args": ["run", "./cmd/tlapm-mcp/"],
        "cwd": "/path/to/tlapm-mcp"
      }
    }
  }
}
```

**Tip:** `race` tries the fixed method list in the Solver Reference; it does not discover which methods are installed. `tlapm --config` can help diagnose the local TLAPM environment, but availability is reported by each race result.
