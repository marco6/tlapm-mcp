package rpc

import (
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tlaplus/tlapm-mcp/internal/prover"
)

func TestProofToolsExposeLineOrRangeTargets(t *testing.T) {
	tools := []struct {
		name string
		tool *mcp.Tool
	}{
		{name: "prove", tool: ProveTool()},
		{name: "race", tool: RaceTool()},
	}
	for _, tt := range tools {
		t.Run(tt.name, func(t *testing.T) {
			schema, ok := tt.tool.InputSchema.(map[string]any)
			if !ok {
				t.Fatal("input schema is not an object")
			}
			properties, ok := schema["properties"].(map[string]any)
			if !ok {
				t.Fatal("input schema has no properties object")
			}
			line, ok := properties["line"].(map[string]any)
			if !ok || line["type"] != "integer" || line["minimum"] != 1 {
				t.Fatalf("line schema = %#v, want one-based integer", properties["line"])
			}
			lineDescription, _ := line["description"].(string)
			if !strings.Contains(lineDescription, "--line N") {
				t.Errorf("line description = %q, want --line N", lineDescription)
			}

			rangeSchema, ok := properties["range"].(map[string]any)
			if !ok || rangeSchema["type"] != "object" {
				t.Fatalf("range schema = %#v, want object", properties["range"])
			}
			rangeDescription, _ := rangeSchema["description"].(string)
			if !strings.Contains(rangeDescription, "--toolbox START END") {
				t.Errorf("range description = %q, want --toolbox START END", rangeDescription)
			}
			oneOf, ok := schema["oneOf"].([]any)
			if !ok || len(oneOf) != 2 {
				t.Errorf("oneOf = %#v, want separate line and range alternatives", schema["oneOf"])
			}
		})
	}
}

func TestProofOptionSchemasUseStandardConstraints(t *testing.T) {
	for _, tool := range []*mcp.Tool{ProveTool(), RaceTool()} {
		schema := tool.InputSchema.(map[string]any)
		properties := schema["properties"].(map[string]any)
		threads := properties["threads"].(map[string]any)
		if threads["type"] != "integer" || threads["minimum"] != 1 {
			t.Errorf("%s threads schema = %#v, want positive integer", tool.Name, threads)
		}
		mode := properties["use_fingerprints"].(map[string]any)
		branches, ok := mode["anyOf"].([]any)
		if !ok || len(branches) != 2 {
			t.Fatalf("%s fingerprint schema = %#v, want standard anyOf", tool.Name, mode)
		}
		stringBranch := branches[1].(map[string]any)
		if stringBranch["type"] != "string" || !reflect.DeepEqual(stringBranch["enum"], []any{"check"}) {
			t.Errorf("%s fingerprint string schema = %#v", tool.Name, stringBranch)
		}
	}
}

func TestParseFingerprintModes(t *testing.T) {
	for _, tt := range []struct {
		value any
		want  prover.FPMode
	}{
		{value: true, want: prover.FPDefault},
		{value: false, want: prover.FPNo},
		{value: "check", want: prover.FPCheck},
	} {
		got, err := parseFingerprintMode(tt.value)
		if err != nil || got != tt.want {
			t.Errorf("parseFingerprintMode(%v) = %v, %v; want %v, nil", tt.value, got, err, tt.want)
		}
	}
}

func TestArgumentParsingRejectsInvalidThreadAndFingerprintValues(t *testing.T) {
	base := map[string]any{"module": "Spec.tla", "line": float64(1)}
	for _, tt := range []struct {
		name  string
		args  map[string]any
		parse func(map[string]any) error
	}{
		{name: "prove zero threads", args: map[string]any{"threads": float64(0)}, parse: func(a map[string]any) error { _, err := parseProveArgs(a); return err }},
		{name: "prove negative threads", args: map[string]any{"threads": float64(-1)}, parse: func(a map[string]any) error { _, err := parseProveArgs(a); return err }},
		{name: "prove unknown cache mode", args: map[string]any{"use_fingerprints": "typo"}, parse: func(a map[string]any) error { _, err := parseProveArgs(a); return err }},
		{name: "race zero threads", args: map[string]any{"threads": float64(0)}, parse: func(a map[string]any) error { _, err := parseRaceArgs(a); return err }},
		{name: "race negative threads", args: map[string]any{"threads": float64(-1)}, parse: func(a map[string]any) error { _, err := parseRaceArgs(a); return err }},
		{name: "race unknown cache mode", args: map[string]any{"use_fingerprints": "typo"}, parse: func(a map[string]any) error { _, err := parseRaceArgs(a); return err }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			args := make(map[string]any, len(base)+len(tt.args))
			for key, value := range base {
				args[key] = value
			}
			for key, value := range tt.args {
				args[key] = value
			}
			if err := tt.parse(args); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
