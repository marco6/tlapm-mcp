# Preface

This repository hosts tlapm-mcp, a Go MCP server that wraps the TLA⁺ Proof Manager (`tlapm`) binary to let LLMs reason about TLA⁺ proofs interactively. This `AGENTS.md` is the root index into the dispersed knowledge base; read it to orient yourself, then follow the links below for details.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

tlapm-mcp exposes three tools: `check`, `prove`, and `race`. `check` parses and elaborates a whole module or optional source target without launching backends; `prove` and `race` require one source line or an inclusive line range. The MCP does not parse TLA+ source; it parses TLAPM output. The architecture is split across command entry points (thin), server setup, RPC handlers/tool definitions, and core TLAPM orchestration.


# Important

- The server communicates over MCP stdio; tool I/O is JSON via stdin/stdout.
- All timing values are in seconds (floats), never milliseconds.
- Cached proof results are controlled by `cached` (`true` by default; `false` disables caching).
- `prove` and `race` require exactly one target form: `line`, or inclusive `from` and `to` values. `check` accepts no target or either target form.


# Directory

- `cmd/` - Binary entry point for the MCP server.
- `internal/` - Core internal packages: server setup, RPC handlers, and prover logic.
- `testcases/` - Copy of testdata for manual testing.
- `go.mod` - Go module definition.
- `go.sum` - Go dependency checksums.
- `README.md` - Project readme with tool schemas and examples.
- `CONTRIBUTING.md` - Contribution guide with Go-specific notes.

# Documents

- `.kb/agents.md` - General rules for the knowledge base reading and writing.
- `.kb/ci.md` - CI workflow configuration and testing pipeline.

- `cmd/AGENTS.md` - CLI entry points and server bootstrap.
- `internal/AGENTS.md` - Core internal packages overview.
- `internal/server/AGENTS.md` - MCP server setup.
- `internal/rpc/AGENTS.md` - Tool handlers and definitions.
- `internal/prover/AGENTS.md` - TLA⁺ prover orchestration.
