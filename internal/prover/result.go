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
	Warnings         []Warning          `json:"warnings,omitempty"`
	ProofText        string             `json:"proof_text"`
	FingerprintsUsed string             `json:"fingerprints_used"`
	ErrorCode        string             `json:"error_code,omitempty"` // classification for incomplete or invalid TLAPM output
}

// Warning is a compact, non-fatal hint about a proof step.
type Warning struct {
	Line int    `json:"line"`
	What string `json:"what"`
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
	if r.Success {
		r.Warnings = buildWarnings(r.Obligations)
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

// byTactics maps TLAPM backend names to identifiers accepted in a BY clause.
var byTactics = map[string]string{
	"smt":      "SMT",
	"smt3":     "SMT",
	"z3":       "Z3",
	"z33":      "Z3",
	"cvc4":     "CVC4",
	"cvc33":    "CVC4",
	"verit":    "veriT",
	"spass":    "Spass",
	"zipper":   "Zipper",
	"yices":    "Yices",
	"yices3":   "Yices",
	"ls4":      "LS4",
	"zenon":    "Zenon",
	"isabelle": "Isa",
}

func buildWarnings(obligations []Obligation) []Warning {
	warnings := make([]Warning, 0)
	seen := make(map[string]bool)
	for _, obligation := range obligations {
		if obligation.Status != "proved" || obligation.Line <= 0 {
			continue
		}
		successful := strings.ToLower(obligation.prover)
		tactic, ok := byTactics[successful]
		if !ok || !hadBackendFallback(obligation, successful) {
			continue
		}
		key := strconv.Itoa(obligation.Line) + "\x00" + tactic
		if seen[key] {
			continue
		}
		seen[key] = true
		warnings = append(warnings, Warning{
			Line: obligation.Line,
			What: "missing `BY " + tactic + "`",
		})
	}
	if len(warnings) == 0 {
		return nil
	}
	return warnings
}

// hadBackendFallback reports whether a known backend failed before the backend
// that proved the obligation.
func hadBackendFallback(obligation Obligation, successful string) bool {
	for _, attempt := range obligation.attempts {
		if attempt.status == "proved" {
			break
		}
		if attempt.status != "failed" && attempt.status != "interrupted" {
			continue
		}
		prover := strings.ToLower(attempt.prover)
		if prover == "" || prover == successful {
			continue
		}
		if _, ok := byTactics[prover]; ok {
			return true
		}
	}
	return false
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
