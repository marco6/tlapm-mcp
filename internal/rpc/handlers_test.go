package rpc

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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
