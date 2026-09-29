// Package server sets up and runs the MCP stdio server.
package server

import (
	"context"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tlaplus/tlapm-mcp/internal/prover"
	"github.com/tlaplus/tlapm-mcp/internal/rpc"
)

// Server wraps the MCP server and prover.
type Server struct {
	mcpServer *mcp.Server
	prover    *prover.Prover
}

// New creates a new server with the given prover.
func New(p *prover.Prover) *Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "tlapm-mcp", Version: "0.1.0"}, nil)
	return &Server{mcpServer: srv, prover: p}
}

// RegisterTools registers all MCP tools with their handlers.
func (s *Server) RegisterTools() {
	s.mcpServer.AddTool(rpc.ProveTool(), rpc.ProveHandler(s.prover))
	s.mcpServer.AddTool(rpc.RaceTool(), rpc.RaceHandler(s.prover))
	s.mcpServer.AddTool(rpc.ListTheoremsTool(), rpc.ListTheoremsHandler(s.prover))
	s.mcpServer.AddTool(rpc.ResolveRangeTool(), rpc.ResolveRangeHandler(s.prover))
}

// Run starts the MCP stdio server.
func (s *Server) Run(ctx context.Context) error {
	return s.mcpServer.Run(ctx, &mcp.StdioTransport{})
}

// Start creates a server, registers tools, and runs it.
func Start() {
	p := prover.New()
	srv := New(p)
	srv.RegisterTools()
	if err := srv.Run(context.Background()); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
