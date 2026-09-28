package prover

import (
	"testing"
)

func TestParseStep(t *testing.T) {
	tests := []struct {
		name     string
		step     string
		wantKind StepKind
		wantName string
		wantErr  bool
	}{
		// Whole theorem
		{"empty step", "", StepWhole, "", false},
		{"empty brackets", "<>", StepWhole, "", false},
		{"theorem only", "Correctness", StepWhole, "Correctness", false},

		// Exact nested path
		{"single step", "Correctness/<1>", StepExact, "Correctness", false},
		{"nested path", "Correctness/<1>/<2>", StepExact, "Correctness", false},
		{"deep nesting", "Correctness/<1>/<2>/<3>/<4>", StepExact, "Correctness", false},
		{"all subproofs", "Correctness/<*>", StepExact, "Correctness", false},
		{"all subproofs no theorem", "<*>", StepExact, "", false},

		// Range
		{"range", "Correctness/<1>..<3>", StepRange, "Correctness", false},
		{"range no theorem", "<1>..<3>", StepRange, "", false},
		{"range no theorem nested", "<1>/<2>..<3>", StepRange, "", false},

		// Line
		{"line target", "line:35", StepLine, "", false},
		{"line target zero", "line:0", StepLine, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ParseStep(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseStep(%q) error = %v, wantErr %v", tt.step, err, tt.wantErr)
				return
			}
			if s.Kind != tt.wantKind {
				t.Errorf("ParseStep(%q) Kind = %v, want %v", tt.step, s.Kind, tt.wantKind)
			}
			if s.Theorem != tt.wantName && !tt.wantErr {
				t.Errorf("ParseStep(%q) Theorem = %q, want %q", tt.step, s.Theorem, tt.wantName)
			}
		})
	}
}

