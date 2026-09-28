// main is the MCP stdio entry point for the TLA+ Proof Manager server.
package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tlaplus/tlapm-mcp/internal/prover"
)

var p *prover.Prover

func main() {
	p = prover.New()
	server := mcp.NewServer(&mcp.Implementation{Name: "tlapm-mcp", Version: "0.1.0"}, nil)

	server.AddTool(proveTool(), proveHandler)
	server.AddTool(raceTool(), raceHandler)
	server.AddTool(listTheoremsTool(), listTheoremsHandler)
	server.AddTool(resolveRangeTool(), resolveRangeHandler)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

// proveTool returns the MCP tool definition for prove.
func proveTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "prove",
		Description: "Prove a specific theorem or subproof step in a TLA+ module.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"module": map[string]any{
					"type":        "string",
					"description": "Path to the TLA+ module file.",
				},
				"step": map[string]any{
					"type":        "string",
					"description": "Theorem or subproof path.",
				},
				"solver": map[string]any{
					"type":        "string",
					"description": "Solver to use (e.g. z3, smt, zenon).",
				},
				"use_fingerprints": map[string]any{
					"type":          "union",
					"types":         []any{"boolean", "string"},
					"description":   "Fingerprint mode: true (use cache), false (no cache), or \"check\" (validate versions).",
				},
				"threads": map[string]any{
					"type":        "integer",
					"description": "Number of worker threads.",
				},
			},
			"required": []any{"module"},
		},
	}
}

// proveHandler is the tool handler for prove.
func proveHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, err
	}
	pa, err := parseProveArgs(args)
	if err != nil {
		return nil, err
	}
	result, err := p.Prove(ctx, pa)
	if err != nil {
		// If it's a structured TLAPMError, include it in the result and also return it
		if tErr, ok := err.(*prover.TLAPMError); ok {
			result.ErrorCode = string(tErr.Code)
			result.ProofText = tErr.Message
			data, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				return nil, marshalErr
			}
			return &mcp.CallToolResult{
				Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
			}, tErr
		}
		return nil, err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

// raceTool returns the MCP tool definition for race.
func raceTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "race",
		Description: "Race all available provers on a module/theorem and return the fastest success.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"module": map[string]any{
					"type":        "string",
					"description": "Path to the TLA+ module file.",
				},
				"step": map[string]any{
					"type":        "string",
					"description": "Target theorem or subproof.",
				},
				"use_fingerprints": map[string]any{
					"type":          "union",
					"types":         []any{"boolean", "string"},
					"description":   "Fingerprint mode: true (use cache), false (no cache), or \"check\" (validate versions).",
				},
				"threads": map[string]any{
					"type":        "integer",
					"description": "Max parallel prover invocations.",
				},
			},
			"required": []any{"module"},
		},
	}
}

// raceHandler is the tool handler for race.
func raceHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, err
	}
	ra, err := parseRaceArgs(args)
	if err != nil {
		return nil, err
	}
	result, err := p.Race(ctx, ra)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

// listTheoremsTool returns the MCP tool definition for list_theorems.
func listTheoremsTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "list_theorems",
		Description: "List all provable targets in a TLA+ module.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"module": map[string]any{
					"type":        "string",
					"description": "Path to the TLA+ module file.",
				},
				"include_subproofs": map[string]any{
					"type":        "boolean",
					"description": "Include nested subproof information.",
				},
			},
			"required": []any{"module"},
		},
	}
}

// listTheoremsHandler is the tool handler for list_theorems.
func listTheoremsHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, err
	}
	la, err := parseListTheoremsArgs(args)
	if err != nil {
		return nil, err
	}
	result, err := p.ListTheorems(ctx, la)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

// parseProveArgs extracts ProveArgs from tool call arguments.
func parseProveArgs(args map[string]any) (prover.ProveArgs, error) {
	pa := prover.ProveArgs{FPModes: prover.FPDefault}

	if v, ok := args["module"].(string); ok {
		pa.Module = v
	} else {
		return pa, nil
	}
	if v, ok := args["step"].(string); ok {
		pa.Step = v
	}
	if v, ok := args["solver"].(string); ok {
		pa.Solver = v
	}
	// Handle use_fingerprints as boolean or string ("check")
	if v, ok := args["use_fingerprints"].(bool); ok {
		if !v {
			pa.FPModes = prover.FPNo
		}
	} else if v, ok := args["use_fingerprints"].(string); ok {
		switch v {
		case "check":
			pa.FPModes = prover.FPCheck
		case "no", "false":
			pa.FPModes = prover.FPNo
		default:
			pa.FPModes = prover.FPDefault
		}
	}
	if v, ok := args["threads"].(float64); ok {
		pa.Threads = int(v)
	}
	return pa, nil
}

// parseRaceArgs extracts RaceArgs from tool call arguments.
func parseRaceArgs(args map[string]any) (prover.RaceArgs, error) {
	ra := prover.RaceArgs{FPModes: prover.FPDefault, Threads: 2}

	if v, ok := args["module"].(string); ok {
		ra.Module = v
	} else {
		return ra, nil
	}
	if v, ok := args["step"].(string); ok {
		ra.Step = v
	}
	// Handle use_fingerprints as boolean or string ("check")
	if v, ok := args["use_fingerprints"].(bool); ok {
		if !v {
			ra.FPModes = prover.FPNo
		}
	} else if v, ok := args["use_fingerprints"].(string); ok {
		switch v {
		case "check":
			ra.FPModes = prover.FPCheck
		case "no", "false":
			ra.FPModes = prover.FPNo
		default:
			ra.FPModes = prover.FPDefault
		}
	}
	if v, ok := args["threads"].(float64); ok {
		ra.Threads = int(v)
	}
	return ra, nil
}
// parseListTheoremsArgs extracts ListTheoremsArgs from tool call arguments.
func parseListTheoremsArgs(args map[string]any) (prover.ListTheoremsArgs, error) {
	la := prover.ListTheoremsArgs{}

	if v, ok := args["module"].(string); ok {
		la.Module = v
	} else {
		return la, nil
	}
	if v, ok := args["include_subproofs"].(bool); ok {
		la.IncludeSubproofs = v
	}
	return la, nil
}

// resolveRangeTool returns the MCP tool definition for resolve_range.
func resolveRangeTool() *mcp.Tool {
	return &mcp.Tool{
		Name:        "resolve_range",
		Description: "Resolve a range step (e.g. <1>..<3>) against the DFS tree of proof steps in a TLA+ module.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"module": map[string]any{
					"type":        "string",
					"description": "Path to the TLA+ module file.",
				},
				"step": map[string]any{
					"type":        "string",
					"description": "Range step selector, e.g. 'Correctness/<1>..<3>'.",
				},
			},
			"required": []any{"module", "step"},
		},
	}
}

// resolveRangeHandler resolves a range step against the DFS proof tree.
func resolveRangeHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args map[string]any
	if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
		return nil, err
	}
	ra, err := parseResolveRangeArgs(args)
	if err != nil {
		return nil, err
	}
	result, err := p.ResolveRange(ctx, ra)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
	}, nil
}

// parseResolveRangeArgs extracts ResolveRangeArgs from tool call arguments.
func parseResolveRangeArgs(args map[string]any) (prover.ResolveRangeArgs, error) {
	ra := prover.ResolveRangeArgs{}

	if v, ok := args["module"].(string); ok {
		ra.Module = v
	}
	if v, ok := args["step"].(string); ok {
		ra.Step = v
	}
	return ra, nil
}
