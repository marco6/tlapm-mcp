# Preface

This document serves as the local knowledge base index for the `cmd/` directory. It outlines the entry point for the `tlapm-mcp` MCP server.

Read the top-level `.kb/agents.md` file before continuing below.


# Overview

The `cmd/` directory contains the main binary entry point for the `tlapm-mcp` application. Instead of housing core business logic, this directory is responsible for initializing the server, wiring up the prover, and starting the MCP stdio transport. Core logic is deferred to the packages in `internal/`.


# Directory

- `tlapm-mcp/main.go` - Thin application bootstrap that delegates to `server.Start()`.

