# Preface

This document describes the scope of the `internal/rpc/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the tool handlers and definitions.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The `rpc/` package contains the `prove` and `race` tool definitions, handlers, and argument parsers. Both schemas require a module plus either one numeric `line` or an inclusive `from`/`to` pair.


# Directory

- `handlers.go` - `prove`/`race` definitions and handlers, flattened target schema helpers, and argument parsing.

