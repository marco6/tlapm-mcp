package prover

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBuildCheckCmd(t *testing.T) {
	tests := []struct {
		name string
		args CheckArgs
		want []string
	}{
		{
			name: "whole module",
			args: CheckArgs{Module: "Spec.tla"},
			want: []string{"--summary", "-N", "Spec.tla"},
		},
		{
			name: "line target",
			args: CheckArgs{Module: "Spec.tla", Target: LineTarget{Line: 12}},
			want: []string{"--summary", "-N", "--line", "12", "Spec.tla"},
		},
		{
			name: "range target",
			args: CheckArgs{Module: "Spec.tla", Target: LineTarget{Range: &LineRange{Start: 12, End: 16}}},
			want: []string{"--summary", "-N", "--toolbox", "12", "16", "Spec.tla"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := buildCheckCmd(context.Background(), tt.args)
			if got := cmd.Args[1:]; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("check command args = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseObligationCount(t *testing.T) {
	for _, tt := range []struct {
		name  string
		text  string
		count *int
	}{
		{name: "summary", text: "---- summary of module \"Spec\" ----\n  obligations_count = 3\n====", count: intPointer(3)},
		{name: "toolbox event", text: "@!!BEGIN\n@!!type:obligationsnumber\n@!!count:2\n@!!END", count: intPointer(2)},
		{name: "unknown", text: "TLAPM output without an obligation count"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := parseObligationCount(tt.text)
			if (got == nil) != (tt.count == nil) {
				t.Fatalf("parseObligationCount() = %v, want %v", got, tt.count)
			}
			if got != nil && *got != *tt.count {
				t.Fatalf("parseObligationCount() = %d, want %d", *got, *tt.count)
			}
		})
	}
}

func TestParseDiagnostics(t *testing.T) {
	output := `File "Broken.tla", line 4, character 3
Unexpected ====
File "<unknown>":
Error: Could not parse "Broken.tla" successfully.
[WARNING]: Optional proof metadata was omitted.
`
	want := []Diagnostic{
		{Line: 4, Column: 3, Severity: "error", Message: "Unexpected ===="},
		{Severity: "warning", Message: "Optional proof metadata was omitted."},
	}
	if got := parseDiagnostics(output); !reflect.DeepEqual(got, want) {
		t.Fatalf("parseDiagnostics() = %#v, want %#v", got, want)
	}
}

func intPointer(value int) *int {
	return &value
}

func TestCheck_ArithmeticTheorem(t *testing.T) {
	p := NewWithFS(testdataFS)
	result, err := p.Check(context.Background(), CheckArgs{
		Module: testFile(t, "arithmetic_theorem.tla"),
		Target: proofObligationTarget(),
	})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !result.Success {
		t.Fatalf("Check() success = false, diagnostics = %#v", result.Diagnostics)
	}
	if result.Module != testFile(t, "arithmetic_theorem.tla") {
		t.Errorf("Module = %q, want the supplied path", result.Module)
	}
	if result.Range == nil || result.Range.Start != 6 || result.Range.End != 8 {
		t.Errorf("Range = %+v, want inclusive lines 6..8", result.Range)
	}
	if result.ObligationCount == nil || *result.ObligationCount != 1 {
		t.Errorf("ObligationCount = %v, want 1", result.ObligationCount)
	}
}

func TestCheck_ParseFailureIncludesDiagnosticAndProcessDetails(t *testing.T) {
	module := filepath.Join(t.TempDir(), "Broken.tla")
	source := "---- MODULE Broken ----\nTHEOREM T ==\n    TRUE\nBY\n====\n"
	if err := os.WriteFile(module, []byte(source), 0o600); err != nil {
		t.Fatalf("write invalid module: %v", err)
	}

	result, err := New().Check(context.Background(), CheckArgs{Module: module})
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if result.Success {
		t.Fatal("Check() succeeded for a module with a parse error")
	}
	if result.ExitCode == nil || *result.ExitCode == 0 {
		t.Errorf("ExitCode = %v, want a non-zero process exit code", result.ExitCode)
	}
	if result.Stderr == nil {
		t.Fatal("Stderr is nil for an abnormal TLAPM exit")
	}
	if len(result.Diagnostics) == 0 {
		t.Fatal("Check() returned no structured diagnostics")
	}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Line == 4 && diagnostic.Column == 3 && strings.Contains(diagnostic.Message, "Unexpected") {
			return
		}
	}
	t.Errorf("diagnostics = %#v, want parse diagnostic at line 4, column 3", result.Diagnostics)
}
