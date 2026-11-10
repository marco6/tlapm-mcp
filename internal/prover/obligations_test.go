package prover

import (
	"encoding/json"
	"testing"
)

func TestParseToolboxObligationsMergesByInternalID(t *testing.T) {
	output := `@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:8:1:8:3
@!!status:to be proved
@!!END
@!!BEGIN
@!!type:obligationsnumber
@!!count:1
@!!END
@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:8:1:8:3
@!!status:proved
@!!prover:smt
@!!already:true
@!!END
`

	obligations, count := parseToolboxObligations(output)
	if count == nil || *count != 1 {
		t.Fatalf("obligation count = %v, want 1", count)
	}
	if len(obligations) != 1 || obligations[0].id != 1 || obligations[0].Line != 8 || obligations[0].Status != "proved" {
		t.Fatalf("merged obligations = %#v, want one proved obligation at line 8", obligations)
	}
	if unresolved := unresolvedObligations(obligations); len(unresolved) != 0 {
		t.Fatalf("unresolved obligations = %#v, want none", unresolved)
	}
}

func TestFailedObligationUsesCompactJSON(t *testing.T) {
	output := `@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:4:1:4:3
@!!status:failed
@!!prover:smt
@!!reason:false
@!!END
@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:4:1:4:3
@!!status:failed
@!!prover:zenon
@!!reason:false
@!!END
@!!BEGIN
@!!type:obligationsnumber
@!!count:1
@!!END
`

	obligations, _ := parseToolboxObligations(output)
	finalizeObligations(obligations, false, "")
	if len(obligations) != 1 {
		t.Fatalf("parsed obligations = %#v, want one merged record", obligations)
	}
	got := unresolvedObligations(obligations)[0]
	if got.Line != 4 || got.Status != "failed" || got.FailureReason != "false" {
		t.Fatalf("failed obligation = %+v", got)
	}
	payload, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("marshal obligation: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		t.Fatalf("unmarshal obligation: %v", err)
	}
	if len(fields) != 3 || fields["line"] != float64(4) || fields["status"] != "failed" || fields["failure_reason"] != "false" {
		t.Fatalf("compact obligation JSON = %s", payload)
	}
}

func TestFinalizePendingObligation(t *testing.T) {
	obligations := []Obligation{{id: 1, Status: "pending"}}
	finalizeObligations(obligations, false, "TLAPM exited before reporting a result")
	if obligations[0].Status != "backend-error" || obligations[0].FailureReason == "" {
		t.Fatalf("finalized obligation = %+v", obligations[0])
	}
}

func TestParseResultWarnsAboutBackendAfterFallback(t *testing.T) {
	output := `@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:to be proved
@!!END
@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:failed
@!!prover:smt
@!!reason:false
@!!already:false
@!!END
@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:proved
@!!prover:zenon
@!!already:false
@!!END
@!!BEGIN
@!!type:obligationsnumber
@!!count:1
@!!END
[INFO]: All 1 obligation proved.
`

	result, err := parseResult(output, ProveArgs{
		Module: "Fallback.tla",
		Target: LineTarget{Line: 7},
	})
	if err != nil {
		t.Fatalf("parseResult() error = %v", err)
	}
	if !result.Success {
		t.Fatal("parseResult() did not report success")
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one warning", result.Warnings)
	}
	if result.Warnings[0] != (Warning{Line: 7, What: "missing `BY Zenon`"}) {
		t.Fatalf("warning = %+v, want line 7 missing BY Zenon", result.Warnings[0])
	}
}

func TestParseResultWarnsAboutIsabelleAfterTwoFallbacks(t *testing.T) {
	output := `@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:9:1:9:8
@!!status:failed
@!!prover:smt
@!!reason:timeout
@!!END
@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:9:1:9:8
@!!status:failed
@!!prover:zenon
@!!reason:false
@!!END
@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:9:1:9:8
@!!status:proved
@!!prover:isabelle
@!!END
[INFO]: All 1 obligation proved.
`

	result, err := parseResult(output, ProveArgs{Module: "Fallback.tla", Target: LineTarget{Line: 9}})
	if err != nil {
		t.Fatalf("parseResult() error = %v", err)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one warning", result.Warnings)
	}
	if result.Warnings[0] != (Warning{Line: 9, What: "missing `BY Isa`"}) {
		t.Fatalf("warning = %+v, want line 9 missing BY Isa", result.Warnings[0])
	}
}

