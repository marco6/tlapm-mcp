package prover

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrCode identifies the category of a tool failure.
type ErrCode string

const (
	ErrInvalidModule  ErrCode = "INVALID_MODULE"
	ErrInvalidRange   ErrCode = "INVALID_RANGE"
	ErrInvalidRequest ErrCode = "INVALID_REQUEST"
	// ErrTLAPMNotFound is returned when the tlapm binary is not on PATH.
	ErrTLAPMNotFound ErrCode = "TLAPM_NOT_FOUND"
	// ErrModuleNotFound is retained as an alias for ErrInvalidModule.
	ErrModuleNotFound ErrCode = ErrInvalidModule
	// ErrSolverUnavailable is returned when a solver binary is missing or incompatible.
	ErrSolverUnavailable ErrCode = "SOLVER_UNAVAILABLE"
	// ErrFingerprintCorrupted is returned when the fingerprint cache file is corrupt.
	ErrFingerprintCorrupted ErrCode = "FINGERPRINT_CORRUPTED"
	ErrTLAPMParse           ErrCode = "TLAPM_PARSE_ERROR"
	// ErrParse is retained as an alias for ErrTLAPMParse.
	ErrParse            ErrCode = ErrTLAPMParse
	ErrProofFailed      ErrCode = "PROOF_FAILED"
	ErrBackendTimeout   ErrCode = "BACKEND_TIMEOUT"
	ErrBackendFailure   ErrCode = "BACKEND_FAILURE"
	ErrRequestTimeout   ErrCode = "MCP_REQUEST_TIMEOUT"
	ErrRequestCancelled ErrCode = "MCP_REQUEST_CANCELLED"
	ErrNoDiagnostics    ErrCode = "TLAPM_EXIT_NO_DIAGNOSTICS"
	ErrOutputIncomplete ErrCode = "TLAPM_OUTPUT_INCOMPLETE"
	ErrTLAPMFailure     ErrCode = "TLAPM_FAILURE"
	// ErrNoObligations is returned when the selected target produces no proof obligations.
	ErrNoObligations ErrCode = "NO_OBLIGATIONS"
	// ErrExitCode is retained for compatibility with older callers.
	ErrExitCode ErrCode = "EXIT_CODE"
	// ErrStep is returned when a step selector is invalid.
	ErrStep ErrCode = "STEP_INVALID"
	// ErrIO is returned for file I/O errors.
	ErrIO ErrCode = "IO_ERROR"
)

// TLAPMError is a structured tool error with context.
type TLAPMError struct {
	Tool    string  `json:"tool,omitempty"`    // which MCP tool triggered this
	Code    ErrCode `json:"code"`              // error category
	Message string  `json:"message"`           // human-readable message
	Details string  `json:"details,omitempty"` // extra context (e.g., file path, solver name)
}

// Error implements the error interface.
func (e *TLAPMError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s] %s: %s (%s)", e.Tool, e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s] %s: %s", e.Tool, e.Code, e.Message)
}

// ToMap converts the error to a map for JSON serialization.
func (e *TLAPMError) ToMap() map[string]any {
	m := map[string]any{
		"tool":    e.Tool,
		"code":    string(e.Code),
		"message": e.Message,
	}
	if e.Details != "" {
		m["details"] = e.Details
	}
	return m
}

// Is returns true if the error is a TLAPMError with the given code.
func Is(code ErrCode) func(error) bool {
	return func(err error) bool {
		var tErr *TLAPMError
		return err != nil && errors.As(err, &tErr) && tErr.Code == code
	}
}

// As checks if an error is or wraps a TLAPMError.
func As(err error, target **TLAPMError) bool {
	if err == nil || target == nil {
		return false
	}
	return errors.As(err, target)
}

// WrapToolError wraps a raw error with tool context.
func WrapToolError(tool string, code ErrCode, msg string, details string) *TLAPMError {
	return &TLAPMError{
		Tool:    tool,
		Code:    code,
		Message: msg,
		Details: details,
	}
}

// WrapError wraps an error with tool context and preserves the original.
func WrapError(tool string, code ErrCode, msg string, original error) error {
	base := WrapToolError(tool, code, msg, "")
	if original != nil {
		base.Details = original.Error()
	}
	return base
}

