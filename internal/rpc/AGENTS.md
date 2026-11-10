# Preface

This document describes the scope of the `internal/rpc/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the tool handlers and definitions.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The `rpc/` package contains the `check` and `prove` tool definitions, handlers, and argument parsers. Both require a module; `check` always checks the whole module, while `prove` requires either one numeric `line` or an inclusive `from`/`to` pair. Invalid inputs and TLAPM failures are returned with stable error codes in JSON.


# Directory

- `handlers.go` - `check`/`prove` definitions and handlers, target schema helpers, and argument parsing.
