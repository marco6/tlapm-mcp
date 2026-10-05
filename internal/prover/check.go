package prover

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// CheckArgs holds the arguments for a parse and elaboration check.
type CheckArgs struct {
	Module string
	Target LineTarget
}

// CheckResult is the structured output of a parse and elaboration check.
type CheckResult struct {
	Success         bool         `json:"success"`
	Module          string       `json:"module"`
	Line            int          `json:"line,omitempty"`
	Range           *LineRange   `json:"range,omitempty"`
	Diagnostics     []Diagnostic `json:"diagnostics"`
	ObligationCount *int         `json:"obligation_count,omitempty"`
	ExitCode        *int         `json:"exit_code,omitempty"`
	Stderr          *string      `json:"stderr,omitempty"`
	ErrorCode       string       `json:"error_code,omitempty"`
}

// Diagnostic is a message reported while parsing or elaborating a module.
// A zero line or column means TLAPM did not report that location.
type Diagnostic struct {
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
}

// Check parses and elaborates the selected module without running proof backends.
func (p *Prover) Check(ctx context.Context, args CheckArgs) (CheckResult, error) {
	if err := args.Target.validateOptional(); err != nil {
		return CheckResult{}, WrapToolError("check", ErrInvalidRange, err.Error(), "")
	}
	if args.Module == "" {
		return CheckResult{}, WrapToolError("check", ErrInvalidModule, "module path is required", "")
	}
	if err := ensureModuleExists("check", p.resolveFS(), args.Module); err != nil {
		return CheckResult{}, err
	}
	if err := ensureTLAPMBinary("check"); err != nil {
		LogError("check", err)
		return CheckResult{}, err
	}

	cmd := buildCheckCmd(ctx, args)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	commandErr := cmd.Run()
	if commandErr != nil {
		if ctx.Err() != nil {
			return CheckResult{}, RequestContextError("check", ctx.Err())
		}
		var exitErr *exec.ExitError
		if !errors.As(commandErr, &exitErr) {
			return CheckResult{}, ParseExitCodeError("check", stderr.Bytes(), commandErr)
		}
	}

	result := CheckResult{
		Module:      args.Module,
		Line:        args.Target.Line,
		Range:       args.Target.Range,
		Diagnostics: []Diagnostic{},
	}
	output := stdout.String() + "\n" + stderr.String()
	result.Diagnostics = parseDiagnostics(output)
	result.ObligationCount = parseObligationCount(output)
	result.Success = commandErr == nil && !hasErrorDiagnostic(result.Diagnostics)
	if !result.Success {
		result.ErrorCode = string(classifyTLAPMOutput(output, commandErr != nil))
	}

	if commandErr != nil {
		var exitErr *exec.ExitError
		if errors.As(commandErr, &exitErr) {
			code := exitErr.ExitCode()
			result.ExitCode = &code
		}
		stderrText := stderr.String()
		result.Stderr = &stderrText
		if !hasErrorDiagnostic(result.Diagnostics) {
			message := "TLAPM exited unsuccessfully"
			if result.ExitCode != nil {
				message = fmt.Sprintf("TLAPM exited with code %d", *result.ExitCode)
			}
			result.Diagnostics = append(result.Diagnostics, Diagnostic{Severity: "error", Message: message})
		}
	}
	return result, nil
}

func buildCheckCmd(ctx context.Context, args CheckArgs) *exec.Cmd {
	cmdArgs := []string{"--summary", "-N"}
	if args.Target.Line > 0 || args.Target.Range != nil {
		cmdArgs = args.Target.appendCommandArgs(cmdArgs)
	}
	cmdArgs = append(cmdArgs, args.Module)
	return exec.CommandContext(ctx, "tlapm", cmdArgs...)
}

var (
	locatedDiagnosticRe  = regexp.MustCompile(`^File "[^"]+", line ([0-9]+), characters? ([0-9]+)(?:-([0-9]+))?:?\s*$`)
	severityDiagnosticRe = regexp.MustCompile(`(?i)^\[?(error|warning|info)\]?:\s*(.*)$`)
	obligationCountRe    = regexp.MustCompile(`(?m)^\s*obligations_count\s*=\s*([0-9]+)\s*$`)
)

func parseDiagnostics(output string) []Diagnostic {
	lines := strings.Split(output, "\n")
	diagnostics := make([]Diagnostic, 0)
	locatedCount := 0

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if match := locatedDiagnosticRe.FindStringSubmatch(line); match != nil {
			lineNumber, _ := strconv.Atoi(match[1])
			column, _ := strconv.Atoi(match[2])
			message := ""
			for j := i + 1; j < len(lines); j++ {
				message = strings.TrimSpace(lines[j])
				if message != "" {
					i = j
					break
				}
			}
			severity := "error"
			if match := severityDiagnosticRe.FindStringSubmatch(message); match != nil {
				severity = strings.ToLower(match[1])
				message = match[2]
			}
			if message == "" {
				message = "TLAPM reported a diagnostic at this location"
			}
			diagnostics = append(diagnostics, Diagnostic{
				Line:     lineNumber,
				Column:   column,
				Severity: severity,
				Message:  message,
			})
			locatedCount++
			continue
		}

		if match := severityDiagnosticRe.FindStringSubmatch(line); match != nil {
			severity := strings.ToLower(match[1])
			message := match[2]
			if locatedCount > 0 && strings.Contains(strings.ToLower(message), "could not parse") {
				continue
			}
			diagnostics = append(diagnostics, Diagnostic{Severity: severity, Message: message})
		}
	}

	return diagnostics
}

func parseObligationCount(output string) *int {
	if match := obligationCountRe.FindStringSubmatch(output); match != nil {
		count, err := strconv.Atoi(match[1])
		if err == nil {
			return &count
		}
	}

	lines := strings.Split(output, "\n")
	for i, line := range lines {
		if strings.Contains(line, "@!!type:obligationsnumber") {
			for j := i + 1; j < len(lines); j++ {
				if strings.TrimSpace(lines[j]) == "@!!END" {
					break
				}
				if strings.HasPrefix(strings.TrimSpace(lines[j]), "@!!count:") {
					count, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(lines[j]), "@!!count:"))
					if err == nil {
						return &count
					}
					break
				}
			}
		}
	}
	return nil
}

func hasErrorDiagnostic(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == "error" {
			return true
		}
	}
	return false
}
