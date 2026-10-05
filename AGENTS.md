# Preface

This repository hosts tlapm-mcp, a Go MCP server that wraps the TLA⁺ Proof Manager (`tlapm`) binary to let LLMs reason about TLA⁺ proofs interactively. This `AGENTS.md` is the root index into the dispersed knowledge base; read it to orient yourself, then follow the links below for details.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

tlapm-mcp exposes two tools, `prove` and `race`. Both accept one source line or an inclusive line range and pass it directly to `tlapm` (`--line` or `--toolbox`). The MCP does not parse TLA+ source; it only parses tlapm output. The architecture is split across command entry points (thin), server setup, RPC handlers/tool definitions, and core tlapm orchestration.


# Important

- The server communicates over MCP stdio; tool I/O is JSON via stdin/stdout.
- All timing values are in seconds (floats), never milliseconds.
- Cached proof results are controlled by `cached` (`true` by default; `false` disables caching).
- Exactly one target form is required: `line`, or inclusive `from` and `to` values.


# Directory

- `cmd/` - Binary entry point for the MCP server.
- `internal/` - Core internal packages: server setup, RPC handlers, and prover logic.
- `testcases/` - Copy of testdata for manual testing.
- `go.mod` - Go module definition.
- `go.sum` - Go dependency checksums.
- `README.md` - Project readme with tool schemas and examples.
- `CONTRIBUTING.md` - Contribution guide with Go-specific notes.
- `TODO.md` - Implementation todo list and acceptance criteria.


# Documents

- `.kb/agents.md` - General rules for the knowledge base reading and writing.
- `cmd/AGENTS.md` - CLI entry points and server bootstrap.
- `internal/AGENTS.md` - Core internal packages overview.
- `internal/server/AGENTS.md` - MCP server setup.
- `internal/rpc/AGENTS.md` - Tool handlers and definitions.
- `internal/prover/AGENTS.md` - TLA⁺ prover orchestration.
