package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// SSHExecutor runs commands on a remote host via SSH.
//
// Each command is executed as: ssh -o BatchMode=yes -o ConnectTimeout=5 <target> "<command>"
// BatchMode prevents interactive password prompts (requires key-based auth).
type SSHExecutor struct {
	// Target is the SSH destination (e.g. "root@192.168.1.100" or "user@host:2222").
	Target string
	// Timeout is the maximum execution time (default: 30s).
	Timeout time.Duration
	// Port overrides the SSH port (default: 22).
	Port string
}

// NewSSHExecutor creates an executor for a remote host.
// It verifies that `ssh` is available in PATH.
func NewSSHExecutor(target string) (*SSHExecutor, error) {
	if _, err := exec.LookPath("ssh"); err != nil {
		return nil, fmt.Errorf("ssh executor: 'ssh' not found in PATH")
	}
	return &SSHExecutor{
		Target:  target,
		Timeout: DefaultTimeout,
	}, nil
}

// Execute runs a command on the remote host via SSH.
func (s *SSHExecutor) Execute(ctx context.Context, command string) (ExecResult, error) {
	// Security gate.
	if blocked, pattern := isBlocked(command); blocked {
		return ExecResult{
			ExitCode: -1,
			Stderr:   fmt.Sprintf("🛑 BLOCKED: command matched security rule: %s", pattern),
		}, ErrBlockedCommand
	}

	// Timeout.
	timeout := s.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Build SSH args.
	args := []string{
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=5",
		"-o", "StrictHostKeyChecking=no",
	}
	if s.Port != "" {
		args = append(args, "-p", s.Port)
	}
	args = append(args, s.Target, command)

	cmd := exec.CommandContext(ctx, "ssh", args...)

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
			result.Stderr += "\n⏱️ TIMEOUT: SSH command exceeded maximum execution time"
			return result, fmt.Errorf("ssh: command timed out after %s", timeout)
		}
		return result, fmt.Errorf("ssh exec failed: %w", err)
	}

	return result, nil
}
