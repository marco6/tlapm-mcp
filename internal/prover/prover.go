// Package prover orchestrates tlapm invocations.
package prover

import (
	"context"
	"os"
	"os/exec"
)

// Prover runs tlapm and parses its output.
type Prover struct{}

// New creates a new Prover.
func New() *Prover {
	return &Prover{}
}

// FPMode controls how fingerprints (proof cache) are used.
// - FPDefault (0): use cached results (equivalent to --usefp).
// - FPNo (1): ignore existing fingerprints (equivalent to --nofp).
// - FPCheck (2): load fingerprints but validate tlapm/zenon/Isabelle versions (equivalent to --safefp).
type FPMode int

const (
	FPDefault FPMode = iota
	FPNo
	FPCheck
)

// String returns the human-readable fingerprint mode.
func (m FPMode) String() string {
	switch m {
	case FPDefault:
		return "use"
	case FPNo:
		return "no"
	case FPCheck:
		return "check"
	default:
		return "use"
	}
}

// ProveArgs holds the arguments for a prove invocation.
type ProveArgs struct {
	Module   string    // path to .tla file
	Step     string    // theorem/subproof selector
	Solver   string    // solver name (empty = tlapm default)
	FPModes  FPMode    // fingerprint mode (default: use cached)
	Threads  int       // worker threads
}

// RaceArgs holds the arguments for a race invocation.
type RaceArgs struct {
	Module  string   // path to .tla file
	Step    string   // theorem/subproof selector
	FPModes FPMode   // fingerprint mode (default: use cached)
	Threads int      // max parallel invocations
}

// ListTheoremsArgs holds the arguments for list_theorems.
type ListTheoremsArgs struct {
	Module           string
	IncludeSubproofs bool
}

// Prove runs tlapm on the given module/step and returns the parsed result.
func (p *Prover) Prove(ctx context.Context, args ProveArgs) (Result, error) {
	cmd := p.buildProveCmd(args)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Still parse output even on non-zero exit (partial proofs)
	}
	return parseResult(string(output), args)
}

// Race runs all available solvers in parallel and returns sorted results.
func (p *Prover) Race(ctx context.Context, args RaceArgs) (RaceResult, error) {
	solvers := []string{"z3", "smt", "zenon", "auto", "blast", "force", "cvc4", "yices", "verit", "spass", "zipper", "ls4", "fail"}
	return raceSolvers(ctx, args, solvers)
}

// ListTheorems parses the TLA+ file and returns all provable targets.
func (p *Prover) ListTheorems(ctx context.Context, args ListTheoremsArgs) (ListResult, error) {
	return listTheorems(ctx, args.Module, args.IncludeSubproofs)
}
// ResolveRangeArgs holds the arguments for a resolve_range invocation.
type ResolveRangeArgs struct {
	Module string // path to .tla file
	Step   string // range step selector, e.g. "Correctness/<1>..<3>"
}

// ResolveRange resolves a range step against the DFS tree built from the module file.
func (p *Prover) ResolveRange(ctx context.Context, args ResolveRangeArgs) (ResolvedRange, error) {
	content, err := os.ReadFile(args.Module)
	if err != nil {
		return ResolvedRange{}, err
	}
	s, err := ParseStep(args.Step)
	if err != nil {
		return ResolvedRange{}, err
	}
	return s.ResolveStepRange(string(content))
}

// buildProveCmd constructs the exec.Cmd for a prove invocation.
func (p *Prover) buildProveCmd(args ProveArgs) *exec.Cmd {
	cmdArgs := []string{"--timing"}

	if args.Solver != "" {
		cmdArgs = append(cmdArgs, "--solver", args.Solver)
	}
	if args.Threads > 1 {
		cmdArgs = append(cmdArgs, "--threads", string(rune('0'+args.Threads)))
	}
	switch args.FPModes {
	case FPNo:
		cmdArgs = append(cmdArgs, "--nofp")
	case FPCheck:
		cmdArgs = append(cmdArgs, "--safefp")
	case FPDefault:
		// default: use cached (no extra flag needed)
	}
	if args.Step != "" {
		cmdArgs = append(cmdArgs, args.Step)
	}
	cmdArgs = append(cmdArgs, args.Module)

	return exec.Command("tlapm", cmdArgs...)
}
