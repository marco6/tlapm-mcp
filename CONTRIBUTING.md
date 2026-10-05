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
2. **Passes numeric targets to `tlapm`** using `--line N` for one line or `--toolbox FROM TO` for an inclusive range supplied as `from`/`to`.
3. **Does not parse TLA+ source files**; `tlapm` handles source parsing.
4. **Reports proof status conservatively**: success requires a zero TLAPM exit status and an explicit aggregate “all obligations proved” summary.

## Adding a new tool

1. Add the tool definition and argument parsing in `internal/rpc/handlers.go`.
2. Register the tool and handler in `internal/server/server.go`.

## Adding solver support

The proof method or solver is selected in the TLA+ proof's `BY` clause; the `prove` tool does not override it. `race` tries a fixed set of TLAPM methods in `internal/prover/prover.go`, passing each as `--method`; it does not read `tlapm --config` or auto-detect installed methods.

1. Ensure the relevant TLAPM method and dependencies are on `$PATH`.
2. Add or remove method names in the `Prover.Race` list when changing the set `race` tries.
3. Run `race` to see per-method success and availability results.

## Testing

### Unit tests

```sh
# Full tests; integration tests require a working tlapm executable
go test ./...

# Native line/range targeting and non-TLAPM safety tests
go test ./internal/prover/ -run 'TestLineTargetCommandArgs|TestBuildProveCmdUsesTLAPMDefaults|TestEnsureModuleExistsAcceptsAbsolutePath' -v -count=1

# Integration tests (spawns tlapm)
go test ./internal/prover/ -run TestIntegration -v -count=1
```

Integration tests embed fixtures and invoke TLAPM for proof scenarios. They require the native binary to be installed; without it, tests that expect proved results fail as environment errors.

### Targeting source lines

Tool calls use one-based source lines and accept absolute module paths or paths relative to the server working directory. Provide either one `line` or both `from` and `to` in an MCP call:

```jsonc
{ "module": "Spec.tla", "line": 35 }
{ "module": "Spec.tla", "from": 35, "to": 42 }
```

`tlapm` receives a single line as `--line N` and an inclusive range as `--toolbox FROM TO`.

The MCP passes these arguments directly to `tlapm`; it never scans module contents.

## Style

- Keep the MCP interface stable. New fields in `prover.Result` should be additive.
- Error messages should be human-readable and machine-parseable.
- Timing should always be in seconds (floats), never milliseconds.
- Never parse module source in MCP code; parse only `tlapm` output.
