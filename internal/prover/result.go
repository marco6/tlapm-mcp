package prover

import (
	"context"
	"fmt"
	"io/fs"

	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Result is the structured output of a prove invocation.
type Result struct {
	Success            bool              `json:"success"`
	Module             string            `json:"module"`
	Step               string            `json:"step"`
	Solver             string            `json:"solver"`
	TotalTime          float64           `json:"total_time_seconds"`
	Timing             map[string]float64 `json:"timing"`
	Obligations        []Obligation      `json:"obligations,omitempty"`
	ProofText          string            `json:"proof_text"`
	FingerprintsUsed   string            `json:"fingerprints_used"`
	ErrorCode          string            `json:"error_code,omitempty"` // structured error code from tlapm
}

// Obligation is a single proof obligation with its status.
type Obligation struct {
	Line   int    `json:"line"`
	Text   string `json:"text"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// RaceResult is the structured output of a race invocation.
type RaceResult struct {
	Fastest RaceSolverResult `json:"fastest"`
	Results []RaceSolverResult `json:"results"`
}

// RaceSolverResult is a single solver result in a race.
type RaceSolverResult struct {
	Solver              string  `json:"solver"`
	Success             bool    `json:"success"`
	TotalTimeSeconds    float64 `json:"total_time_seconds"`
	ObligationsFailed   int     `json:"obligations_failed"`
	Error               string  `json:"error,omitempty"`
}

// ListResult is the structured output of list_theorems.
type ListResult struct {
	Module  string   `json:"module"`
	File    string   `json:"file"`
	Targets []Target `json:"targets"`
}

// Target is a provable target in a TLA+ module.
type Target struct {
	Name         string `json:"name"`
	Line         int    `json:"line"`
	Kind         string `json:"kind"`         // THEOREM, AXIOM, DEFINITION
	HasSubproofs bool   `json:"has_subproofs"`
	SubproofPath string `json:"subproof_path,omitempty"`
}

// parseResult parses tlapm output into a Result.
func parseResult(output string, args ProveArgs) (Result, error) {
	r := Result{
		Module:           moduleName(args.Module),
		Step:             args.Step,
		Solver:           args.Solver,
		FingerprintsUsed: args.FPModes.String(),
		Timing:           make(map[string]float64),
	}

	// Parse [INFO] / [ERROR] lines
	infoRe := regexp.MustCompile(`^\[INFO\]:\s*(.*)`)
	errorRe := regexp.MustCompile(`^\[ERROR\]:\s*(.*)`)

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)

		if m := infoRe.FindStringSubmatch(line); m != nil {
			if r.ProofText == "" {
				r.ProofText = m[1]
			}
			if strings.Contains(m[1], "obligation proved") {
				r.Success = true
			}
		}
		if m := errorRe.FindStringSubmatch(line); m != nil {
			r.Obligations = append(r.Obligations, Obligation{
				Text:   m[1],
				Status: "failed",
				Error:  m[1],
			})
		}
	}

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
			r.ErrorCode = string(ErrExitCode)
		}
	}

	return r, nil
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

// raceSolvers runs all solvers in parallel and returns sorted results.
func raceSolvers(ctx context.Context, args RaceArgs, solvers []string) (RaceResult, error) {
	type result struct {
		solver string
		res    RaceSolverResult
		err    error
	}

	results := make([]result, len(solvers))
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit parallelism
	sem := make(chan struct{}, args.Threads)

	for i, solver := range solvers {
		wg.Add(1)
		go func(idx int, s string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			start := time.Now()
			r := RaceSolverResult{Solver: s}

			cmd := exec.Command("tlapm", "--timing", "--solver", s, args.Module)
			if args.Step != "" {
				cmd.Args = append(cmd.Args, args.Step)
			}
			switch args.FPModes {
			case FPNo:
				cmd.Args = append(cmd.Args, "--nofp")
			case FPCheck:
				cmd.Args = append(cmd.Args, "--safefp")
			case FPDefault:
				// default: use cached (no extra flag needed)
			}
			out, err := cmd.CombinedOutput()
			r.TotalTimeSeconds = time.Since(start).Seconds()

			if err != nil {
				r.Success = false
				r.Error = strings.TrimSpace(string(out))
				// Distinguish solver-unavailable from proof-failure
				if strings.Contains(err.Error(), "executable file not found") {
					r.Error = fmt.Sprintf("solver %s not available: %v", s, err)
					r.ObligationsFailed = -1 // sentinel: solver unavailable
				} else if strings.Contains(strings.ToLower(r.Error), "corrupt") ||
					strings.Contains(strings.ToLower(r.Error), "invalid fingerprint") {
					r.Error = fmt.Sprintf("corrupted fingerprint for solver %s", s)
				}
			} else {
				r.Success = true
			}

			mu.Lock()
			results[idx].res = r
			mu.Unlock()
		}(i, solver)
	}

	wg.Wait()

	// Build RaceResult with sorted results
	r := RaceResult{Results: make([]RaceSolverResult, len(solvers))}
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

// listTheorems parses a TLA+ file for provable targets.
func listTheorems(fsys fs.FS, ctx context.Context, modulePath string, includeSubproofs bool) (ListResult, error) {
	if _, err := fs.Stat(fsys, modulePath); err != nil {
		if osIsNotExist(err) {
			return ListResult{}, WrapToolError("list_theorems", ErrModuleNotFound,
				"module file not found", modulePath)
		}
		return ListResult{}, WrapError("list_theorems", ErrIO,
			"failed to read module file", err)
	}
	data, err := fs.ReadFile(fsys, modulePath)
	if err != nil {
		return ListResult{}, WrapError("list_theorems", ErrIO,
			"failed to read module file", err)
	}

	result := ListResult{
		Module: moduleName(modulePath),
		File:   modulePath,
	}

	theoremRe := regexp.MustCompile(`^(\s*)(<\d+[a-z]?[0-9]*[a-z]*>\.?\d*[\.\w]*)?\s*(THEOREM|AXIOM|DEFINE)\s+(\w+)\s*(.*)$`)
	stepRe := regexp.MustCompile(`<(\d+[a-z]?[0-9]*[a-z]*)>`)

	lines := strings.Split(string(data), "\n")
	var currentSubproofs []string

	for i, line := range lines {
		if m := theoremRe.FindStringSubmatch(line); m != nil {
			name := m[4]

			target := Target{
				Name:   name,
				Line:   i + 1,
				Kind:   m[3],
			}

			if includeSubproofs && len(currentSubproofs) > 0 {
				target.HasSubproofs = true
				target.SubproofPath = name + "/" + strings.Join(currentSubproofs, "/")
			}

			result.Targets = append(result.Targets, target)
			currentSubproofs = nil
		}

		if includeSubproofs {
			for _, s := range stepRe.FindAllStringSubmatch(line, -1) {
				currentSubproofs = append(currentSubproofs, s[1])
			}
		}
	}

	return result, nil
}

