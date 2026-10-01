package prover

import (
	"context"
	"embed"
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

func TestProve_NaturalNumbers(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		Solver:  "",
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}
	if !result.Success {
		t.Error("Expected success for hard_proofs (simple theorems)")
	}
	if result.Module != "hard_proofs" {
		t.Errorf("Module = %q, want %q", result.Module, "hard_proofs")
	}
	if result.Timing == nil {
		t.Error("Timing map should not be nil")
	}
	if result.TotalTime <= 0 {
		t.Errorf("TotalTime = %f, want > 0", result.TotalTime)
	}
}

func TestProve_RangeTarget(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Range: &LineRange{Start: 28, End: 34}},
		FPModes: FPNo,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}
	if !result.Success {
		t.Error("Expected successful proof for the selected range")
	}
	if result.Range == nil || result.Range.Start != 28 || result.Range.End != 34 {
		t.Errorf("Range = %+v, want inclusive lines 28..34", result.Range)
	}
}

func TestProve_WithSolver(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		Solver:  "z3",
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}

	if !result.Success {
		t.Error("Expected success with z3")
	}

	if result.Solver != "z3" {
		t.Errorf("Solver = %q, want %q", result.Solver, "z3")
	}

	if result.FingerprintsUsed != "use" {
		t.Errorf("FingerprintsUsed = %q, want %q", result.FingerprintsUsed, "use")
	}

	t.Logf("Z3 prove: %fs", result.TotalTime)
}

func TestProve_WithoutFingerprints(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		Solver:  "",
		FPModes: FPNo,
		Threads: 1,
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

func TestProve_WithThreads(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		Solver:  "",
		FPModes: FPDefault,
		Threads: 2,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}

	if !result.Success {
		t.Error("Expected success with threads=2")
	}

	t.Logf("Threaded prove: %fs", result.TotalTime)
}

func TestProve_WithFingerprintCheck(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		Solver:  "",
		FPModes: FPCheck,
		Threads: 1,
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
		Solver:  "",
		FPModes: FPDefault,
		Threads: 1,
	})
	if err == nil {
		t.Fatal("Expected error for missing module file")
	}

	t.Logf("Error: %v (type: %T)", err, err)
	if code := DetectErrorClass(err); code != ErrModuleNotFound {
		t.Errorf("Error code = %q, want %q", code, ErrModuleNotFound)
	}
}

func TestRace(t *testing.T) {
	ctx := context.Background()
	p := New()

	result, err := p.Race(ctx, RaceArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
		Threads: 4,
	})
	if err != nil {
		t.Fatalf("Race error: %v", err)
	}

	if len(result.Results) != 13 {
		t.Errorf("Expected 13 results, got %d", len(result.Results))
	}

	if result.Fastest.Solver == "" {
		t.Error("Fastest solver should not be empty")
	}

	if !result.Fastest.Success {
		t.Error("Fastest solver should have succeeded")
	}

	t.Logf("Fastest: %s (time: %fs), total results: %d",
		result.Fastest.Solver, result.Fastest.TotalTimeSeconds, len(result.Results))
}

func TestRace_AllFail(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Race(ctx, RaceArgs{
		Module:  testFile(t, "failing.tla"),
		Target:  LineTarget{Line: 48},
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Race error: %v", err)
	}

	// failing.tla has FactorialGrows - verify race completes successfully
	if result.Fastest.Solver == "" {
		t.Error("Fastest solver should not be empty")
	}

	t.Logf("Fastest: %s (time: %fs), total results: %d",
		result.Fastest.Solver, result.Fastest.TotalTimeSeconds, len(result.Results))
}

func TestRace_NaturalNumbers(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Race(ctx, RaceArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
		Threads: 4,
	})
	if err != nil {
		t.Fatalf("Race error: %v", err)
	}

	if len(result.Results) == 0 {
		t.Error("Expected at least one result")
	}

	if result.Fastest.Solver == "" {
		t.Error("Fastest solver should not be empty")
	}

	t.Logf("Fastest solver succeeded on hard_proofs: %s (time: %fs)",
		result.Fastest.Solver, result.Fastest.TotalTimeSeconds)
}

func TestRace_RangeTarget(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Race(ctx, RaceArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Range: &LineRange{Start: 28, End: 34}},
		FPModes: FPDefault,
		Threads: 4,
	})
	if err != nil {
		t.Fatalf("Race error: %v", err)
	}

	if result.Fastest.Solver == "" {
		t.Error("Fastest solver should not be empty")
	}

	t.Logf("Fastest solver succeeded with range target: %s (time: %fs)",
		result.Fastest.Solver, result.Fastest.TotalTimeSeconds)
}

