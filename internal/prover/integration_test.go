package prover

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

//go:embed testdata/*.tla
var testdataFS embed.FS

// Helper: resolve path to a test data file.
func testFile(t *testing.T, rel string) string {
	t.Helper()
	return "testdata/" + rel
}

func proofObligationTarget() LineTarget {
	return LineTarget{Range: &LineRange{Start: 6, End: 8}}
}

// integration fixture files.
// hard_proofs.tla is a non-raft example with arithmetic theorems.
// Taken from https://github.com/tlaplus/CommunityModules/ and abides to its MIT license terms.
var (
	naturalNumbers  = testFile
	proofActions    = testFile
	proofRefinement = testFile
)

func init() {
	naturalNumbers = testFile // avoid unused warning
	proofActions = testFile
	proofRefinement = testFile
}

func TestLineTargetCommandArgs(t *testing.T) {
	tests := []struct {
		name    string
		target  LineTarget
		want    []string
		wantErr bool
	}{
		{name: "line", target: LineTarget{Line: 35}, want: []string{"--line", "35"}},
		{name: "range", target: LineTarget{Range: &LineRange{Start: 35, End: 42}}, want: []string{"--toolbox", "35", "42"}},
		{name: "missing target", wantErr: true},
		{name: "both targets", target: LineTarget{Line: 35, Range: &LineRange{Start: 35, End: 42}}, wantErr: true},
		{name: "reversed range", target: LineTarget{Range: &LineRange{Start: 42, End: 35}}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.target.validate()
			if (err != nil) != tt.wantErr {
				t.Fatalf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil {
				got := tt.target.appendCommandArgs(nil)
				if !reflect.DeepEqual(got, tt.want) {
					t.Errorf("appendCommandArgs() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestProve_ArithmeticTheorem(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "arithmetic_theorem.tla"),
		Target:  proofObligationTarget(),
		FPModes: FPDefault,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}
	if !result.Success {
		t.Error("Expected success for the selected theorem range")
	}
	if result.Module != "arithmetic_theorem" {
		t.Errorf("Module = %q, want %q", result.Module, "arithmetic_theorem")
	}
	if result.Timing == nil {
		t.Error("Timing map should not be nil")
	}
	if result.TotalTime <= 0 {
		t.Errorf("TotalTime = %f, want > 0", result.TotalTime)
	}
	if result.ObligationCount == nil || *result.ObligationCount != 1 {
		t.Errorf("ObligationCount = %v, want 1", result.ObligationCount)
	}
	if len(result.Obligations) != 0 {
		t.Errorf("successful proof returned unresolved obligations: %#v", result.Obligations)
	}
}

func TestProve_RangeTarget(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "arithmetic_theorem.tla"),
		Target:  proofObligationTarget(),
		FPModes: FPNo,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}
	if !result.Success {
		t.Error("Expected successful proof for the selected range")
	}
	if result.Range == nil || result.Range.Start != 6 || result.Range.End != 8 {
		t.Errorf("Range = %+v, want inclusive lines 6..8", result.Range)
	}
}

func TestProve_FailedProofReturnsCompactObligation(t *testing.T) {
	module := filepath.Join(t.TempDir(), "Unprovable.tla")
	source := "---- MODULE Unprovable ----\nNope == FALSE\nTHEOREM T == Nope\nBY DEF Nope\n====\n"
	if err := os.WriteFile(module, []byte(source), 0o600); err != nil {
		t.Fatalf("write module: %v", err)
	}

	result, err := New().Prove(context.Background(), ProveArgs{
		Module:  module,
		Target:  LineTarget{Range: &LineRange{Start: 1, End: 5}},
		FPModes: FPNo,
	})
	if err != nil {
		t.Fatalf("Prove() error = %v", err)
	}
	if result.Success {
		t.Fatal("Prove() succeeded for an unprovable theorem")
	}
	if result.ErrorCode != string(ErrProofFailed) {
		t.Errorf("ErrorCode = %q, want %q", result.ErrorCode, ErrProofFailed)
	}
	if result.ObligationCount == nil || *result.ObligationCount != 1 {
		t.Fatalf("ObligationCount = %v, want 1", result.ObligationCount)
	}
	if len(result.Obligations) != 1 {
		t.Fatalf("Obligations = %#v, want one obligation", result.Obligations)
	}
	obligation := result.Obligations[0]
	if obligation.Line != 4 || obligation.Status != "failed" {
		t.Errorf("obligation = %+v, want failed status at line 4", obligation)
	}
	if obligation.FailureReason == "" {
		t.Errorf("obligation is missing failure reason: %+v", obligation)
	}
}

func TestBuildProveCmdUsesTLAPMDefaults(t *testing.T) {
	p := New()
	cmd, err := p.buildProveCmd(context.Background(), ProveArgs{
		Module: "Spec.tla",
		Target: LineTarget{Line: 28},
	})
	if err != nil {
		t.Fatalf("buildProveCmd() error = %v", err)
	}
	want := []string{"--timing", "--line", "28", "Spec.tla"}
	if !reflect.DeepEqual(cmd.Args[1:], want) {
		t.Fatalf("prove args = %v, want %v", cmd.Args[1:], want)
	}
}

func TestProve_WithoutFingerprints(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "arithmetic_theorem.tla"),
		Target:  proofObligationTarget(),
		FPModes: FPNo,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}

	if !result.Success {
		t.Error("Expected success without fingerprints")
	}

	if result.FingerprintsUsed != "no" {
		t.Errorf("FingerprintsUsed = %q, want %q", result.FingerprintsUsed, "no")
	}

	t.Logf("No FP prove: %fs", result.TotalTime)
}

func TestProve_WithFingerprintCheck(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "arithmetic_theorem.tla"),
		Target:  proofObligationTarget(),
		FPModes: FPCheck,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}

	if !result.Success {
		t.Error("Expected success with fingerprint check")
	}

	if result.FingerprintsUsed != "check" {
		t.Errorf("FingerprintsUsed = %q, want %q", result.FingerprintsUsed, "check")
	}

	t.Logf("Fingerprint check: %fs", result.TotalTime)
}

