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

// ProveArgs holds the arguments for a prove invocation.
type ProveArgs struct {
	Module   string // path to .tla file
	Step     string // theorem/subproof selector
	Solver   string // solver name (empty = tlapm default)
	UseFP    bool   // use fingerprints
	Threads  int    // worker threads
	FPCheck  bool   // --safefp version check
}

// RaceArgs holds the arguments for a race invocation.
type RaceArgs struct {
	Module  string
	Step    string
	UseFP   bool
	FPCheck bool
	Threads int // max parallel invocations
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
	if !args.UseFP {
		cmdArgs = append(cmdArgs, "--nofp")
	} else if args.FPCheck {
		cmdArgs = append(cmdArgs, "--safefp")
	}
	if args.Step != "" {
		cmdArgs = append(cmdArgs, args.Step)
	}
	cmdArgs = append(cmdArgs, args.Module)

	return exec.Command("tlapm", cmdArgs...)
}