func TestStepValidation(t *testing.T) {
	tests := []struct {
		name    string
		step    Step
		wantErr bool
	}{
		{"valid whole", Step{Kind: StepWhole, Theorem: "Correctness"}, false},
		{"valid nested", Step{Kind: StepExact, Theorem: "Correctness", Nested: []int{1, 2, 3}}, false},
		{"valid range", Step{Kind: StepRange, Theorem: "Correctness", RangeStart: 1, RangeEnd: 3}, false},
		{"valid line", Step{Kind: StepLine, LineNum: 35}, false},
		{"invalid range start > end", Step{Kind: StepRange, RangeStart: 5, RangeEnd: 2}, true},
		{"invalid line", Step{Kind: StepLine, LineNum: 0}, true},
		{"invalid nested zero", Step{Kind: StepExact, Nested: []int{1, 0, 3}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.step.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStepString(t *testing.T) {
	tests := []struct {
		name string
		step Step
		want string
	}{
		{"whole", Step{Kind: StepWhole, Theorem: "Correctness"}, "Correctness"},
		{"line", Step{Kind: StepLine, LineNum: 35}, "line:35"},
		{"range", Step{Kind: StepRange, Theorem: "Correctness", RangeRaw: "1..3"}, "Correctness/<1..3>"},
		{"exact nested", Step{Kind: StepExact, Theorem: "Correctness", Nested: []int{1, 2, 3}}, "Correctness/<1>/<2>/<3>"},
		{"exact all", Step{Kind: StepExact, Theorem: "Correctness", RangeAll: true}, "Correctness/<*>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.step.String()
			if got != tt.want {
				t.Errorf("Step.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStepPredicates(t *testing.T) {
	t.Run("IsWhole", func(t *testing.T) {
		s1 := Step{Kind: StepWhole, Theorem: "T"}
		if !s1.IsWhole() {
			t.Error("StepWhole should be IsWhole")
		}
		s2 := Step{Kind: StepExact, Theorem: "T"}
		if !s2.IsWhole() {
			t.Error("StepExact with no nested should be IsWhole")
		}
		s3 := Step{Kind: StepExact, Theorem: "T", Nested: []int{1}}
		if s3.IsWhole() {
			t.Error("StepExact with nested should not be IsWhole")
		}
	})

	t.Run("HasRange", func(t *testing.T) {
		s4 := Step{Kind: StepRange}
		if !s4.HasRange() {
			t.Error("StepRange should have range")
		}
		s5 := Step{Kind: StepExact}
		if s5.HasRange() {
			t.Error("StepExact should not have range")
		}
	})

	t.Run("HasNested", func(t *testing.T) {
		s6 := Step{Kind: StepExact, Nested: []int{1, 2}}
		if !s6.HasNested() {
			t.Error("StepExact with nested should have nested")
		}
		s7 := Step{Kind: StepExact}
		if s7.HasNested() {
			t.Error("StepExact with no nested should not have nested")
		}
	})
}

func TestParseStepEdgeCases(t *testing.T) {
	// Step numbers with suffixes
	s, err := ParseStep("Correctness/<1a>")
	if err != nil {
		t.Fatalf("ParseStep(<1a>) error: %v", err)
	}
	if s.Kind != StepExact {
		t.Errorf("Expected StepExact for <1a>, got %v", s.Kind)
	}

	// Deep nesting
	s, err = ParseStep("Correctness/<1>/<2>/<3>/<4>/<5>")
	if err != nil {
		t.Fatalf("ParseStep(deep) error: %v", err)
	}
	if len(s.Nested) != 5 {
		t.Errorf("Expected 5 nested steps, got %d", len(s.Nested))
	}

	// Range with non-sequential numbers (valid, resolved by DFS order)
	s, err = ParseStep("Correctness/<1>..<5>")
	if err != nil {
		t.Fatalf("ParseStep(range non-sequential) error: %v", err)
	}
	if s.RangeStart != 1 || s.RangeEnd != 5 {
		t.Errorf("Expected range 1..5, got %d..%d", s.RangeStart, s.RangeEnd)
	}
}

func TestBuildDFSTree(t *testing.T) {
	content := `
--------------------- MODULE test ----------------------

LEMMA Correctness ==
    ASSUME NEW Q1 \in Quorum
    PROVE  Q1 # {}
<1>1. /\ Q1 \subseteq Server
       /\ Q2 \subseteq Server
    BY DEF Quorum
<1>2. /\ Cardinality(Q1) * 2 > Cardinality(Server)
       /\ Cardinality(Q2) * 2 > Cardinality(Server)
    BY DEF Quorum
<1>3. Cardinality(Q1) + Cardinality(Q2) > Cardinality(Server)
    BY <1>1, <1>2, FS_Subset

<1>1. \E Q1 \in Quorum:
        \A voter \in Q1: VoteFor(voter, candidate1, term)
    OBVIOUS
<1> PICK Q1 \in Quorum:
        \A voter \in Q1: VoteFor(voter, candidate1, term) BY <1>1
<1>2. \E Q2 \in Quorum:
        \A voter \in Q2: VoteFor(voter, candidate2, term)
    OBVIOUS
<1> PICK Q2 \in Quorum:
        \A voter \in Q2: VoteFor(voter, candidate2, term) BY <1>2

<1> QED BY <1>1, <1>3
    DEF VoteFor

========================================================================
`

	tree, err := buildDFSTree(content)
	if err != nil {
		t.Fatalf("buildDFSTree() error: %v", err)
	}

	// Root should have depth -1 and at least one child
	if tree.Depth != -1 {
		t.Errorf("Root depth = %d, want -1", tree.Depth)
	}
	if len(tree.Children) == 0 {
		t.Error("Root should have children")
	}

	// Check that all nodes have positive DFS indices
	var checkDFS func(n *DFSNode)
	checkDFS = func(n *DFSNode) {
		for _, child := range n.Children {
			if child.DFSIndex <= 0 {
				t.Errorf("Node <%s> at depth %d has DFSIndex %d, want > 0",
					child.StepNum, child.Depth, child.DFSIndex)
			}
			checkDFS(child)
		}
	}
	checkDFS(tree)

	// Verify total node count (root + all steps)
	var countNodes func(n *DFSNode) int
	countNodes = func(n *DFSNode) int {
		total := 1 // count self
		for _, child := range n.Children {
			total += countNodes(child)
		}
		return total
	}
	total := countNodes(tree)
	if total < 4 {
		t.Errorf("Expected at least 4 nodes (including root), got %d", total)
	}

	t.Logf("Tree has %d nodes, DFS indices assigned correctly", total)
}

func TestResolveStepRange(t *testing.T) {
	content := `
--------------------- MODULE test ----------------------

LEMMA Correctness ==
    PROVE  Q1 # {}
<1>1. /\ Q1 \subseteq Server
       /\ Q2 \subseteq Server
    BY DEF Quorum
<1>2. /\ Cardinality(Q1) * 2 > Cardinality(Server)
    BY DEF Quorum
<1>3. Cardinality(Q1) + Cardinality(Q2) > Cardinality(Server)
    BY <1>1, <1>2

<1> QED BY <1>1, <1>3
    DEF VoteFor

========================================================================
`

	tests := []struct {
		name           string
		step           string
		wantStepsMin   int
		wantResolvedOK bool
	}{
		{
			name:           "range 1..3",
			step:           "Correctness/<1>..<3>",
			wantStepsMin:   1,
			wantResolvedOK: true,
		},
		{
			name:           "range with no theorem",
			step:           "<1>..<3>",
			wantStepsMin:   1,
			wantResolvedOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ParseStep(tt.step)
			if err != nil {
				t.Fatalf("ParseStep(%q) error: %v", tt.step, err)
			}

			if s.Kind != StepRange {
				t.Fatalf("Expected StepRange, got %v", s.Kind)
			}

			resolved, err := s.ResolveStepRange(content)
			if (err == nil) != tt.wantResolvedOK {
				t.Errorf("ResolveStepRange() error = %v, wantOK %v", err, tt.wantResolvedOK)
			}

			if resolved.StepCount < tt.wantStepsMin {
				t.Errorf("Expected at least %d steps, got %d", tt.wantStepsMin, resolved.StepCount)
			}

			t.Logf("Step %q resolved to %d steps: %v", tt.step, resolved.StepCount, resolved.Steps)
		})
	}
}

func TestResolveStepRange_NotRange(t *testing.T) {
	content := "<1>test."
	// StepExact is not a range
	s := Step{Kind: StepExact, Theorem: "Correctness", Nested: []int{1}}
	_, err := s.ResolveStepRange(content)
	if err == nil {
		t.Error("Expected error for non-range step")
	}
}

func TestDFSNodeStructure(t *testing.T) {
	content := `
--------------------- MODULE test ----------------------

LEMMA Test ==
    PROVE  Q1 # {}
<1>1. /\ Q1 \subseteq Server
       /\ Q2 \subseteq Server
    BY DEF Quorum
<1>2. /\ Cardinality(Q1) * 2 > Cardinality(Server)
    BY DEF Quorum
<1>3. Cardinality(Q1) + Cardinality(Q2) > Cardinality(Server)
    BY <1>1, <1>2

<1> QED BY <1>1, <1>3
    DEF VoteFor

========================================================================
`
	tree, err := buildDFSTree(content)
	if err != nil {
		t.Fatalf("buildDFSTree() error: %v", err)
	}

	// Verify step numbers are preserved
	var checkStepNums func(n *DFSNode)
	checkStepNums = func(n *DFSNode) {
		for _, child := range n.Children {
			if child.StepNum == "" {
				t.Errorf("Node at depth %d has empty StepNum", child.Depth)
			}
			checkStepNums(child)
		}
	}
	checkStepNums(tree)

	// Verify depth hierarchy: children should be at higher depth than parents
	var checkDepth func(n *DFSNode)
	checkDepth = func(n *DFSNode) {
		for _, child := range n.Children {
			if child.Depth <= n.Depth {
				t.Errorf("Child depth %d <= parent depth %d", child.Depth, n.Depth)
			}
			checkDepth(child)
		}
	}
	checkDepth(tree)
}

func TestNestedRangeResolution(t *testing.T) {
	// Test case from the TODO:
	// <1>/<2>..<3> should resolve to steps within <1> from <2> to <3>
	content := `
--------------------- MODULE test ----------------------

LEMMA Correctness ==
    PROVE  Q1 # {}
<1>1. /\ Q1 \subseteq Server
    BY DEF Quorum
<2>1. subfact step
    OBVIOUS
<2>2. anothersubfact
    OBVIOUS
<1>2. Cardinality(Q1) + Cardinality(Q2) > Cardinality(Server)
    BY <1>1, <1>3

<1> QED BY <1>1, <1>3
    DEF VoteFor

========================================================================
`

	s, err := ParseStep("Correctness/<1>/<2>..<3>")
	if err != nil {
		t.Fatalf("ParseStep(<1>/<2>..<3>) error: %v", err)
	}

	resolved, err := s.ResolveStepRange(content)
	if err != nil {
		t.Fatalf("ResolveStepRange() error: %v", err)
	}

	if resolved.StepCount < 1 {
		t.Errorf("Expected at least 1 step in nested range, got %d", resolved.StepCount)
	}

	t.Logf("Nested range <1>/<2>..<3> resolved to %d steps: %v",
		resolved.StepCount, resolved.Steps)
}

func TestRangeStartEndOrder(t *testing.T) {
	// Range where start <= end should be valid
	s := Step{
		Kind:       StepRange,
		Theorem:    "Correctness",
		RangeStart: 1,
		RangeEnd:   5,
		RangeRaw:   "1..5",
	}

	if err := s.Validate(); err != nil {
		t.Errorf("Valid range 1..5 should pass validation: %v", err)
	}

	// Range where start > end should fail
	s2 := Step{
		Kind:       StepRange,
		Theorem:    "Correctness",
		RangeStart: 5,
		RangeEnd:   2,
		RangeRaw:   "5..2",
	}

	if err := s2.Validate(); err == nil {
		t.Error("Invalid range 5..2 should fail validation")
	}
}