// DetectErrorClass examines an error and returns an appropriate ErrCode.
func DetectErrorClass(err error) ErrCode {
	if err == nil {
		return ""
	}
	var tErr *TLAPMError
	if errors.As(err, &tErr) {
		return tErr.Code
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "executable file not found"):
		return ErrTLAPMNotFound
	case strings.Contains(msg, "no such file or directory") ||
		strings.Contains(msg, "not found"):
		return ErrModuleNotFound
	case strings.Contains(msg, "solver"):
		return ErrSolverUnavailable
	default:
		return ErrExitCode
	}
}

// ParseExitCodeError creates a structured error from an exec.ExitError or generic exec error.
func ParseExitCodeError(tool string, output []byte, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return WrapToolError(tool, ErrTLAPMNotFound, "tlapm binary not found", err.Error())
	}

	outputStr := string(output)
	code := classifyTLAPMOutput(outputStr, true)
	message := errorMessage(code)
	details := diagnosticSummary(outputStr)
	if details == "" {
		details = err.Error()
	}
	return WrapToolError(tool, code, message, details)
}

// RequestContextError classifies an MCP request cancellation or deadline.
func RequestContextError(tool string, err error) *TLAPMError {
	if err == nil {
		return nil
	}
	code := ErrRequestCancelled
	message := "MCP request was cancelled"
	if errors.Is(err, context.DeadlineExceeded) {
		code = ErrRequestTimeout
		message = "MCP request timed out"
	}
	return WrapToolError(tool, code, message, err.Error())
}

func classifyTLAPMOutput(output string, processExited bool) ErrCode {
	lower := strings.ToLower(output)
	switch {
	case strings.Contains(lower, "fingerprint") &&
		(strings.Contains(lower, "corrupt") || strings.Contains(lower, "invalid")):
		return ErrFingerprintCorrupted
	case isTLAPMParseFailure(lower):
		return ErrTLAPMParse
	case strings.Contains(lower, "solver") && strings.Contains(lower, "not available"):
		return ErrSolverUnavailable
	case hasBackendTimeoutDiagnostic(output):
		return ErrBackendTimeout
	}

	obligations, _ := parseToolboxObligations(output)
	for _, obligation := range obligations {
		if obligation.Status == "timeout" {
			return ErrBackendTimeout
		}
	}
	for _, obligation := range obligations {
		if obligation.Status == "failed" {
			return ErrProofFailed
		}
	}
	if proofFailureSummaryRe.MatchString(output) {
		return ErrProofFailed
	}
	if strings.Contains(lower, "backend errors processing") || strings.Contains(lower, "backend error") {
		return ErrBackendFailure
	}
	if hasZeroObligationCompletion(output) {
		return ErrNoObligations
	}
	if processExited {
		if !hasTLAPMDiagnostics(output) {
			return ErrNoDiagnostics
		}
		return ErrTLAPMFailure
	}
	if hasTLAPMDiagnostics(output) {
		return ErrTLAPMFailure
	}
	return ErrOutputIncomplete
}

var proofFailureSummaryRe = regexp.MustCompile(`(?i)\b[0-9]+\s*/\s*[0-9]+\s+obligations?\s+failed\b`)

func isTLAPMParseFailure(lowerOutput string) bool {
	for _, line := range strings.Split(lowerOutput, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "error:") && !strings.HasPrefix(line, "[error]:") {
			continue
		}
		if strings.Contains(line, "could not parse") ||
			strings.Contains(line, "syntax error") ||
			strings.Contains(line, "could not elaborate") ||
			strings.Contains(line, "elaboration failed") ||
			strings.Contains(line, "module not found") ||
			strings.Contains(line, "cannot find module") ||
			(strings.Contains(line, "operator ") && strings.Contains(line, " not found")) {
			return true
		}
	}
	return false
}

func hasBackendTimeoutDiagnostic(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		line = strings.ToLower(strings.TrimSpace(line))
		if !strings.HasPrefix(line, "[error]:") && !strings.HasPrefix(line, "error:") && !strings.HasPrefix(line, "zenon error:") {
			continue
		}
		if strings.Contains(line, "timed out") || strings.Contains(line, "timeout") || strings.Contains(line, "time limit exceeded") {
			return true
		}
	}
	return false
}

func hasTLAPMDiagnostics(output string) bool {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[ERROR]:") ||
			strings.HasPrefix(line, "Error:") ||
			strings.HasPrefix(line, "Zenon error:") {
			return true
		}
	}
	return isTLAPMParseFailure(strings.ToLower(output))
}

