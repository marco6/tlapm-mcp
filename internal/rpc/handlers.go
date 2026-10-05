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
// check
// ---------------------------------------------------------------------------

// CheckTool returns the MCP tool definition for check.
func CheckTool() *mcp.Tool {
	properties := targetProperties()
	properties["module"] = map[string]any{
		"type":        "string",
		"description": "Path to the TLA+ module file (absolute or relative to the server working directory).",
	}
	return &mcp.Tool{
		Name:        "check",
		Description: "Parse and elaborate a TLA+ module without running proof backends. An optional line or inclusive range limits the check.",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": properties,
			"required":   []any{"module"},
			"oneOf":      optionalTargetAlternatives(),
		},
	}
}

// CheckHandler is the tool handler for check.
func CheckHandler(p *prover.Prover) func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var args map[string]any
		if err := json.Unmarshal(req.Params.Arguments, &args); err != nil {
			return nil, err
		}
		ca, err := parseCheckArgs(args)
		if err != nil {
			return nil, err
		}
		result, err := p.Check(ctx, ca)
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

func parseCheckArgs(args map[string]any) (prover.CheckArgs, error) {
	module, ok := args["module"].(string)
	if !ok || module == "" {
		return prover.CheckArgs{}, fmt.Errorf("module is required")
	}
	target, err := parseOptionalLineTarget(args)
	if err != nil {
		return prover.CheckArgs{}, err
	}
	return prover.CheckArgs{Module: module, Target: target}, nil
}

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
	properties["cached"] = cachedProperty()
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
	if value, exists := args["cached"]; exists {
		mode, err := parseCached(value)
		if err != nil {
			return prover.ProveArgs{}, err
		}
		pa.FPModes = mode
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
	properties["cached"] = cachedProperty()
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
	ra := prover.RaceArgs{Module: module, Target: target, FPModes: prover.FPDefault}
	if value, exists := args["cached"]; exists {
		mode, err := parseCached(value)
		if err != nil {
			return prover.RaceArgs{}, err
		}
		ra.FPModes = mode
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

func cachedProperty() map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": "Whether to use cached proof results (default: true).",
	}
}

func parseCached(value any) (prover.FPMode, error) {
	cached, ok := value.(bool)
	if !ok {
		return prover.FPDefault, fmt.Errorf("cached must be a boolean")
	}
	if !cached {
		return prover.FPNo, nil
	}
	return prover.FPDefault, nil
}

func targetProperties() map[string]any {
	return map[string]any{
		"line": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "One source line; passed to tlapm as --line N.",
		},
		"from": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "First source line in an inclusive range; passed to tlapm as --toolbox FROM TO.",
		},
		"to": map[string]any{
			"type":        "integer",
			"minimum":     1,
			"description": "Last source line in an inclusive range; passed to tlapm as --toolbox FROM TO.",
		},
	}
}

func targetAlternatives() []any {
	return []any{
		map[string]any{
			"required": []any{"line"},
			"not": map[string]any{
				"anyOf": []any{
					map[string]any{"required": []any{"from"}},
					map[string]any{"required": []any{"to"}},
				},
			},
		},
		map[string]any{
			"required": []any{"from", "to"},
			"not":      map[string]any{"required": []any{"line"}},
		},
	}
}

func optionalTargetAlternatives() []any {
	return append(targetAlternatives(), map[string]any{
		"not": map[string]any{
			"anyOf": []any{
				map[string]any{"required": []any{"line"}},
				map[string]any{"required": []any{"from"}},
				map[string]any{"required": []any{"to"}},
			},
		},
	})
}

func parseLineTarget(args map[string]any) (prover.LineTarget, error) {
	lineValue, hasLine := args["line"]
	fromValue, hasFrom := args["from"]
	toValue, hasTo := args["to"]
	if hasLine {
		if hasFrom || hasTo {
			return prover.LineTarget{}, fmt.Errorf("provide either line or from and to, not both")
		}
		line, err := positiveInteger(lineValue, "line")
		if err != nil {
			return prover.LineTarget{}, err
		}
		return prover.LineTarget{Line: line}, nil
	}
	if !hasFrom || !hasTo {
		return prover.LineTarget{}, fmt.Errorf("provide line or both from and to")
	}
	from, err := positiveInteger(fromValue, "from")
	if err != nil {
		return prover.LineTarget{}, err
	}
	to, err := positiveInteger(toValue, "to")
	if err != nil {
		return prover.LineTarget{}, err
	}
	if to < from {
		return prover.LineTarget{}, fmt.Errorf("to must not be before from")
	}
	return prover.LineTarget{Range: &prover.LineRange{Start: from, End: to}}, nil
}

func parseOptionalLineTarget(args map[string]any) (prover.LineTarget, error) {
	_, hasLine := args["line"]
	_, hasFrom := args["from"]
	_, hasTo := args["to"]
	if !hasLine && !hasFrom && !hasTo {
		return prover.LineTarget{}, nil
	}
	return parseLineTarget(args)
}

func positiveInteger(value any, name string) (int, error) {
	n, ok := value.(float64)
	if !ok || n < 1 || n != float64(int(n)) {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return int(n), nil
}
