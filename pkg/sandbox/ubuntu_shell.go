package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// DefaultTimeout is applied when the caller does not set a context deadline.
const DefaultTimeout = 30 * time.Second

// MaxOutputBytes is the maximum number of bytes captured from stdout/stderr.
// Anything beyond this is truncated to protect against memory exhaustion.
const MaxOutputBytes = 64 * 1024 // 64 KB

// ErrBlockedCommand is returned when a command matches the blacklist.
var ErrBlockedCommand = fmt.Errorf("sandbox: command blocked by security policy")

// dangerousPatterns contains regex patterns for commands that should never
// be executed, even by accident (LLM hallucination, prompt injection, etc.).
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\brm\s+(-\w*\s+)*-?\w*r\w*f\w*\s+/`),   // rm -rf /
	regexp.MustCompile(`(?i)\bmkfs\b`),                               // format filesystem
	regexp.MustCompile(`(?i)\bdd\s+.*of\s*=\s*/dev/`),                // dd to device
	regexp.MustCompile(`:\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;\s*:`),   // fork bomb
	regexp.MustCompile(`(?i)\bshutdown\b`),                           // shutdown
	regexp.MustCompile(`(?i)\breboot\b`),                             // reboot
	regexp.MustCompile(`(?i)\bhalt\b`),                               // halt
	regexp.MustCompile(`(?i)\binit\s+0\b`),                           // init 0
	regexp.MustCompile(`(?i)\bsystemctl\s+(poweroff|reboot|halt)\b`), // systemctl power ops
	regexp.MustCompile(`(?i)>\s*/dev/sd[a-z]`),                       // write to raw disk
	regexp.MustCompile(`(?i)\bchmod\s+(-\w+\s+)*777\s+/`),           // chmod 777 / 
}

// UbuntuShellExecutor runs commands directly on the host via /bin/bash.
//
// It includes security hardening:
//   - Command blacklist (regex-based, checked before exec)
//   - Output size cap (prevents memory exhaustion)
//   - Configurable timeout with guaranteed deadline
type UbuntuShellExecutor struct {
	// Shell is the path to the interpreter. Defaults to "/bin/bash".
	Shell string
	// Timeout overrides the default deadline. Zero means use DefaultTimeout.
	Timeout time.Duration
}

// NewUbuntuShellExecutor creates an executor that delegates to /bin/bash.
func NewUbuntuShellExecutor() *UbuntuShellExecutor {
	return &UbuntuShellExecutor{Shell: "/bin/bash"}
}

// Execute runs the command through the configured shell after security checks.
func (e *UbuntuShellExecutor) Execute(ctx context.Context, command string) (ExecResult, error) {
	// ── Security gate: blacklist check ──
	if blocked, pattern := isBlocked(command); blocked {
		return ExecResult{
			ExitCode: -1,
			Stderr:   fmt.Sprintf("🛑 BLOCKED: command matched security rule: %s", pattern),
		}, ErrBlockedCommand
	}

	// ── Apply timeout ──
	timeout := e.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	shell := e.Shell
	if shell == "" {
		shell = "/bin/bash"
	}

	cmd := exec.CommandContext(ctx, shell, "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := ExecResult{
		Stdout: truncate(stdout.String(), MaxOutputBytes),
		Stderr: truncate(stderr.String(), MaxOutputBytes),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		// Check if the context deadline was exceeded (timeout).
		if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Stderr += "\n⏱️ TIMEOUT: command exceeded maximum execution time"
			return result, fmt.Errorf("sandbox: command timed out after %s", timeout)
		}
		return result, fmt.Errorf("shell exec failed: %w", err)
	}

	return result, nil
}

// isBlocked checks the command against all dangerous patterns.
func isBlocked(command string) (bool, string) {
	for _, pat := range dangerousPatterns {
		if pat.MatchString(command) {
			return true, pat.String()
		}
	}
	return false, ""
}

// truncate limits a string to maxBytes, appending a truncation notice.
func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + strings.Repeat(".", 3) + fmt.Sprintf("\n[truncated: %d bytes total]", len(s))
}
