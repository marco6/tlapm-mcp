# Preface

This repository hosts tlapm-mcp, a Go MCP server that wraps the TLA⁺ Proof Manager (`tlapm`) binary to let LLMs reason about TLA⁺ proofs interactively. This `AGENTS.md` is the root index into the dispersed knowledge base; read it to orient yourself, then follow the links below for details.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

tlapm-mcp exposes three core tools — `prove`, `race`, and `list_theorems` — plus a `resolve_range` utility tool. The server is a thin wrapper around the `tlapm` CLI binary, spawning it with appropriate flags (`--solver`, `--timing`, `--nofp`, `--line`) and parsing its structured output into typed JSON. The architecture is split across command entry points (thin), a server layer (MCP setup), an RPC layer (handlers and tool definitions), and the prover package (core tlapm orchestration).


# Important

- The server communicates over MCP stdio; all tool I/O is JSON via stdin/stdout.
- All timing values are in seconds (floats), never milliseconds.
- Fingerprint/caching is controlled via `use_fingerprints` accepting `true`, `false`, or `"check"`.
- Step notation supports both nested paths (`Theorem/<1>/<2>`) and DFS ranges (`Theorem/<1>..<3>`).


# Directory

- `cmd/` - Binary entry point for the MCP server.
- `internal/` - Core internal packages: server setup, RPC handlers, and prover logic.
- `examples/` - TLA⁺ example modules (both raft and non-raft).
- `testcases/` - Copy of testdata for manual testing.
- `go.mod` - Go module definition.
- `go.sum` - Go dependency checksums.
- `README.md` - Project readme with tool schemas and examples.
- `CONTRIBUTING.md` - Contribution guide with Go-specific notes.
- `TODO.md` - Implementation todo list and acceptance criteria.


# Documents

- `.kb/agents.md` - General rules for the knowledge base reading and writing.
- `.kb/ci.md` - CI workflow configuration and testing pipeline.

- `cmd/AGENTS.md` - CLI entry points and server bootstrap.
- `internal/AGENTS.md` - Core internal packages overview.
- `internal/server/AGENTS.md` - MCP server setup.
- `internal/rpc/AGENTS.md` - Tool handlers and definitions.
- `internal/prover/AGENTS.md` - TLA⁺ prover orchestration.
- `examples/AGENTS.md` - Example TLA⁺ modules.

