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

1. **Parses MCP requests** for `prove`, `race`, `list_theorems`, and `resolve_range`.
2. **Spawns `tlapm`** with the appropriate flags (`--solver`, `--timing`, `--nofp`, `--line`, etc.).
3. **Parses tlapm's structured output** (console text + fingerprint files).
4. **Returns typed JSON** with success/failure, timing breakdowns, and obligation details.

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

# Step parsing tests
go test ./internal/prover/ -run TestParseStep -v

# Integration tests (spawns tlapm)
go test ./internal/prover/ -run TestIntegration -v -count=1
```

Integration tests use `go:embed` to load TLA+ files from `internal/prover/testdata/`. The prover uses `fs.FS` for filesystem abstraction, so production code uses `os.DirFS(".")` directly while tests embed files.

### Example modules

| File | Source | Description |
|------|--------|-------------|
| `examples/NaturalNumbers.tla` | CommunityModules (MIT) | Standalone arithmetic module (Even/Odd theorems) |
| `internal/prover/testdata/arithmetic.tla` | CommunityModules (MIT) | Derived from NaturalNumbers; includes EvenPlusOddIsEven (failing) |
| `internal/prover/testdata/failing.tla` | CommunityModules (MIT) | Factorial theorems; FactorialGrows fails |
| `examples/proof_more_than_one_leader.tla` | raft spec | Lemma + theorem with nested subproof steps |


### Testing subproofs

```sh
# Target a specific step by line
tlapm --line 35 Spec.tla

# Target a step by path (the server constructs this from the step field)
tlapm --timing --solver z3 --line 35 Spec.tla
```

## Style

- Keep the MCP interface stable. New fields in `Output` are additive.
- Error messages should be human-readable and machine-parseable.
- Timing should always be in seconds (floats), never milliseconds.
- Use the `tlapm` native output format — don't reinvent parsing unless necessary.
