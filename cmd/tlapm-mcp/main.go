// main is the MCP stdio entry point for the TLA+ Proof Manager server.
package main

import (
	"github.com/tlaplus/tlapm-mcp/internal/server"
)

func main() {
	server.Start()
}
