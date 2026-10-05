# tlapm-mcp

An MCP server that wraps the [TLA⁺ Proof Manager (tlapm)](https://github.com/tlaplus/tlapm) to let LLMs reason about TLA⁺ proofs interactively.

## Quick start

Requirements: Go 1.24+ and the `tlapm` executable available on `PATH`.

Build the server from the repository root:

```bash
go build -o tlapm-mcp ./cmd/tlapm-mcp/
```

Register it with an MCP client. For example, in OpenCode's `.opencode/config.jsonc`:

```jsonc
{
  "mcpServers": {
    "tlapm": {
      "command": "/absolute/path/to/tlapm-mcp/tlapm-mcp",
      "cwd": "/absolute/path/to/tlapm-mcp"
    }
  }
}
```

Replace both paths with the repository location. Then ask the client: “Prove lines 6 through 8 of `testcases/arithmetic_theorem.tla`.” The server should report that 1 obligation was proved. See [MCP Tools](#mcp-tools) for request fields and response details.

## MCP Tools

### `check`

Parse and elaborate a module without launching proof backends. Targeting is optional: omit `line`, `from`, and `to` to check the whole module, or provide one line or an inclusive range.

```jsonc
{
  "module": "/path/to/Spec.tla"
}
```

```jsonc
{
  "module": "/path/to/Spec.tla",
  "from": 28,
  "to": 34
}
```

The check runs TLAPM in no-backend summary mode. `success` means parsing and elaboration completed; it does not mean the proof obligations were proved. Diagnostics contain `line`, `column`, `severity`, and `message`; a zero line or column means TLAPM did not report that location. `obligation_count` is included when TLAPM reports it. For an abnormal TLAPM exit, the response also includes `exit_code` and `stderr`.

```jsonc
{
  "success": true,
  "module": "/path/to/Spec.tla",
  "range": { "start": 28, "end": 34 },
  "diagnostics": [],
  "obligation_count": 2
}
```

### `prove`

Prove one source line or an inclusive source line range. Provide either `line` or both `from` and `to`.

```jsonc
{
  "module": "/path/to/Spec.tla",       // required: absolute or server-working-directory-relative module path
  "line": 28,                           // one-based source line; alternatively use from and to
  "cached": true                        // optional: use cached proof results (default)
}
```

For an inclusive range, replace `line` with `from` and `to`:

```jsonc
{
  "module": "/path/to/Spec.tla",       // absolute or server-working-directory-relative path
  "from": 28,
  "to": 34
}
```

Single lines use `--line N`; ranges use `--toolbox FROM TO`.

**Returns:**

```jsonc
{
  "success": false,
  "module": "Spec",
  "line": 28,                         // or "range": { "start": 28, "end": 34 }
  "total_time_seconds": 0.234,
  "timing": { "interaction": 0.210 },
  "obligation_count": 1,
  "obligations": [
    {
      "line": 8,
      "status": "failed",
      "failure_reason": "false"
    }
  ],
  "proof_text": "[ERROR]: 1/1 obligation failed."
}
```

The server reports `success: true` only when `tlapm` exits successfully and emits an aggregate `[INFO]: All N obligations proved.` line with `N > 0`. A zero-obligation summary is returned as `success: false` with `error_code: "NO_OBLIGATIONS"`; it does not confirm that the selected target was proved. A message that only says an individual obligation was proved is not sufficient. `obligation_count` is the total generated count. `obligations` contains only unresolved obligations, with their source line, status, and a failure reason when available.

### `race`

Try each method in the fixed method list documented below on one source line or an inclusive line range. Each candidate is passed to TLAPM with `--method`; at most two calls run in parallel, and each method reports its own result, including unavailable methods. An explicit `BY` method in the TLA⁺ proof takes precedence over this default, so `race` only compares candidates for obligations without an explicit proof method.

```jsonc
{
  "module": "/path/to/Spec.tla",
  "line": 28,                           // alternatively use "from": 28, "to": 34
  "cached": true
}
```

`line` is passed as `--line N`; `from` and `to` are passed as `--toolbox FROM TO`.

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
    { "solver": "z3",    "success": false, "total_time_seconds": 1.203, "obligations_failed": 1, "error": "TLAPM output did not confirm that all obligations were proved", "obligations": [{ "line": 8, "status": "failed", "failure_reason": "false" }] }
  ],
  "obligation_count": 1
}
```

`obligations_failed` is `-1` when the wrapper cannot establish a count. If no method proves the target, `fastest` contains the fastest failed result; it is not a successful proof.

The response keeps `solver` as the result-field name for compatibility; its value identifies the attempted TLAPM method.

## Line and range targeting

Targets are one-based source line numbers, not theorem names or proof-step paths. Supply exactly one target form:

```jsonc
{ "module": "Spec.tla", "line": 35 }
{ "module": "Spec.tla", "from": 35, "to": 42 }
```

- `line` selects one source line and becomes `tlapm --line N`.
- `from` and `to` select an inclusive range and become `tlapm --toolbox FROM TO`.

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

## Cached Results

tlapm caches proof results in fingerprint files (`.tlacache/`). Cached results are used by default:

- **`cached: true`** (default) — uses cached results and skips proven obligations.
- **`cached: false`** — ignores existing cached results and recomputes, useful after definition changes.

## Obligation diagnostics

`prove` and `race` include only unresolved obligations. Each contains a source line and status, with a failure reason when available; successful obligations are represented by the aggregate counts and result status.

## Example interactions

```
> Prove lines 6 through 8 of `testcases/arithmetic_theorem.tla`.

< {"module": "testcases/arithmetic_theorem.tla", "from": 6, "to": 8}

< {"success": true, "module": "arithmetic_theorem", "range": {"start": 6, "end": 8}, "proof_text": "All 1 obligation proved."}
```

## Build & Editor Setup

### Building

```bash
# Ensure Go 1.24+ is available
go version

# Build the binary at the repository root
go build -o tlapm-mcp ./cmd/tlapm-mcp/

# Or run directly without building
go run ./cmd/tlapm-mcp/
```

The binary communicates over MCP stdio as a long-lived process. Every `prove` and `race` call requires exactly one of `line` or inclusive `from`/`to`; proof method selection belongs in the module's `BY` clause.

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
      "command": "/path/to/tlapm-mcp/tlapm-mcp",
      "cwd": "/path/to/tlapm-mcp"
    }
  }
}
```

The caller provides exactly one of `line` or inclusive `from`/`to` in each `prove` / `race` tool call:

```jsonc
// One line -> tlapm --line N
{ "module": "Spec.tla", "line": 35, "cached": false }

// Inclusive range -> tlapm --toolbox FROM TO
{ "module": "Spec.tla", "from": 35, "to": 42 }
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
