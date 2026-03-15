package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"time"
)

// DockerExecutor runs commands inside an ephemeral Docker container.
//
// Each Execute call spins up a fresh container via `docker run --rm`,
// providing true process and filesystem isolation from the host.
type DockerExecutor struct {
	// Image is the Docker image to use (default: "ubuntu:22.04").
	Image string
	// Timeout is the maximum execution time (default: 30s).
	Timeout time.Duration
}

// NewDockerExecutor creates a DockerExecutor with sensible defaults.
// It verifies that the `docker` CLI is available in PATH.
func NewDockerExecutor() (*DockerExecutor, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker executor: 'docker' not found in PATH — is Docker installed?")
	}
	return &DockerExecutor{
		Image:   "ubuntu:22.04",
		Timeout: DefaultTimeout,
	}, nil
}

// Execute runs the command inside a disposable Docker container.
func (d *DockerExecutor) Execute(ctx context.Context, command string) (ExecResult, error) {
	// Security gate: same blacklist as UbuntuShellExecutor.
	if blocked, pattern := isBlocked(command); blocked {
		return ExecResult{
			ExitCode: -1,
			Stderr:   fmt.Sprintf("🛑 BLOCKED: command matched security rule: %s", pattern),
		}, ErrBlockedCommand
	}

	// Apply timeout.
	timeout := d.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	image := d.Image
	if image == "" {
		image = "ubuntu:22.04"
	}

	// Build: docker run --rm --network=none <image> /bin/bash -c "<command>"
	//   --rm          : auto-remove container after exit
	//   --network=none: no network access inside the container
	args := []string{
		"run", "--rm",
		"--network=none",
		"--memory=128m",    // limit memory to 128MB
		"--cpus=0.5",       // limit to half a CPU core
		image,
		"/bin/bash", "-c", command,
	}

	cmd := exec.CommandContext(ctx, "docker", args...)

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
			result.Stderr += "\n⏱️ TIMEOUT: container exceeded maximum execution time"
			// Attempt to kill lingering containers (best effort).
			return result, fmt.Errorf("docker: command timed out after %s", timeout)
		}
		return result, fmt.Errorf("docker exec failed: %w", err)
	}

	return result, nil
}

// String returns a human-readable description of this executor.
func (d *DockerExecutor) String() string {
	return fmt.Sprintf("DockerExecutor(image=%s, timeout=%s)", d.Image, d.Timeout)
}
