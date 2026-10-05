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

func (t LineTarget) validateOptional() error {
	if t.Line == 0 && t.Range == nil {
		return nil
	}
	return t.validate()
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
	FPModes FPMode     // fingerprint mode (default: use cached)
}

// RaceArgs holds the arguments for a race invocation.
type RaceArgs struct {
	Module  string     // path to .tla file
	Target  LineTarget // single source line or inclusive line range
	FPModes FPMode     // fingerprint mode (default: use cached)
}

// Prove runs tlapm on the given module/target and parses the tool output.
func (p *Prover) Prove(ctx context.Context, args ProveArgs) (Result, error) {
	cmd, err := p.buildProveCmd(ctx, args)
	if err != nil {
		return Result{}, WrapToolError("prove", ErrInvalidRange, err.Error(), "")
	}
	if err := ensureModuleExists("prove", p.resolveFS(), args.Module); err != nil {
		return Result{}, err
	}
	if err := ensureTLAPMBinary("prove"); err != nil {
		LogError("prove", err)
		return Result{}, err
	}

	output, commandErr := cmd.CombinedOutput()
	var toolErr error
	if commandErr != nil {
		if ctx.Err() != nil {
			return Result{}, RequestContextError("prove", ctx.Err())
		}
		toolErr = ParseExitCodeError("prove", output, commandErr)
		LogError("prove", toolErr)
		if tErr, ok := toolErr.(*TLAPMError); ok && tErr.Code == ErrTLAPMNotFound {
			return Result{}, toolErr
		}
	}

	result, err := parseResult(string(output), args)
	if err != nil {
		return Result{}, err
	}
	if commandErr != nil {
		result.Success = false
		if tErr, ok := toolErr.(*TLAPMError); ok {
			result.ErrorCode = string(tErr.Code)
		} else {
			result.ErrorCode = string(ErrExitCode)
		}
	}
	return result, nil
}

// Race runs the supported TLAPM methods in parallel and returns sorted results.
func (p *Prover) Race(ctx context.Context, args RaceArgs) (RaceResult, error) {
	if err := args.Target.validate(); err != nil {
		return RaceResult{}, WrapToolError("race", ErrInvalidRange, err.Error(), "")
	}
	if err := ensureModuleExists("race", p.resolveFS(), args.Module); err != nil {
		return RaceResult{}, err
	}
	if err := ensureTLAPMBinary("race"); err != nil {
		LogError("race", err)
		return RaceResult{}, err
	}
	methods := []string{"z3", "smt", "zenon", "auto", "blast", "force", "cvc4", "yices", "verit", "spass", "zipper", "ls4", "fail"}
	result, err := raceSolvers(ctx, args, methods)
	if ctx.Err() != nil {
		return RaceResult{}, RequestContextError("race", ctx.Err())
	}
	return result, err
}

// buildProveCmd constructs the exec.Cmd for a prove invocation.
func (p *Prover) buildProveCmd(ctx context.Context, args ProveArgs) (*exec.Cmd, error) {
	if err := args.Target.validate(); err != nil {
		return nil, err
	}
	cmdArgs := []string{"--timing"}
	cmdArgs = args.Target.appendCommandArgs(cmdArgs)
	switch args.FPModes {
	case FPNo:
		cmdArgs = append(cmdArgs, "--nofp")
	case FPCheck:
		cmdArgs = append(cmdArgs, "--safefp")
	case FPDefault:
		// default: use cached (no extra flag needed)
	}
	cmdArgs = append(cmdArgs, args.Module)

	return exec.CommandContext(ctx, "tlapm", cmdArgs...), nil
}
