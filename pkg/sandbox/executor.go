// Package sandbox defines the Executor interface for running commands
// in isolated or semi-isolated environments.
package sandbox

import "context"

// ExecResult holds the output of a command execution.
type ExecResult struct {
	// ExitCode is the process exit code (0 = success).
	ExitCode int
	// Stdout is the standard output captured from the command.
	Stdout string
	// Stderr is the standard error captured from the command.
	Stderr string
}

// Executor is the common interface for all sandbox backends.
//
// Implementations may range from a raw os/exec wrapper (UbuntuShellExecutor)
// to Docker-based or WASM-sandboxed runners.
type Executor interface {
	// Execute runs the given command string and returns the result.
	Execute(ctx context.Context, command string) (ExecResult, error)
}
