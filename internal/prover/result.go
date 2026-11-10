package prover

import (
	"regexp"
	"strconv"
	"strings"
)

// Result is the structured output of a prove invocation.
type Result struct {
	Success          bool               `json:"success"`
	Module           string             `json:"module"`
	Line             int                `json:"line,omitempty"`
	Range            *LineRange         `json:"range,omitempty"`
	TotalTime        float64            `json:"total_time_seconds"`
	Timing           map[string]float64 `json:"timing"`
	ObligationCount  *int               `json:"obligation_count,omitempty"`
	Obligations      []Obligation       `json:"obligations,omitempty"`
	ProofText        string             `json:"proof_text"`
	FingerprintsUsed string             `json:"fingerprints_used"`
	ErrorCode        string             `json:"error_code,omitempty"` // classification for incomplete or invalid TLAPM output
}

// parseResult parses tlapm output into a Result.
func parseResult(output string, args ProveArgs) (Result, error) {
	r := Result{
		Module:           moduleName(args.Module),
		Line:             args.Target.Line,
		Range:            args.Target.Range,
		FingerprintsUsed: args.FPModes.String(),
		Timing:           make(map[string]float64),
	}
	r.Obligations, r.ObligationCount = parseToolboxObligations(output)
	if r.ObligationCount == nil {
		if count, found := completedObligationCount(output); found {
			r.ObligationCount = &count
		}
	}

	infoRe := regexp.MustCompile(`^\[INFO\]:\s*(.*)`)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if m := infoRe.FindStringSubmatch(line); m != nil && r.ProofText == "" {
			r.ProofText = m[1]
		}
	}
	r.Success = allObligationsProved(output) && len(parseObligationErrors(output)) == 0
	finalizeObligations(r.Obligations, r.Success, strings.Join(parseObligationErrors(output), "; "))
	if r.ObligationCount == nil && hasStructuredObligations(r.Obligations) {
		count := len(r.Obligations)
		r.ObligationCount = &count
	}
	r.Obligations = unresolvedObligations(r.Obligations)

	// Parse timing section
	parseTiming(output, &r)

	// Parse total time
	r.TotalTime = parseTotalTime(output)
	if r.TotalTime == 0 {
		if v, ok := r.Timing["total"]; ok {
			r.TotalTime = v
		}
	}

	// Set error code based on success state and output content
	if !r.Success {
		r.ErrorCode = string(classifyTLAPMOutput(output, false))
	}

	return r, nil
}

var proofCompletionRe = regexp.MustCompile(`^\[INFO\]:\s*All\s+(\d+)\s+obligations?\s+proved\.?$`)

func completedObligationCount(output string) (int, bool) {
	count := 0
	found := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if m := proofCompletionRe.FindStringSubmatch(line); m != nil {
			n, err := strconv.Atoi(m[1])
			if err == nil && (!found || n > count) {
				count = n
				found = true
			}
		}
	}
	return count, found
}

func hasZeroObligationCompletion(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "[ERROR]:") {
			return false
		}
	}
	count, found := completedObligationCount(output)
	return found && count == 0
}

func allObligationsProved(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[ERROR]:") {
			return false
		}
	}
	count, found := completedObligationCount(output)
	return found && count > 0
}

// moduleName extracts the module name from a file path.
func moduleName(path string) string {
	base := path
	if idx := strings.LastIndex(path, "/"); idx >= 0 {
		base = path[idx+1:]
	}
	return strings.TrimSuffix(base, ".tla")
}

var timingRe = regexp.MustCompile(`(\w+)\s+\|\s+([\d.]+)`)
var totalTimeRe = regexp.MustCompile(`Total\s+time:\s+([\d.]+)\s+s`)

func parseTiming(output string, r *Result) {
	lines := strings.Split(output, "\n")
	inTiming := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		clean := strings.TrimPrefix(strings.TrimPrefix(line, "(*"), "(* ")
		if strings.Contains(line, "operation") || strings.HasPrefix(clean, "Timing") {
			inTiming = true
			continue
		}
		if inTiming && strings.Contains(line, "|") {
			if m := timingRe.FindStringSubmatch(line); m != nil {
				if v, err := strconv.ParseFloat(m[2], 64); err == nil {
					r.Timing[m[1]] = v
				}
			} else if strings.Contains(line, "---") {
				inTiming = false
			}
		}
	}
}

func parseTotalTime(output string) float64 {
	if m := totalTimeRe.FindStringSubmatch(output); m != nil {
		if v, err := strconv.ParseFloat(m[1], 64); err == nil {
			return v
		}
	}
	return 0
}
