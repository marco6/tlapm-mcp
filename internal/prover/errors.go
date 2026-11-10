package prover

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
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
	// ErrNoObligations is returned when the selected target produces no proof obligations.
	ErrNoObligations ErrCode = "NO_OBLIGATIONS"
	// ErrExitCode is returned when tlapm exits with a non-zero code.
	ErrExitCode ErrCode = "EXIT_CODE"
	// ErrStep is returned when a step selector is invalid.
	ErrStep ErrCode = "STEP_INVALID"
	// ErrIO is returned for file I/O errors.
	ErrIO ErrCode = "IO_ERROR"
)

// TLAPMError is a structured error with context.
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
	if strings.Contains(outputStr, "Fingerprint") &&
		(strings.Contains(outputStr, "corrupt") || strings.Contains(outputStr, "invalid")) {
		return WrapToolError(tool, ErrFingerprintCorrupted,
			"corrupted fingerprint file", outputStr)
	}
	if strings.Contains(outputStr, "solver") && strings.Contains(outputStr, "not available") {
		return WrapToolError(tool, ErrSolverUnavailable, "solver not available", outputStr)
	}
	return WrapError(tool, ErrExitCode, fmt.Sprintf("tlapm exited with code %v", err), err)
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
