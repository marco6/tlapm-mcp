# Preface

This document describes the scope of the `internal/rpc/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the tool handlers and definitions.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The `rpc/` package contains all MCP tool handlers, tool definitions, and argument parsing functions. Each tool (`prove`, `race`, `list_theorems`, `resolve_range`) has its own handler function, tool definition, and arg parser, all organized in a single file for simplicity.


# Directory

- `handlers.go` - Tool definitions (`ProveTool()`, `RaceTool()`, etc.), handler functions that close over `*prover.Prover`, and argument parsing functions (`parseProveArgs()`, `parseRaceArgs()`, etc.).

