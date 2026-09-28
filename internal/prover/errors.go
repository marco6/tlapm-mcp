package prover

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrCode identifies the category of a TLAPM error.
type ErrCode string

const (
	// ErrTLAPMNotFound is returned when the tlapm binary is not on PATH.
	ErrTLAPMNotFound ErrCode = "TLAPM_NOT_FOUND"
	// ErrModuleNotFound is returned when the specified module file does not exist.
	ErrModuleNotFound ErrCode = "MODULE_NOT_FOUND"
	// ErrSolverUnavailable is returned when a solver binary is missing or incompatible.
	ErrSolverUnavailable ErrCode = "SOLVER_UNAVAILABLE"
	// ErrFingerprintCorrupted is returned when the fingerprint cache file is corrupt.
	ErrFingerprintCorrupted ErrCode = "FINGERPRINT_CORRUPTED"
	// ErrParse is returned when tlapm output cannot be parsed.
	ErrParse ErrCode = "PARSE_ERROR"
	// ErrExitCode is returned when tlapm exits with a non-zero code.
	ErrExitCode ErrCode = "EXIT_CODE"
	// ErrStep is returned when a step selector is invalid.
	ErrStep ErrCode = "STEP_INVALID"
	// ErrIO is returned for file I/O errors.
	ErrIO ErrCode = "IO_ERROR"
)

// TLAPMError is a structured error with context.
type TLAPMError struct {
	Tool    string     `json:"tool,omitempty"`    // which MCP tool triggered this
	Code    ErrCode    `json:"code"`              // error category
	Message string     `json:"message"`           // human-readable message
	Details string     `json:"details,omitempty"` // extra context (e.g., file path, solver name)
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
		return err != nil && (As(err, &tErr) && tErr.Code == code)
	}
}

// As checks if an error is or wraps a TLAPMError.
func As(err error, target **TLAPMError) bool {
	if err == nil || target == nil {
		return false
	}
	_, ok := err.(*TLAPMError)
	return ok
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
	msg := err.Error()
	switch {
	case strings.Contains(msg, "executable file not found"):
		return ErrTLAPMNotFound
	case strings.Contains(msg, "no such file or directory"):
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

	// Check for specific error messages in the tlapm output
	outputStr := string(output)
	if strings.Contains(outputStr, "Fingerprint") &&
		(strings.Contains(outputStr, "corrupt") || strings.Contains(outputStr, "invalid")) {
		return WrapToolError(tool, ErrFingerprintCorrupted,
			"corrupted fingerprint file", outputStr)
	}

	if strings.Contains(outputStr, "solver") &&
		strings.Contains(outputStr, "not available") {
		return WrapToolError(tool, ErrSolverUnavailable,
			"solver not available", outputStr)
	}

	return WrapError(tool, ErrExitCode,
		fmt.Sprintf("tlapm exited with code %v", err),
		err)
}

// LogError writes a structured error to the error stream.
// In MCP stdio mode, errors are logged to stderr as JSON for the client to read.
func LogError(tool string, err error) {
	if err == nil {
		return
	}
	var msg string
	var code string
	if tErr, ok := err.(*TLAPMError); ok {
		msg = tErr.Message
		code = string(tErr.Code)
	} else {
		msg = err.Error()
		code = string(DetectErrorClass(err))
	}

	// Log to stderr in a format that MCP clients can consume
	fmt.Fprintf(os.Stderr, "{\"jsonrpc\":\"2.0\",\"method\":\"notifications/error\",\"params\":{\"tool\":\"%s\",\"code\":\"%s\",\"message\":\"%s\"}}\n",
		tool, code, msg)
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
func ensureTLAPMBinary() error {
	_, err := exec.LookPath("tlapm")
	if err != nil {
		return WrapToolError("prove", ErrTLAPMNotFound,
			"tlapm binary not found", err.Error())
	}
	return nil
}

// EnsureModuleExists checks that the module file exists and is readable.
func EnsureModuleExists(modulePath string) error {
	if _, err := os.Stat(modulePath); err != nil {
		if os.IsNotExist(err) {
			return WrapToolError("prove", ErrModuleNotFound,
				"module file not found", modulePath)
		}
		return WrapError("prove", ErrIO,
			"cannot read module file", err)
	}
	return nil
}
