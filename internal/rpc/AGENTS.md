# Preface

This document describes the scope of the `internal/rpc/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the tool handlers and definitions.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The `rpc/` package contains the `prove` and `race` tool definitions, handlers, and argument parsers. Both schemas require a module plus exactly one numeric `line` or inclusive `range` (`start`/`end`).


# Directory

- `handlers.go` - `prove`/`race` definitions and handlers, line/range schema helpers, and argument parsing.