func TestParseResultDoesNotTreatInProgressBackendAsFailure(t *testing.T) {
	output := `@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:being proved
@!!prover:smt
@!!END
@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:proved
@!!prover:smt
@!!END
[INFO]: All 1 obligation proved.
`

	result, err := parseResult(output, ProveArgs{Module: "Direct.tla", Target: LineTarget{Line: 7}})
	if err != nil {
		t.Fatalf("parseResult() error = %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %#v, want none", result.Warnings)
	}
}

func TestParseResultIgnoresNonBackendFailures(t *testing.T) {
	output := `@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:failed
@!!prover:expandcdot
@!!reason:hidden definition
@!!END
@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:proved
@!!prover:smt
@!!END
[INFO]: All 1 obligation proved.
`

	result, err := parseResult(output, ProveArgs{Module: "Direct.tla", Target: LineTarget{Line: 7}})
	if err != nil {
		t.Fatalf("parseResult() error = %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %#v, want none", result.Warnings)
	}
}

func TestParseResultOmitsWarningsOnFailedProof(t *testing.T) {
	output := `@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:failed
@!!prover:smt
@!!reason:false
@!!END
@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:failed
@!!prover:zenon
@!!reason:false
@!!END
[ERROR]: 1/1 obligation failed.
`

	result, err := parseResult(output, ProveArgs{Module: "Unproved.tla", Target: LineTarget{Line: 7}})
	if err != nil {
		t.Fatalf("parseResult() error = %v", err)
	}
	if result.Success {
		t.Fatal("parseResult() reported success")
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("Warnings = %#v, want none for an unproved obligation", result.Warnings)
	}
}

func TestParseResultDeduplicatesWarningsForSameLine(t *testing.T) {
	output := `@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:failed
@!!prover:smt
@!!reason:false
@!!END
@!!BEGIN
@!!type:obligation
@!!id:1
@!!loc:7:1:7:8
@!!status:proved
@!!prover:zenon
@!!END
@!!BEGIN
@!!type:obligation
@!!id:2
@!!loc:7:1:7:8
@!!status:failed
@!!prover:isabelle
@!!reason:false
@!!END
@!!BEGIN
@!!type:obligation
@!!id:2
@!!loc:7:1:7:8
@!!status:proved
@!!prover:zenon
@!!END
[INFO]: All 2 obligations proved.
`

	result, err := parseResult(output, ProveArgs{Module: "Merged.tla", Target: LineTarget{Line: 7}})
	if err != nil {
		t.Fatalf("parseResult() error = %v", err)
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("Warnings = %#v, want one warning", result.Warnings)
	}
	if result.Warnings[0] != (Warning{Line: 7, What: "missing `BY Zenon`"}) {
		t.Fatalf("warning = %+v, want line 7 missing BY Zenon", result.Warnings[0])
	}
}

func TestParsePartialToolboxEventAtEOF(t *testing.T) {
	output := `@!!BEGIN
@!!type:obligation
@!!id:7
@!!loc:12:2:12:5
@!!status:to be proved`
	obligations, count := parseToolboxObligations(output)
	if count != nil || len(obligations) != 1 {
		t.Fatalf("partial obligations/count = %#v / %v", obligations, count)
	}
	if obligations[0].Line != 12 || obligations[0].Status != "pending" {
		t.Fatalf("partial obligation = %+v", obligations[0])
	}
	finalizeObligations(obligations, false, "TLAPM exited while processing the obligation")
	if obligations[0].Status != "backend-error" {
		t.Errorf("partial obligation status = %q, want backend-error", obligations[0].Status)
	}
}
