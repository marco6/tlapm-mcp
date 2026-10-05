# Preface

This document describes the scope of the `internal/server/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the MCP server setup.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The `server/` package creates the MCP server instance, registers the `check`, `prove`, and `race` tools, and manages stdio transport.


# Directory

- `server.go` - Server setup: `New()`, `RegisterTools()`, `Run()`, and the `Start()` entry point.