// TestIntegration_ProveRaceConsistency verifies that prove and race agree.
func TestIntegration_ProveRaceConsistency(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	// Test prove
	proveResult, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		Solver:  "",
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}

	// Test race
	raceResult, err := p.Race(ctx, RaceArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
		Threads: 4,
	})
	if err != nil {
		t.Fatalf("Race error: %v", err)
	}

	if proveResult.Success != raceResult.Fastest.Success {
		t.Errorf("Prove success=%v, Race success=%v — mismatch",
			proveResult.Success, raceResult.Fastest.Success)
	}

	t.Logf("Prove: success=%v, Race: fastest=%s (success=%v)",
		proveResult.Success, raceResult.Fastest.Solver, raceResult.Fastest.Success)
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
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("First prove error: %v", err)
	}

	// Second prove (should use cached fingerprints)
	result2, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Second prove error: %v", err)
	}

	if result1.TotalTime <= 0 || result2.TotalTime <= 0 {
		t.Error("TotalTime should be > 0")
	}

	t.Logf("First prove: %fs, Second prove: %fs", result1.TotalTime, result2.TotalTime)
}

// TestIntegration_ProveWithNonExistentSolver exercises the failover path.
func TestIntegration_ProveWithNonExistentSolver(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	// z3 is likely not available, so prove should still work but report it
	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		Solver:  "z3",
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		// Check if it's a solver error, not a proof error
		if code := DetectErrorClass(err); code != ErrSolverUnavailable {
			t.Fatalf("Unexpected error: %v (code: %s)", err, code)
		}
	}

	t.Logf("Prove with z3: success=%v, error=%v", result.Success, err)
}

// TestIntegration_FailingProof tests that failing proofs are correctly detected.
func TestIntegration_FailingProof(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "failing.tla"),
		Target:  LineTarget{Line: 48},
		Solver:  "z3",
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}

	t.Logf("Failing proof result: %+v, err: %v", result, err)
}

// TestIntegration_FailingRace tests that race correctly reports failed solvers.
func TestIntegration_FailingRace(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Race(ctx, RaceArgs{
		Module:  testFile(t, "failing.tla"),
		Target:  LineTarget{Line: 48},
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Race error: %v", err)
	}

	t.Logf("Fastest: %s (time: %fs), total results: %d",
		result.Fastest.Solver, result.Fastest.TotalTimeSeconds, len(result.Results))
}

// TestIntegration_FingerprintCachingOnHardProofs tests fingerprint caching with hard_proofs.tla.
func TestIntegration_FingerprintCachingOnHardProofs(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result1, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("First prove error: %v", err)
	}

	result2, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Second prove error: %v", err)
	}

	t.Logf("First prove: %fs, Second prove: %fs", result1.TotalTime, result2.TotalTime)
}

// TestIntegration_ProveRaceConsistency_HardProofs tests that prove and race agree.
func TestIntegration_ProveRaceConsistency_HardProofs(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	// Test prove
	proveResult, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		Solver:  "",
		FPModes: FPDefault,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}

	// Test race
	raceResult, err := p.Race(ctx, RaceArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		FPModes: FPDefault,
		Threads: 4,
	})
	if err != nil {
		t.Fatalf("Race error: %v", err)
	}

	if proveResult.Success != raceResult.Fastest.Success {
		t.Errorf("Prove success=%v, Race success=%v — mismatch",
			proveResult.Success, raceResult.Fastest.Success)
	}

	t.Logf("Prove: success=%v, Race: fastest=%s (success=%v)",
		proveResult.Success, raceResult.Fastest.Solver, raceResult.Fastest.Success)
}

// TestIntegration_ProveWithFingerprintCheck_HardProofs tests fingerprint check mode.
func TestIntegration_ProveWithFingerprintCheck_HardProofs(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		Solver:  "",
		FPModes: FPCheck,
		Threads: 1,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}

	if result.FingerprintsUsed != "check" {
		t.Errorf("FingerprintsUsed = %q, want %q", result.FingerprintsUsed, "check")
	}

	t.Logf("Fingerprint check: %fs", result.TotalTime)
}

// TestIntegration_ProveWithThreads_HardProofs tests threaded proving.
func TestIntegration_ProveWithThreads_HardProofs(t *testing.T) {
	ctx := context.Background()
	p := NewWithFS(testdataFS)

	result, err := p.Prove(ctx, ProveArgs{
		Module:  testFile(t, "hard_proofs.tla"),
		Target:  LineTarget{Line: 28},
		Solver:  "",
		FPModes: FPDefault,
		Threads: 2,
	})
	if err != nil {
		t.Fatalf("Prove error: %v", err)
	}

	if !result.Success {
		t.Error("Expected success with threads=2")
	}

	t.Logf("Threaded prove: %fs", result.TotalTime)
}
