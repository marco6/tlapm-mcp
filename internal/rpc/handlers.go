// Package rpc implements JSON-RPC tool handlers for the MCP server.
package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tlaplus/tlapm-mcp/internal/prover"
)

// ---------------------------------------------------------------------------
// prove
// ---------------------------------------------------------------------------

// ProveTool returns the MCP tool definition for prove.
func ProveTool() *mcp.Tool {
	properties := targetProperties()
	properties["module"] = map[string]any{
		"type":        "string",
		"description": "Path to the TLA+ module file.",
	}
	properties["solver"] = map[string]any{
		"type":        "string",
		"description": "Solver to use (e.g. z3, smt, zenon).",
	}
	properties["use_fingerprints"] = map[string]any{
		"type":        "union",
		"types":       []any{"boolean", "string"},
		"description": "Fingerprint mode: true (use cache), false (no cache), or \"check\" (validate versions).",
	}
	properties["threads"] = map[string]any{
		"type":        "integer",
		"description": "Number of worker threads.",
	}
	return &mcp.Tool{
		Name:        "prove",
		Description: "Prove one source line or an inclusive line range in a TLA+ module.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   []any{"module"},
			"oneOf":      targetAlternatives(),
		},
	}
}

// ProveHandler is the tool handler for prove.
func ProveHandler(p *prover.Prover) func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
}

// parseProveArgs extracts ProveArgs from tool call arguments.
func parseProveArgs(args map[string]any) (prover.ProveArgs, error) {
	module, ok := args["module"].(string)
	if !ok || module == "" {
		return prover.ProveArgs{}, fmt.Errorf("module is required")
	}
	target, err := parseLineTarget(args)
	if err != nil {
		return prover.ProveArgs{}, err
	}
	pa := prover.ProveArgs{Module: module, Target: target, FPModes: prover.FPDefault}

	if v, ok := args["solver"].(string); ok {
		pa.Solver = v
	}
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

// ---------------------------------------------------------------------------
// race
// ---------------------------------------------------------------------------

// RaceTool returns the MCP tool definition for race.
func RaceTool() *mcp.Tool {
	properties := targetProperties()
	properties["module"] = map[string]any{
		"type":        "string",
		"description": "Path to the TLA+ module file.",
	}
	properties["use_fingerprints"] = map[string]any{
		"type":        "union",
		"types":       []any{"boolean", "string"},
		"description": "Fingerprint mode: true (use cache), false (no cache), or \"check\" (validate versions).",
	}
	properties["threads"] = map[string]any{
		"type":        "integer",
		"description": "Max parallel prover invocations.",
	}
	return &mcp.Tool{
		Name:        "race",
		Description: "Race all available provers on one source line or an inclusive line range and return the fastest success.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   []any{"module"},
			"oneOf":      targetAlternatives(),
		},
	}
}

// RaceHandler is the tool handler for race.
func RaceHandler(p *prover.Prover) func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
}
func parseRaceArgs(args map[string]any) (prover.RaceArgs, error) {
	module, ok := args["module"].(string)
	if !ok || module == "" {
		return prover.RaceArgs{}, fmt.Errorf("module is required")
	}
	target, err := parseLineTarget(args)
	if err != nil {
		return prover.RaceArgs{}, err
	}
	ra := prover.RaceArgs{Module: module, Target: target, FPModes: prover.FPDefault, Threads: 2}
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

func targetProperties() map[string]any {
	return map[string]any{
		"line": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "One source line; passed to tlapm as --line N.",
		},
		"range": map[string]any{
			"type":        "object",
			"description": "Inclusive source line range; passed to tlapm as --toolbox START END.",
			"properties": map[string]any{
				"start": map[string]any{"type": "integer", "minimum": 1},
				"end":   map[string]any{"type": "integer", "minimum": 1},
			},
			"required":             []any{"start", "end"},
			"additionalProperties": false,
		},
	}
}

func targetAlternatives() []any {
	return []any{
		map[string]any{
			"required": []any{"line"},
			"not":      map[string]any{"required": []any{"range"}},
		},
		map[string]any{
			"required": []any{"range"},
			"not":      map[string]any{"required": []any{"line"}},
		},
	}
}

func parseLineTarget(args map[string]any) (prover.LineTarget, error) {
	lineValue, hasLine := args["line"]
	rangeValue, hasRange := args["range"]
	if hasLine == hasRange {
		return prover.LineTarget{}, fmt.Errorf("provide exactly one of line or range")
	}
	if hasLine {
		line, err := positiveInteger(lineValue, "line")
		if err != nil {
			return prover.LineTarget{}, err
		}
		return prover.LineTarget{Line: line}, nil
	}
	rangeMap, ok := rangeValue.(map[string]any)
	if !ok {
		return prover.LineTarget{}, fmt.Errorf("range must contain start and end line numbers")
	}
	start, err := positiveInteger(rangeMap["start"], "range.start")
	if err != nil {
		return prover.LineTarget{}, err
	}
	end, err := positiveInteger(rangeMap["end"], "range.end")
	if err != nil {
		return prover.LineTarget{}, err
	}
	if end < start {
		return prover.LineTarget{}, fmt.Errorf("range.end must not be before range.start")
	}
	return prover.LineTarget{Range: &prover.LineRange{Start: start, End: end}}, nil
}

func positiveInteger(value any, name string) (int, error) {
	n, ok := value.(float64)
	if !ok || n < 1 || n != float64(int(n)) {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return int(n), nil
}
