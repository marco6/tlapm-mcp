package prover

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Result is the structured output of a prove invocation.
type Result struct {
	Success          bool               `json:"success"`
	Module           string             `json:"module"`
	Line             int                `json:"line,omitempty"`
	Range            *LineRange         `json:"range,omitempty"`
	Solver           string             `json:"solver"`
	TotalTime        float64            `json:"total_time_seconds"`
	Timing           map[string]float64 `json:"timing"`
	Obligations      []Obligation       `json:"obligations,omitempty"`
	ProofText        string             `json:"proof_text"`
	FingerprintsUsed string             `json:"fingerprints_used"`
	ErrorCode        string             `json:"error_code,omitempty"` // classification for incomplete or invalid TLAPM output
}

// Obligation is a single proof obligation with its status.
type Obligation struct {
	Line   int    `json:"line,omitempty"`
	Text   string `json:"text"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// RaceResult is the structured output of a race invocation.
type RaceResult struct {
	Fastest RaceSolverResult   `json:"fastest"`
	Results []RaceSolverResult `json:"results"`
}

// RaceSolverResult is a single solver result in a race.
type RaceSolverResult struct {
	Solver           string  `json:"solver"`
	Success          bool    `json:"success"`
	TotalTimeSeconds float64 `json:"total_time_seconds"`
	// -1 means TLAPM did not provide a trustworthy failed-obligation count.
	ObligationsFailed int    `json:"obligations_failed"`
	Error             string `json:"error,omitempty"`
}

// parseResult parses tlapm output into a Result.
func parseResult(output string, args ProveArgs) (Result, error) {
	r := Result{
		Module:           moduleName(args.Module),
		Line:             args.Target.Line,
		Range:            args.Target.Range,
		Solver:           args.Solver,
		FingerprintsUsed: args.FPModes.String(),
		Timing:           make(map[string]float64),
	}

	infoRe := regexp.MustCompile(`^\[INFO\]:\s*(.*)`)
	errorRe := regexp.MustCompile(`^\[ERROR\]:\s*(.*)`)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if m := infoRe.FindStringSubmatch(line); m != nil && r.ProofText == "" {
			r.ProofText = m[1]
		}
		if m := errorRe.FindStringSubmatch(line); m != nil {
			r.Obligations = append(r.Obligations, Obligation{
				Text:   m[1],
				Status: "failed",
				Error:  m[1],
			})
		}
	}
	r.Success = allObligationsProved(output) && len(r.Obligations) == 0

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
		if strings.Contains(output, "corrupt") || strings.Contains(output, "invalid fingerprint") {
			r.ErrorCode = string(ErrFingerprintCorrupted)
		} else {
			r.ErrorCode = string(ErrParse)
		}
	}

	return r, nil
}

var proofCompletionRe = regexp.MustCompile(`^\[INFO\]:\s*All\s+\d+\s+obligations?\s+proved\.?$`)

func allObligationsProved(output string) bool {
	complete := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[ERROR]:") {
			return false
		}
		if proofCompletionRe.MatchString(line) {
			complete = true
		}
	}
	return complete
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

// raceSolvers runs all methods in parallel and returns sorted results.
func raceSolvers(ctx context.Context, args RaceArgs, methods []string) (RaceResult, error) {
	if err := args.Target.validate(); err != nil {
		return RaceResult{}, err
	}
	if args.Threads <= 0 {
		return RaceResult{}, fmt.Errorf("threads must be a positive integer")
	}
	type result struct {
		solver string
		res    RaceSolverResult
		err    error
	}

	results := make([]result, len(methods))
	var mu sync.Mutex
	var wg sync.WaitGroup

	parallelism := args.Threads
	if parallelism > len(methods) {
		parallelism = len(methods)
	}
	sem := make(chan struct{}, parallelism)

	for i, method := range methods {
		wg.Add(1)
		go func(idx int, method string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			r := RaceSolverResult{Solver: method, ObligationsFailed: -1}
			cmdArgs := buildRaceCmdArgs(method, args)
			cmd := exec.Command("tlapm", cmdArgs...)
			out, err := cmd.CombinedOutput()
			r.TotalTimeSeconds = time.Since(start).Seconds()

			if errors.Is(err, exec.ErrNotFound) {
				r.Error = fmt.Sprintf("tlapm binary not available: %v", err)
			} else if err != nil {
				r.Error = strings.TrimSpace(string(out))
				if strings.Contains(strings.ToLower(r.Error), "corrupt") ||
					strings.Contains(strings.ToLower(r.Error), "invalid fingerprint") {
					r.Error = fmt.Sprintf("corrupted fingerprint for method %s", method)
				}
			} else if allObligationsProved(string(out)) {
				r.Success = true
				r.ObligationsFailed = 0
			} else {
				r.Error = "TLAPM output did not confirm that all obligations were proved"
			}

			mu.Lock()
			results[idx].res = r
			mu.Unlock()
		}(i, method)
	}

	wg.Wait()

	// Build RaceResult with sorted results
	r := RaceResult{Results: make([]RaceSolverResult, len(methods))}
	for i, res := range results {
		r.Results[i] = res.res
	}

	// Sort by time ascending
	for i := 0; i < len(r.Results)-1; i++ {
		for j := i + 1; j < len(r.Results); j++ {
			if r.Results[j].TotalTimeSeconds < r.Results[i].TotalTimeSeconds {
				r.Results[i], r.Results[j] = r.Results[j], r.Results[i]
			}
		}
	}

	// Fastest = first successful, or first overall
	for _, res := range r.Results {
		if res.Success {
			r.Fastest = res
			break
		}
	}
	if r.Fastest.Solver == "" && len(r.Results) > 0 {
		r.Fastest = r.Results[0]
	}

	return r, nil
}

func buildRaceCmdArgs(method string, args RaceArgs) []string {
	cmdArgs := []string{"--timing", "--method", method}
	cmdArgs = args.Target.appendCommandArgs(cmdArgs)
	switch args.FPModes {
	case FPNo:
		cmdArgs = append(cmdArgs, "--nofp")
	case FPCheck:
		cmdArgs = append(cmdArgs, "--safefp")
	}
	return append(cmdArgs, args.Module)
}
