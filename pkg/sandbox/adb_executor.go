package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ADBExecutor runs commands on an Android device via `adb shell`.
//
// Each command is executed as: adb -s <serial> shell "<command>"
// This enables direct interaction with connected Android devices
// or emulators for automation and reverse engineering tasks.
type ADBExecutor struct {
	// Serial is the ADB device serial (e.g. "emulator-5554" or "192.168.1.50:5555").
	Serial string
	// Timeout is the maximum execution time (default: 30s).
	Timeout time.Duration
}

// NewADBExecutor creates an executor for a specific Android device.
// It verifies that `adb` is available in PATH.
func NewADBExecutor(serial string) (*ADBExecutor, error) {
	if _, err := exec.LookPath("adb"); err != nil {
		return nil, fmt.Errorf("adb executor: 'adb' not found in PATH")
	}
	return &ADBExecutor{
		Serial:  serial,
		Timeout: DefaultTimeout,
	}, nil
}

// Execute runs a command on the target device via adb shell.
func (a *ADBExecutor) Execute(ctx context.Context, command string) (ExecResult, error) {
	// Security gate.
	if blocked, pattern := isBlocked(command); blocked {
		return ExecResult{
			ExitCode: -1,
			Stderr:   fmt.Sprintf("🛑 BLOCKED: command matched security rule: %s", pattern),
		}, ErrBlockedCommand
	}

	// Timeout.
	timeout := a.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Build: adb -s <serial> shell "<command>"
	args := []string{"-s", a.Serial, "shell", command}
	cmd := exec.CommandContext(ctx, "adb", args...)

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
		if ctx.Err() == context.DeadlineExceeded {
			result.ExitCode = -1
			result.Stderr += "\n⏱️ TIMEOUT: adb command exceeded maximum execution time"
			return result, fmt.Errorf("adb: command timed out after %s", timeout)
		}
		return result, fmt.Errorf("adb exec failed: %w", err)
	}

	return result, nil
}
