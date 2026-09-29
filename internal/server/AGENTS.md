# Preface

This document describes the scope of the `internal/server/` directory within the tlapm-mcp repository, providing context for automated agents navigating and modifying the MCP server setup.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The `server/` package contains the MCP server setup logic. It creates the MCP server instance, registers all tools with their handlers, and manages the stdio transport for client communication.


# Directory

- `server.go` - Server setup: `New()`, `RegisterTools()`, `Run()`, and the `Start()` entry point.

