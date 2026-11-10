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
