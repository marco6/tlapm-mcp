# Contributing to tlapm-mcp

## Architecture

```
┌─────────────┐     MCP JSON-RPC      ┌──────────────┐
│   LLM /     │ ◄──────────────────► │   tlapm-mcp  │
│   Editor    │                      │   Server     │
└─────────────┘                      └──────┬───────┘
                                            │
                                      tlapm CLI
                                      (native binary)
```

The server is a thin wrapper around the `tlapm` binary. It:

1. **Parses MCP requests** for `prove` and `race`.
2. **Passes numeric targets to `tlapm`** using `--line N` for one line or `--toolbox START END` for an inclusive line range.
3. **Does not parse TLA+ source files**; `tlapm` handles source parsing.
4. **Parses tlapm output** and returns typed JSON with success/failure, timing, and obligation details.

## Adding a new tool

1. Register the tool in `cmd/tlapm-mcp/main.go` with a `Tool` definition (name, description, `InputSchema`) and a handler function `func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error)`.
2. Add argument parsing in the same file (`parseXxxArgs`).
3. Wire it via `server.AddTool()`.

## Adding solver support

New backends are detected automatically by `raceSolvers()` in `internal/prover/result.go`, which reads tlapm's `--config` output. To add a solver:

1. Ensure the binary is on `$PATH`.
2. Run `tlapm --method help` to confirm it's listed.
3. The server will auto-detect it; no code change needed.

## Testing

### Unit tests

```sh
# All tests
go test ./...

# Native line/range targeting tests
go test ./internal/prover/ -run 'TestProve_RangeTarget|TestRace_RangeTarget' -v -count=1

# Integration tests (spawns tlapm)
go test ./internal/prover/ -run TestIntegration -v -count=1
```

Integration tests embed fixtures to exercise module-path validation. The prover checks paths with `fs.FS` but never reads module contents; `tlapm` itself opens and parses the source.

### Example modules

| File | Source | Description |
|------|--------|-------------|
| `examples/NaturalNumbers.tla` | CommunityModules (MIT) | Standalone arithmetic module (Even/Odd theorems) |
| `internal/prover/testdata/arithmetic.tla` | CommunityModules (MIT) | Derived from NaturalNumbers; includes EvenPlusOddIsEven (failing) |
| `internal/prover/testdata/failing.tla` | CommunityModules (MIT) | Factorial theorems; FactorialGrows fails |
| `examples/proof_more_than_one_leader.tla` | raft spec | Lemma + theorem with nested subproof steps |


### Targeting source lines

Tool calls use one-based source lines. Provide exactly one line or inclusive range:

```sh
# One line
tlapm --line 35 Spec.tla

# Inclusive line range
tlapm --toolbox 35 42 Spec.tla
```

The MCP passes these arguments directly to `tlapm`; it never scans module contents.

## Style

- Keep the MCP interface stable. New fields in `Output` are additive.
- Error messages should be human-readable and machine-parseable.
- Timing should always be in seconds (floats), never milliseconds.
- Never parse module source in MCP code; parse only `tlapm` output.
