# tlapm-mcp

An MCP server that wraps the [TLA⁺ Proof Manager (tlapm)](https://github.com/tlaplus/tlapm) to let LLMs reason about TLA⁺ proofs interactively.

## Quick start

The model can ask to prove a theorem with simple, natural-language-friendly instructions:

- **"Prove `Correctness`"** — runs the default prover on the theorem at the top level.
- **"Prove `Correctness/<1>`"** — proves only step `<1>` within theorem `Correctness`.
- **"Prove `Correctness/<1>/<2>`"** — proves a nested subproof.
- **"Prove with solver `z3`"** — uses Z3 instead of the default.
- **"Prove without cache"** — disables fingerprint/caching.
- **"Race all provers"** — fires every backend in parallel and returns the fastest success.

## MCP Tools

### `prove`

Prove a specific theorem or subproof step.

```jsonc
{
  "module": "/path/to/Spec.tla",   // required: TLA+ module file
  "step": "Correctness/<1>/<2>",  // optional: theorem + subproof path (e.g. "<1>/<2>", "<1>..<3>")
  "solver": "z3",                  // optional: specific prover
  "use_fingerprints": true,        // optional: use cached results (default)
  "threads": 1                     // optional: parallel threads per prover
}
```

**Returns:**

```jsonc
{
  "success": true,                 // did the proof succeed?
  "module": "Spec",                // module name
  "step": "Correctness/<1>/<2>",  // what was proved
  "solver": "z3",                  // which solver was used
  "total_time_seconds": 0.234,    // wall-clock time
  "timing": {                      // breakdown by operation
    "parsing": 0.012,
    "analysis": 0.003,
    "generation": 0.0,
    "simplification": 0.001,
    "formatting": 0.0,
    "interaction": 0.210,         // main proof work
    "checking": 0.0,
    "fp_loading": 0.005,
    "fp_saving": 0.0,
    "fp_compute": 0.0,
    "other": 0.003
  },
  "obligations": [                 // if success is false, details
    {
      "line": 56,
      "text": "ASSUME Number == Nat \\ {0}, NEW CONSTANT M, NEW CONSTANT N,",
      "status": "failed",
      "error": "Zenon error: exhausted search space"
    }
  ],
  "proof_text": "[INFO]: All 37 obligations proved.",
  "fingerprints_used": true
}
```

### `race`

Race all available provers on a module/theorem and return the fastest successful prover. This is about finding the winner, not proving everything — the server launches each solver in parallel, each against the same target, and reports every result sorted by time.

```jsonc
{
  "module": "/path/to/Spec.tla",
  "step": "Correctness",           // optional: target a specific theorem (whole theorem or subproof)
  "use_fingerprints": true,
  "threads": 2                     // max parallel prover invocations
}
```

**Returns:**

```jsonc
{
  "fastest": {
    "solver": "smt",
    "success": true,
    "total_time_seconds": 0.087
  },
  "results": [
    { "solver": "smt",   "success": true,  "time": 0.087, "obligations_failed": 0 },
    { "solver": "zenon", "success": true,  "time": 0.152, "obligations_failed": 0 },
    { "solver": "z3",    "success": false, "time": 1.203, "obligations_failed": 5, "error": "Zenon error: exhausted search space" },
    { "solver": "auto",  "success": true,  "time": 0.910, "obligations_failed": 0 },
    { "solver": "blast", "success": true,  "time": 1.102, "obligations_failed": 0 }
  ]
}
```

### `list_theorems`

List all provable targets (theorems, axioms, definitions) in a module, including subproof structure.

```jsonc
{
  "module": "/path/to/Spec.tla",
  "include_subproofs": true       // include nested step info
}
```

**Returns:**

```jsonc
{
  "module": "Euclid",
  "file": "/path/to/Spec.tla",
  "targets": [
    {
      "name": "InitProperty",
      "line": 35,
      "kind": "THEOREM",
      "has_subproofs": true,
      "subproof_path": "InitProperty/<1>/<2>"
    },
    {
      "name": "NextProperty",
      "line": 42,
      "kind": "THEOREM",
      "has_subproofs": true,
      "subproof_path": "NextProperty/<1>/<2>/<1>a/<2>1"
    },
    {
      "name": "Correctness",
      "line": 64,
      "kind": "THEOREM",
      "has_subproofs": true,
      "subproof_path": "Correctness/<1>.<2>.<3>"
    },
    {
      "name": "GCDProperty1",
      "line": 38,
      "kind": "AXIOM",
      "has_subproofs": false
    }
  ]
}
```


## Step / Subproof Notation

TLA+ proofs are hierarchical. The server supports two ways to target subproofs:

### Nested path (exact)

```
TheoremName
TheoremName/<1>              — step 1 within the theorem
TheoremName/<1>/<2>          — step 2 inside step 1
TheoremName/<1>/<2>/<3>      — deeper nesting
TheoremName/<*>              — all direct subproofs of TheoremName
```

### Range (from…to)

```
TheoremName/<1>..<3>         — prove steps 1 through 3 (in DFS traversal order)
TheoremName/<2>subfact..<3>  — from <2>subfact to <3>, including everything between
```

The `<from>..<to>` notation works as a **sequential range** through the proof tree. You specify a start and end step, and the server proves everything between them in DFS order. This is more granular than a simple range — you can pin exact steps rather than just numbers.

```
<1>factA.
  <2>subfact.
    <3>subsubfact1.
  <2>anothersubfact.
    <3>anothersubsubfact
<1>factb.
  <3>not_required_to_be_2

Correctness/<1>..<3>         → all steps from the first <1> to the last <3>
Correctness/<1>/<2>..<3>    → from the first <2> inside <1> to the last <3>
Correctness/<1>/<2>..<3>    → includes everything in between (subfact, subsubfact1, anothersubfact, anothersubsubfact)
```

Notably, `<3>` doesn't need to be a child of `<2>` — it can be a child of `<1>` or anywhere between the start and end.

### Line number targeting

```
line:35
```

### Step notation summary

| Pattern | Meaning |
|---------|---------|
| `Correctness` | Whole theorem |
| `Correctness/<1>` | Step `<1>` |
| `Correctness/<1>/<2>` | Nested step (2 inside 1) |
| `Correctness/<*>` | All direct subproofs |
| `Correctness/<1>..<3>` | Range: from `<1>` to `<3>` (DFS order) |
| `Correctness/<1>/<2>..<3>` | Range: from `<2>` to `<3>` inside `<1>` |

## Solver Reference
| Solver | Description |
|--------|-------------|
| `smt` | Default SMT solver (usually Z3 via SMTLIB) |
| `z3` | Direct Z3 |
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

When a proof fails, the `obligations` array contains the text of each unproven obligation, including line number and the error message from the solver. This lets an LLM inspect whether the failure is due to:

- A missing definition (`USE DEF X`)
- A solver timeout / search space exhaustion
- A malformed proof step
- A missing assumption

## Example interactions

```
> Prove `EvenPlusEvenIsEven` in `NaturalNumbers.tla`.

< {"module": "NaturalNumbers.tla", "step": "EvenPlusEvenIsEven", "solver": "z3", "use_fingerprints": true}

< { "success": true, "step": "EvenPlusEvenIsEven", "solver": "z3", "total_time_seconds": 0.019,
    "proof_text": "All 1 obligation proved." }

> Prove `EvenPlusEvenIsEven` without cache.

< {"module": "NaturalNumbers.tla", "step": "EvenPlusEvenIsEven", "use_fingerprints": false}

< { "success": true, "step": "EvenPlusEvenIsEven", "solver": "smt", "total_time_seconds": 0.018 }

> Race all provers on `arithmetic.tla`.

< {"module": "arithmetic.tla", "use_fingerprints": true}

< { "fastest": { "solver": "smt", "success": true, "time": 0.087 },
    "results": [ ... ] }

> Resolve range `<1>..<3>` in `proof_more_than_one_leader.tla`.

< {"module": "proof_more_than_one_leader.tla", "step": "MoreThanOneLeaderInvariant/<1>..<3>"}

< { "resolved_step": "MoreThanOneLeaderInvariant/<1>..<3>",
    "steps": ["<1>1", "<1>2", "<1>3"],
    "step_count": 3 }
```

---

*This project was developed with AI assistance using agentic workflows and a structured knowledge base (`.kb/agents.md`, `AGENTS.md`).*
