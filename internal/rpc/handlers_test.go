package rpc

import (
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/tlaplus/tlapm-mcp/internal/prover"
)

func TestProofToolsExposeFlattenedTargetsAndOptions(t *testing.T) {
	tools := []struct {
		name string
		tool *mcp.Tool
	}{
		{name: "prove", tool: ProveTool()},
		{name: "race", tool: RaceTool()},
	}
	for _, tt := range tools {
		t.Run(tt.name, func(t *testing.T) {
			schema := tt.tool.InputSchema.(map[string]any)
			properties := schema["properties"].(map[string]any)
			for _, name := range []string{"line", "from", "to"} {
				field, ok := properties[name].(map[string]any)
				if !ok || field["type"] != "integer" || field["minimum"] != 1 {
					t.Errorf("%s schema = %#v, want positive integer", name, properties[name])
				}
			}
			for _, removed := range []string{"range", "solver", "threads", "use_fingerprints"} {
				if _, exists := properties[removed]; exists {
					t.Errorf("removed property %q remains in schema", removed)
				}
			}
			cached, ok := properties["cached"].(map[string]any)
			if !ok || cached["type"] != "boolean" {
				t.Errorf("cached schema = %#v, want boolean", properties["cached"])
			}
			alternatives, ok := schema["oneOf"].([]any)
			if !ok || len(alternatives) != 2 {
				t.Fatalf("oneOf = %#v, want line and range alternatives", schema["oneOf"])
			}
			lineAlternative := alternatives[0].(map[string]any)
			rangeAlternative := alternatives[1].(map[string]any)
			if !reflect.DeepEqual(lineAlternative["required"], []any{"line"}) {
				t.Errorf("line alternative required = %#v", lineAlternative["required"])
			}
			if !reflect.DeepEqual(rangeAlternative["required"], []any{"from", "to"}) {
				t.Errorf("range alternative required = %#v", rangeAlternative["required"])
			}
		})
	}
}

func TestParseCachedOption(t *testing.T) {
	parsers := []struct {
		name  string
		parse func(map[string]any) (prover.FPMode, error)
	}{
		{
			name: "prove",
			parse: func(args map[string]any) (prover.FPMode, error) {
				parsed, err := parseProveArgs(args)
				return parsed.FPModes, err
			},
		},
		{
			name: "race",
			parse: func(args map[string]any) (prover.FPMode, error) {
				parsed, err := parseRaceArgs(args)
				return parsed.FPModes, err
			},
		},
	}
	for _, parser := range parsers {
		for _, tt := range []struct {
			name    string
			args    map[string]any
			want    prover.FPMode
			wantErr bool
		}{
			{name: "default", want: prover.FPDefault},
			{name: "enabled", args: map[string]any{"cached": true}, want: prover.FPDefault},
			{name: "disabled", args: map[string]any{"cached": false}, want: prover.FPNo},
			{name: "non-boolean", args: map[string]any{"cached": "check"}, wantErr: true},
		} {
			t.Run(parser.name+"/"+tt.name, func(t *testing.T) {
				args := map[string]any{"module": "Spec.tla", "line": float64(1)}
				for key, value := range tt.args {
					args[key] = value
				}
				got, err := parser.parse(args)
				if (err != nil) != tt.wantErr {
					t.Fatalf("parse cached args error = %v, wantErr %v", err, tt.wantErr)
				}
				if err == nil && got != tt.want {
					t.Fatalf("parse cached args = %v, want %v", got, tt.want)
				}
			})
		}
	}
}

func TestParseFlattenedLineTargets(t *testing.T) {
	for _, tt := range []struct {
		name    string
		args    map[string]any
		want    prover.LineTarget
		wantErr bool
	}{
		{name: "line", args: map[string]any{"line": float64(4)}, want: prover.LineTarget{Line: 4}},
		{name: "inclusive range", args: map[string]any{"from": float64(4), "to": float64(7)}, want: prover.LineTarget{Range: &prover.LineRange{Start: 4, End: 7}}},
		{name: "reversed range", args: map[string]any{"from": float64(7), "to": float64(4)}, wantErr: true},
		{name: "incomplete range", args: map[string]any{"from": float64(4)}, wantErr: true},
		{name: "both target forms", args: map[string]any{"line": float64(4), "from": float64(4), "to": float64(7)}, wantErr: true},
		{name: "nested legacy range", args: map[string]any{"range": map[string]any{"start": float64(4), "end": float64(7)}}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLineTarget(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseLineTarget() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLineTarget() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
