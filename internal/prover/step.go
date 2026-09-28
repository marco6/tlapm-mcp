package prover

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// DFSNode represents a node in the DFS proof tree.
type DFSNode struct {
	StepNum  string // step number string, e.g. "1", "2a", "3"
	Depth    int    // depth in tree (0 = top level)
	Name     string // optional name prefix (e.g. "factA" from "<1>factA.")
	Children []*DFSNode
	DFSIndex int  // position in DFS traversal (set after traversal)
	IsStart  bool // DFS traversal start marker
	IsEnd    bool // DFS traversal end marker
}

// buildDFSTree builds a DFS tree from the TLA+ file content.
// Steps are identified by <N> patterns; depth is determined by indentation.
func buildDFSTree(content string) (*DFSNode, error) {
	root := &DFSNode{Depth: -1} // sentinel root
	stack := []*DFSNode{root}

	lines := strings.Split(content, "\n")
	stepRe := regexp.MustCompile(`<(\d+[a-z]?[0-9]*[a-z]*)>`)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "----") ||
			strings.HasPrefix(trimmed, "==") ||
			strings.HasPrefix(trimmed, "EXTENDS") ||
			strings.HasPrefix(trimmed, "PROVE") ||
			strings.HasPrefix(trimmed, "OBVIOUS") ||
			strings.HasPrefix(trimmed, "LEMMA") ||
			strings.HasPrefix(trimmed, "THEOREM") ||
			strings.HasPrefix(trimmed, "AXIOM") ||
			strings.HasPrefix(trimmed, "DEFINE") ||
			strings.HasPrefix(trimmed, "BY ") ||
			strings.HasPrefix(trimmed, "USE ") ||
			strings.HasPrefix(trimmed, "SUFFICES") ||
			strings.HasPrefix(trimmed, "NEW ") ||
			strings.HasPrefix(trimmed, "ASSUME") {
			continue
		}

		hasStep := stepRe.MatchString(line)
		if !hasStep {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " "))
		depth := indent / 2
		if depth < 0 {
			depth = 0
		}

		matches := stepRe.FindStringSubmatch(line)
		if len(matches) < 2 {
			continue
		}
		stepNum := matches[1]

		name := extractStepName(line, stepNum)

		node := &DFSNode{
			StepNum: stepNum,
			Depth:   depth,
			Name:    name,
		}

		for len(stack) > 1 && stack[len(stack)-1].Depth >= depth {
			stack = stack[:len(stack)-1]
		}

		parent := stack[len(stack)-1]
		parent.Children = append(parent.Children, node)

		stack = append(stack, node)
	}

	assignDFSIndex(root, &counter{})

	return root, nil
}

// counter tracks DFS index assignment.
type counter struct {
	idx int
}

// assignDFSIndex performs a DFS traversal and assigns sequential indices.
func assignDFSIndex(node *DFSNode, c *counter) {
	for _, child := range node.Children {
		c.idx++
		child.DFSIndex = c.idx
		assignDFSIndex(child, c)
	}
}

// extractStepName extracts the text name from a step line.
// E.g. "<1>factA." -> "factA", "<1>1. /\ Q1" -> "" (no name, just number)
func extractStepName(line string, stepNum string) string {
	idx := strings.Index(line, "<"+stepNum+">")
	if idx < 0 {
		return ""
	}
	after := line[idx+len("<"+stepNum+">"):]
	after = strings.TrimLeft(after, " ")

	name := ""
	for _, c := range after {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' {
			name += string(c)
		} else {
			break
		}
	}

	return name
}

