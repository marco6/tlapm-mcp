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
		"description": "Path to the TLA+ module file (absolute or relative to the server working directory).",
	}
	properties["solver"] = map[string]any{
		"type":        "string",
		"description": "SMT solver backend passed to tlapm --solver (for example, z3).",
	}
	properties["use_fingerprints"] = fingerprintModeProperty()
	properties["threads"] = map[string]any{
		"type":        "integer",
		"minimum":     1,
		"description": "Number of worker threads (positive integer).",
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
			return toolErrorResult(err)
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
	if value, exists := args["solver"]; exists {
		solver, ok := value.(string)
		if !ok {
			return prover.ProveArgs{}, fmt.Errorf("solver must be a string")
		}
		pa.Solver = solver
	}
	if value, exists := args["use_fingerprints"]; exists {
		mode, err := parseFingerprintMode(value)
		if err != nil {
			return prover.ProveArgs{}, err
		}
		pa.FPModes = mode
	}
	if value, exists := args["threads"]; exists {
		threads, err := positiveInteger(value, "threads")
		if err != nil {
			return prover.ProveArgs{}, err
		}
		pa.Threads = threads
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
		"description": "Path to the TLA+ module file (absolute or relative to the server working directory).",
	}
	properties["use_fingerprints"] = fingerprintModeProperty()
	properties["threads"] = map[string]any{
		"type":        "integer",
		"minimum":     1,
		"description": "Maximum parallel prover invocations (positive integer).",
	}
	return &mcp.Tool{
		Name:        "race",
		Description: "Try the supported TLAPM methods on one source line or an inclusive line range and return the fastest proved result.",
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
			return toolErrorResult(err)
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
	if value, exists := args["use_fingerprints"]; exists {
		mode, err := parseFingerprintMode(value)
		if err != nil {
			return prover.RaceArgs{}, err
		}
		ra.FPModes = mode
	}
	if value, exists := args["threads"]; exists {
		threads, err := positiveInteger(value, "threads")
		if err != nil {
			return prover.RaceArgs{}, err
		}
		ra.Threads = threads
	}
	return ra, nil
}
func toolErrorResult(err error) (*mcp.CallToolResult, error) {
	tErr, ok := err.(*prover.TLAPMError)
	if !ok {
		return nil, err
	}
	data, marshalErr := json.Marshal(tErr.ToMap())
	if marshalErr != nil {
		return nil, marshalErr
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(data)}},
		IsError: true,
	}, nil
}

func fingerprintModeProperty() map[string]any {
	return map[string]any{
		"anyOf": []any{
			map[string]any{"type": "boolean"},
			map[string]any{"type": "string", "enum": []any{"check"}},
		},
		"description": "Fingerprint mode: true (use cache), false (no cache), or \"check\" (validate versions).",
	}
}

func parseFingerprintMode(value any) (prover.FPMode, error) {
	switch mode := value.(type) {
	case bool:
		if !mode {
			return prover.FPNo, nil
		}
		return prover.FPDefault, nil
	case string:
		if mode == "check" {
			return prover.FPCheck, nil
		}
	}
	return prover.FPDefault, fmt.Errorf("use_fingerprints must be true, false, or \"check\"")
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
