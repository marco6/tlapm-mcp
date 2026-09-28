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

1. **Parses MCP requests** for `prove`, `race`, and `list_theorems`.
2. **Spawns `tlapm`** with the appropriate flags (`--solver`, `--timing`, `--nofp`, `--line`, etc.).
3. **Parses tlapm's structured output** (console text + fingerprint files).
4. **Returns typed JSON** with success/failure, timing breakdowns, and obligation details.

## Adding a new tool

1. Add a handler in the tools registry.
2. Define its `Input` and `Output` types.
3. Wire it in the MCP protocol loop.

## Adding solver support

New backends are detected automatically by tlapm's `--config` output. To add a solver:

1. Ensure the binary is on `$PATH`.
2. Run `tlapm --method help` to confirm it's listed.
3. Add it to the `available_solvers` set in the server.

## Testing

### Manual testing

```sh
# Run the server instdio mode
python -m tlapm_mcp

# Or with a custom tlapm path
TLAPM_PATH=/path/to/tlapm python -m tlapm_mcp
```

### Expected outputs

- `prove` on a passing theorem → `{"success": true, "solver": "...", "total_time_seconds": 0.X}`
- `prove` on a failing theorem → `{"success": false, "obligations": [{...}]}`
- `race` → sorted results with `fastest` field
- `list_theorems` → full target list with subproof paths


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