// ResolveStepRange resolves a range step against the DFS tree built from file content.
// It returns a list of step identifiers in DFS order between the range boundaries.
func (s Step) ResolveStepRange(content string) (ResolvedRange, error) {
	if s.Kind != StepRange {
		return ResolvedRange{}, fmt.Errorf("step is not a range: %v", s.Kind)
	}

	tree, err := buildDFSTree(content)
	if err != nil {
		return ResolvedRange{}, err
	}

	startIdx := s.RangeStart
	endIdx := s.RangeEnd

	allNodes := dfsNodes(tree)

	var steps []string
	for _, node := range allNodes {
		if node.DFSIndex >= startIdx && node.DFSIndex <= endIdx {
			if node.Name != "" {
				steps = append(steps, fmt.Sprintf("<%s>%s", node.StepNum, node.Name))
			} else {
				steps = append(steps, fmt.Sprintf("<%s>", node.StepNum))
			}
		}
	}

	resolvedStep := s.String()
	if s.RangeStart > 0 && s.RangeEnd > 0 {
		resolvedStep = fmt.Sprintf("%s/<%d>..<%d>", s.Theorem, s.RangeStart, s.RangeEnd)
	}

	return ResolvedRange{
		ResolvedStep: resolvedStep,
		Steps:        steps,
		StepCount:    len(steps),
	}, nil
}

// dfsNodes returns all nodes in the tree (including root) in DFS order.
func dfsNodes(root *DFSNode) []*DFSNode {
	var result []*DFSNode
	var visit func(*DFSNode)
	visit = func(n *DFSNode) {
		if n != nil {
			result = append(result, n)
			for _, child := range n.Children {
				visit(child)
			}
		}
	}
	visit(root)
	return result
}

// ResolvedRange represents a resolved step range with the actual steps in DFS order.
type ResolvedRange struct {
	ResolvedStep string   `json:"resolved_step"`
	Steps        []string `json:"steps"`
	StepCount    int      `json:"step_count"`
}

// StepKind classifies a step selector.
type StepKind int

const (
	StepWhole StepKind = iota // whole theorem
	StepExact                 // exact nested path, e.g. <1>/<2>/<3>
	StepRange                 // range, e.g. <1>..<3>
	StepLine                  // line:35
)

// Step represents a parsed step selector.
type Step struct {
	Kind       StepKind
	Theorem    string       // theorem name (empty for line: targets)
	Nested     []int        // exact nesting path: [1, 2, 3]
	RangeStart int          // start of range (1-indexed in DFS)
	RangeEnd   int          // end of range
	RangeRaw   string       // raw range string, e.g. "1..3"
	LineNum    int          // line number (for StepLine)
	RangeAll   bool         // if true, use <*>
}

// ParseStep parses a step string into a structured Step.
//
// Supported formats:
//
//	"TheoremName"        → whole theorem
//	"TheoremName/<1>"    → step 1 within theorem
//	"TheoremName/<1>/<2>"→ nested: step 2 inside step 1
//	"TheoremName/<*>"    → all direct subproofs
//	"TheoremName/<1>..<3>"→ range from <1> to <3>
//	"TheoremName/<1>/<2>..<3>"→ range from <2> to <3> inside <1>
//	"line:35"            → prove at line 35
//	"<*>"                → all subproofs
//	"<1>..<3>"           → range
func ParseStep(step string) (Step, error) {
	s := Step{}

	// Line number targeting
	if strings.HasPrefix(step, "line:") {
		n, err := strconv.Atoi(strings.TrimPrefix(step, "line:"))
		s.Kind = StepLine
		if err != nil || n <= 0 {
			return s, fmt.Errorf("invalid line number: %s", step)
		}
		s.LineNum = n
		return s, nil
	}

	if step == "" || step == "<>" {
		s.Kind = StepWhole
		return s, nil
	}

	// Check if step starts with '<' — it's a pure step selector with no theorem name
	if strings.HasPrefix(step, "<") {
		return parseSelector(step, &s)
	}

	// Split into theorem name and step selector
	parts := splitStep(step)
	s.Theorem = parts[0]

	if len(parts) == 1 {
		s.Kind = StepWhole
		return s, nil
	}

	selector := strings.Join(parts[1:], "/")
	return parseSelector(selector, &s)
}

// splitStep splits "TheoremName/<1>/<2>" into ["TheoremName", "<1>/<2>"].
func splitStep(step string) []string {
	idx := strings.Index(step, "/")
	if idx < 0 {
		return []string{step}
	}
	return []string{step[:idx], step[idx+1:]}
}

