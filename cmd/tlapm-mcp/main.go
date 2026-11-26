// main is the MCP stdio entry point for the TLA+ Proof Manager server.
package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/tlaplus/tlapm-mcp/internal/prover"
	"github.com/tlaplus/tlapm-mcp/internal/server"
)

func main() {
	var includeDirs []string
	flag.Func("I", "add DIR to tlapm's library search path (repeatable)", func(value string) error {
		if value == "" {
			return fmt.Errorf("directory must not be empty")
		}
		includeDirs = append(includeDirs, value)
		return nil
	})
	flag.Parse()
	if flag.NArg() > 0 {
		log.Fatalf("unexpected arguments: %v", flag.Args())
	}

	p := prover.New()
	p.IncludeDirs = includeDirs
	server.Start(p)
}
