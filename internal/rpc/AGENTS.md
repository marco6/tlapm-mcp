# Preface

This document describes the scope of the `internal/rpc/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the tool handlers and definitions.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The `rpc/` package contains the `check`, `prove`, and `race` tool definitions, handlers, and argument parsers. All require a module; `check` accepts an optional target, while `prove` and `race` require either one numeric `line` or an inclusive `from`/`to` pair. Invalid inputs and TLAPM failures are returned with stable error codes in JSON.


# Directory

- `handlers.go` - `check`/`prove`/`race` definitions and handlers, target schema helpers, and argument parsing.
