package prover

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseResultRequiresAggregateProofCompletion(t *testing.T) {
	args := ProveArgs{
		Module:  "Spec.tla",
		Target:  LineTarget{Line: 4},
		FPModes: FPDefault,
	}
	tests := []struct {
		name     string
		output   string
		want     bool
		wantCode string
	}{
		{
			name:   "aggregate completion",
			output: "[INFO]: All 37 obligations proved.\n",
			want:   true,
		},
		{
			name:     "single obligation is not completion",
			output:   "[INFO]: 1 obligation proved.\n",
			wantCode: string(ErrParse),
		},
		{
			name:     "zero obligations is not proof completion",
			output:   "[INFO]: All 0 obligation proved.\n",
			wantCode: string(ErrNoObligations),
		},
		{
			name:     "error overrides completion summary",
			output:   "[INFO]: All 37 obligations proved.\n[ERROR]: solver failed\n",
			wantCode: string(ErrParse),
		},
		{
			name:     "empty output is inconclusive",
			output:   "",
			wantCode: string(ErrParse),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseResult(tt.output, args)
			if err != nil {
				t.Fatalf("parseResult() error = %v", err)
			}
			if got.Success != tt.want {
				t.Errorf("Success = %v, want %v", got.Success, tt.want)
			}
			if got.ErrorCode != tt.wantCode {
				t.Errorf("ErrorCode = %q, want %q", got.ErrorCode, tt.wantCode)
			}
		})
	}
}

func TestAllObligationsProvedRejectsPartialAndConflictingOutput(t *testing.T) {
	for _, output := range []string{
		"[INFO]: 1 obligation proved.\n",
		"[INFO]: All 0 obligation proved.\n",
		"[INFO]: All 37 obligations proved.\n[ERROR]: a later obligation failed\n",
	} {
		if allObligationsProved(output) {
			t.Errorf("allObligationsProved(%q) = true", output)
		}
	}
}

func TestHasZeroObligationCompletionRejectsErrors(t *testing.T) {
	if !hasZeroObligationCompletion("[INFO]: All 0 obligations proved.\n") {
		t.Fatal("hasZeroObligationCompletion() = false for zero-obligation completion")
	}
	if hasZeroObligationCompletion("[INFO]: All 0 obligations proved.\n[ERROR]: a proof obligation failed\n") {
		t.Fatal("hasZeroObligationCompletion() = true with an error line")
	}
}

func TestEnsureModuleExistsAcceptsAbsolutePath(t *testing.T) {
	module := filepath.Join(t.TempDir(), "Spec.tla")
	if err := os.WriteFile(module, []byte("---- MODULE Spec ----\n====\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureModuleExists(os.DirFS("."), module); err != nil {
		t.Fatalf("EnsureModuleExists(%q) error = %v", module, err)
	}
}

func TestProveReturnsTLAPMNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	result, err := NewWithFS(testdataFS).Prove(context.Background(), ProveArgs{
		Module:  "testdata/hard_proofs.tla",
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
	})
	if result.Success {
		t.Fatal("Prove reported success without running TLAPM")
	}
	var tlapmErr *TLAPMError
	if !errors.As(err, &tlapmErr) || tlapmErr.Code != ErrTLAPMNotFound {
		t.Fatalf("Prove error = %v, want TLAPM_NOT_FOUND", err)
	}
}

func TestRaceReturnsTLAPMNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := New().Race(context.Background(), RaceArgs{
		Module:  "Spec.tla",
		Target:  LineTarget{Line: 1},
		FPModes: FPDefault,
	})
	var tlapmErr *TLAPMError
	if !errors.As(err, &tlapmErr) || tlapmErr.Code != ErrTLAPMNotFound {
		t.Fatalf("Race error = %v, want TLAPM_NOT_FOUND", err)
	}
}

func TestBuildRaceCmdArgsUsesMethodFlag(t *testing.T) {
	args := RaceArgs{
		Module:  "Spec.tla",
		Target:  LineTarget{Range: &LineRange{Start: 4, End: 7}},
		FPModes: FPNo,
	}
	got := buildRaceCmdArgs("blast", args)
	want := []string{"--timing", "--method", "blast", "--toolbox", "4", "7", "--nofp", "Spec.tla"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildRaceCmdArgs() = %v, want %v", got, want)
	}
}

func TestLogErrorWritesEscapedJSON(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalStderr := os.Stderr
	os.Stderr = writer
	LogError("prove", WrapToolError("prove", ErrTLAPMNotFound, "missing \"tlapm\"", "line one\nline two"))
	_ = writer.Close()
	os.Stderr = originalStderr
	payload, err := io.ReadAll(reader)
	_ = reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(payload, &got); err != nil {
		t.Fatalf("LogError output is not valid JSON: %q: %v", payload, err)
	}
	if got["message"] != "missing \"tlapm\"" || got["details"] != "line one\nline two" {
		t.Fatalf("LogError JSON = %#v", got)
	}
}