func errorMessage(code ErrCode) string {
	switch code {
	case ErrInvalidModule:
		return "module path is invalid"
	case ErrInvalidRange:
		return "source range is invalid"
	case ErrInvalidRequest:
		return "MCP tool arguments are invalid"
	case ErrTLAPMNotFound:
		return "tlapm binary is unavailable"
	case ErrTLAPMParse:
		return "TLAPM could not parse or elaborate the module"
	case ErrProofFailed:
		return "one or more proof obligations failed"
	case ErrBackendTimeout:
		return "a proof backend timed out"
	case ErrFingerprintCorrupted:
		return "fingerprint cache is corrupted"
	case ErrRequestTimeout:
		return "MCP request timed out"
	case ErrRequestCancelled:
		return "MCP request was cancelled"
	case ErrNoDiagnostics:
		return "TLAPM exited without diagnostics"
	case ErrSolverUnavailable:
		return "solver is unavailable"
	case ErrBackendFailure:
		return "proof backend failed"
	case ErrNoObligations:
		return "no proof obligations were generated for the target"
	case ErrOutputIncomplete:
		return "TLAPM output did not confirm a complete result"
	case ErrTLAPMFailure:
		return "TLAPM exited with an error"
	default:
		return "TLAPM output did not confirm a complete proof result"
	}
}

func diagnosticSummary(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		for _, prefix := range []string{"[ERROR]:", "Error:", "Zenon error:"} {
			if strings.HasPrefix(line, prefix) {
				return strings.TrimSpace(strings.TrimPrefix(line, prefix))
			}
		}
	}
	return ""
}

// LogError writes a JSON diagnostic to stderr; stderr is not MCP protocol output.
func LogError(tool string, err error) {
	if err == nil {
		return
	}
	var diagnostic map[string]any
	if tErr, ok := err.(*TLAPMError); ok {
		diagnostic = tErr.ToMap()
	} else {
		diagnostic = map[string]any{
			"tool":    tool,
			"code":    string(DetectErrorClass(err)),
			"message": err.Error(),
		}
	}
	_ = json.NewEncoder(os.Stderr).Encode(diagnostic)
}

// parseObligationErrors extracts error details from tlapm output lines.
// It parses lines matching [ERROR]: or solver-specific error patterns.
func parseObligationErrors(output string) []string {
	var errors []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[ERROR]:") {
			errors = append(errors, strings.TrimPrefix(line, "[ERROR]:"))
		}
	}
	return errors
}

// parseSolverError extracts the solver name from a solver error message.
func parseSolverError(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[ERROR]:") {
			msg := strings.TrimPrefix(line, "[ERROR]:")
			// Extract solver name from patterns like "Z3 error:" or "solver: z3"
			if strings.Contains(msg, "error") {
				parts := strings.Split(msg, ":")
				if len(parts) >= 2 {
					return strings.TrimSpace(parts[0])
				}
			}
		}
	}
	return "unknown"
}

// isNotFoundError checks if the underlying error is a "file not found" error.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "stat : no such file or directory" ||
		strings.Contains(err.Error(), "no such file or directory") ||
		strings.Contains(err.Error(), "file does not exist")
}

// ensureTLAPMBinary checks that the tlapm binary is available on PATH.
func ensureTLAPMBinary(tool string) error {
	if _, err := exec.LookPath("tlapm"); err != nil {
		return WrapToolError(tool, ErrTLAPMNotFound, "tlapm binary not found", err.Error())
	}
	return nil
}

// EnsureModuleExists checks a module path against the supplied filesystem or the OS for absolute paths.
func EnsureModuleExists(fsys fs.FS, modulePath string) error {
	return ensureModuleExists("prove", fsys, modulePath)
}

func ensureModuleExists(tool string, fsys fs.FS, modulePath string) error {
	var err error
	if filepath.IsAbs(modulePath) {
		_, err = os.Stat(modulePath)
	} else {
		_, err = fs.Stat(fsys, modulePath)
	}
	if err != nil {
		if osIsNotExist(err) {
			return WrapToolError(tool, ErrModuleNotFound, "module file not found", modulePath)
		}
		return WrapError(tool, ErrIO, "cannot read module file", err)
	}
	return nil
}

// osIsNotExist checks if the underlying path error represents a missing file.
func osIsNotExist(err error) bool {
	return err != nil && os.IsNotExist(err)
}