// parseSelector parses the step selector portion (after theorem name).
func parseSelector(selector string, s *Step) (Step, error) {
	// Range notation: <1>..<3> or <1>/<2>..<3>
	if idx := strings.Index(selector, ".."); idx >= 0 {
		s.Kind = StepRange
		left := strings.Trim(selector[:idx], "<>")
		right := strings.Trim(selector[idx+2:], "<>")
		s.RangeRaw = left + ".." + right

		// Check if left side has a parent prefix (e.g. "<1>/<2>..<3>")
		if parentIdx := strings.Index(left, "/"); parentIdx >= 0 {
			nestedStr := left[:parentIdx]
			n, err := parseInt(nestedStr)
			if err != nil {
				return *s, err
			}
			s.Nested = append(s.Nested, n)

			rangeLeft := left[parentIdx+1:]
			s.RangeStart, _ = parseInt(rangeLeft)
			s.RangeEnd, _ = parseInt(right)
		} else {
			s.RangeStart, _ = parseInt(left)
			s.RangeEnd, _ = parseInt(right)
		}
		return *s, nil
	}

	// Check for <*>
	if selector == "<*>" || selector == "*" {
		s.Kind = StepExact
		s.RangeAll = true
		return *s, nil
	}

	// Nested path: <1>/<2>/<3>
	for _, n := range strings.Split(selector, "/") {
		num, err := parseInt(n)
		if err != nil {
			return *s, err
		}
		s.Nested = append(s.Nested, num)
	}

	s.Kind = StepExact
	return *s, nil
}

// parseInt parses a step number string, handling suffixes like <1a>, <2>1.
func parseInt(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	numStr := ""
	for _, c := range s {
		if c >= '0' && c <= '9' {
			numStr += string(c)
		}
	}
	if numStr == "" {
		numStr = s
	}
	return strconv.Atoi(numStr)
}

// String returns the step as a string suitable for tlapm invocation.
func (s Step) String() string {
	switch s.Kind {
	case StepLine:
		return fmt.Sprintf("line:%d", s.LineNum)
	case StepWhole:
		return s.Theorem
	case StepRange:
		if s.Theorem != "" {
			return fmt.Sprintf("%s/<%s>", s.Theorem, s.RangeRaw)
		}
		return fmt.Sprintf("<%s>", s.RangeRaw)
	case StepExact:
		if s.RangeAll {
			if s.Theorem != "" {
				return fmt.Sprintf("%s/<*>", s.Theorem)
			}
			return "<*>"
		}
		if s.Theorem != "" && len(s.Nested) > 0 {
			parts := append([]string{s.Theorem}, nestedParts(s.Nested)...)
			return strings.Join(parts, "/")
		}
		if len(s.Nested) > 0 {
			return strings.Join(nestedParts(s.Nested), "/")
		}
		return s.Theorem
	default:
		return s.Theorem
	}
}

func nestedParts(ns []int) []string {
	parts := make([]string, len(ns))
	for i, n := range ns {
		parts[i] = fmt.Sprintf("<%d>", n)
	}
	return parts
}

// Validate checks that the step is well-formed.
func (s Step) Validate() error {
	if s.Kind == StepLine {
		if s.LineNum <= 0 {
			return fmt.Errorf("line number must be positive: %d", s.LineNum)
		}
		return nil
	}
	if s.RangeStart > 0 && s.RangeEnd > 0 && s.RangeStart > s.RangeEnd {
		return fmt.Errorf("range start %d > end %d", s.RangeStart, s.RangeEnd)
	}
	for _, n := range s.Nested {
		if n <= 0 {
			return fmt.Errorf("nested step must be positive: %d", n)
		}
	}
	return nil
}

// IsWhole returns true if this step selects the whole theorem.
func (s Step) IsWhole() bool {
	return s.Kind == StepWhole || (s.Kind == StepExact && len(s.Nested) == 0)
}

// HasRange returns true if this step has a range.
func (s Step) HasRange() bool {
	return s.Kind == StepRange
}

// HasNested returns true if this step has a nested path.
func (s Step) HasNested() bool {
	return s.Kind == StepExact && len(s.Nested) > 0
}

// stepRe matches step numbers like <1>, <2a>, <3>1, etc.
var stepRe = regexp.MustCompile(`<(\d+[a-z]?[0-9]*[a-z]*)>`)
