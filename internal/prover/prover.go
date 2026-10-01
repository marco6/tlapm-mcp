// Package prover orchestrates tlapm invocations.
package prover

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"strconv"
)

// Prover runs tlapm and parses its output.
type Prover struct {
	// FS is used to validate module paths; module contents are never read.
	// When nil, os.DirFS(".") is used.
	FS fs.FS
}

// New creates a new Prover with the default filesystem (os.DirFS(".")).
func New() *Prover {
	return &Prover{}
}

// NewWithFS creates a new Prover with the given filesystem.
func NewWithFS(f fs.FS) *Prover {
	return &Prover{FS: f}
}

// resolveFS returns the filesystem to use (prover's FS or os.DirFS(".")).
func (p *Prover) resolveFS() fs.FS {
	if p.FS == nil {
		return os.DirFS(".")
	}
	return p.FS
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

// LineRange identifies an inclusive source line range.
type LineRange struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// LineTarget identifies either one source line or an inclusive line range.
type LineTarget struct {
	Line  int
	Range *LineRange
}

// validate rejects missing, conflicting, and invalid line targets.
func (t LineTarget) validate() error {
	if t.Line > 0 && t.Range == nil {
		return nil
	}
	if t.Line == 0 && t.Range != nil && t.Range.Start > 0 && t.Range.End >= t.Range.Start {
		return nil
	}
	return errors.New("exactly one positive line or valid range is required")
}

// appendCommandArgs adds tlapm's native target flags to args.
func (t LineTarget) appendCommandArgs(args []string) []string {
	if t.Range == nil {
		return append(args, "--line", strconv.Itoa(t.Line))
	}
	return append(args, "--toolbox", strconv.Itoa(t.Range.Start), strconv.Itoa(t.Range.End))
}

// ProveArgs holds the arguments for a prove invocation.
type ProveArgs struct {
	Module  string     // path to .tla file
	Target  LineTarget // single source line or inclusive line range
	Solver  string     // solver name (empty = tlapm default)
	FPModes FPMode     // fingerprint mode (default: use cached)
	Threads int        // worker threads
}

// RaceArgs holds the arguments for a race invocation.
type RaceArgs struct {
	Module  string     // path to .tla file
	Target  LineTarget // single source line or inclusive line range
	FPModes FPMode     // fingerprint mode (default: use cached)
	Threads int        // max parallel invocations
}

// Prove runs tlapm on the given module/target and parses the tool output.
func (p *Prover) Prove(ctx context.Context, args ProveArgs) (Result, error) {
	cmd, err := p.buildProveCmd(args)
	if err != nil {
		return Result{}, err
	}
	if err := EnsureModuleExists(p.resolveFS(), args.Module); err != nil {
		return Result{}, err
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		tlapmErr := ParseExitCodeError("prove", output, err)
		LogError("prove", tlapmErr)
	}
	return parseResult(string(output), args)
}

// Race runs all available solvers in parallel and returns sorted results.
func (p *Prover) Race(ctx context.Context, args RaceArgs) (RaceResult, error) {
	solvers := []string{"z3", "smt", "zenon", "auto", "blast", "force", "cvc4", "yices", "verit", "spass", "zipper", "ls4", "fail"}
	return raceSolvers(ctx, args, solvers)
}

// buildProveCmd constructs the exec.Cmd for a prove invocation.
func (p *Prover) buildProveCmd(args ProveArgs) (*exec.Cmd, error) {
	if err := args.Target.validate(); err != nil {
		return nil, err
	}
	cmdArgs := []string{"--timing"}
	cmdArgs = args.Target.appendCommandArgs(cmdArgs)

	if args.Solver != "" {
		cmdArgs = append(cmdArgs, "--solver", args.Solver)
	}
	if args.Threads > 1 {
		cmdArgs = append(cmdArgs, "--threads", strconv.Itoa(args.Threads))
	}
	switch args.FPModes {
	case FPNo:
		cmdArgs = append(cmdArgs, "--nofp")
	case FPCheck:
		cmdArgs = append(cmdArgs, "--safefp")
	case FPDefault:
		// default: use cached (no extra flag needed)
	}
	cmdArgs = append(cmdArgs, args.Module)

	return exec.Command("tlapm", cmdArgs...), nil
}