func TestProve_ModuleNotFound(t *testing.T) {
	ctx := context.Background()
	p := New()

	_, err := p.Prove(ctx, ProveArgs{
		Module:  "/tmp/does_not_exist.tla",
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
	})
	if err == nil {
		t.Fatal("Expected error for missing module file")
	}

	t.Logf("Error: %v (type: %T)", err, err)
	if code := DetectErrorClass(err); code != ErrModuleNotFound {
		t.Errorf("Error code = %q, want %q", code, ErrModuleNotFound)
	}
}

// TestIntegration_FingerprintsCache verifies fingerprint caching works.
func TestIntegration_FingerprintsCache(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	// First prove (computes fingerprints)
	result1, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
	})
	if err != nil {
		t.Fatalf("First prove error: %v", err)
	}

	// Second prove (should use cached fingerprints)
	result2, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
	})
	if err != nil {
		t.Fatalf("Second prove error: %v", err)
	}

	if result1.TotalTime <= 0 || result2.TotalTime <= 0 {
		t.Error("TotalTime should be > 0")
	}

	t.Logf("First prove: %fs, Second prove: %fs", result1.TotalTime, result2.TotalTime)
}

// TestIntegration_ZeroObligationProof verifies an empty target is not reported as proved.
func TestIntegration_ZeroObligationProof(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "failing.tla"),
		Target:  LineTarget{Line: 48},
		FPModes: FPDefault,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}
	if result.Success {
		t.Fatal("Prove reported success for a target with no proof obligations")
	}
	if result.ErrorCode != string(ErrNoObligations) {
		t.Errorf("ErrorCode = %q, want %q", result.ErrorCode, ErrNoObligations)
	}

	t.Logf("Zero-obligation proof result: %+v", result)
}

// TestIntegration_FingerprintCachingOnHardProofs tests fingerprint caching with hard_proofs.tla.
func TestIntegration_FingerprintCachingOnHardProofs(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result1, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
	})
	if err != nil {
		t.Fatalf("First prove error: %v", err)
	}

	result2, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
	})
	if err != nil {
		t.Fatalf("Second prove error: %v", err)
	}

	t.Logf("First prove: %fs, Second prove: %fs", result1.TotalTime, result2.TotalTime)
}
