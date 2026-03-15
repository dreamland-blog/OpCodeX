package skill

import (
	"context"
	"fmt"

	"github.com/dreamland-blog/OpCodeX/pkg/sandbox"
)

// ShellInput describes the parameters for a shell command execution.
type ShellInput struct {
	Command string `json:"command" schema:"required" description:"Shell command to execute on the host"`
}

// ExecuteShellSkill wraps a sandbox.Executor as a Skill, making it
// available to the LLM through the standard tool-call interface.
type ExecuteShellSkill struct {
	executor sandbox.Executor
}

// NewExecuteShellSkill creates a shell execution skill powered by the
// given sandbox executor.
func NewExecuteShellSkill(exec sandbox.Executor) *ExecuteShellSkill {
	return &ExecuteShellSkill{executor: exec}
}

// Name returns the skill identifier used in tool-call matching.
func (s *ExecuteShellSkill) Name() string { return "shell_exec" }

// Description returns a human/LLM-readable summary.
func (s *ExecuteShellSkill) Description() string {
	return "Execute a shell command on the host system and return stdout/stderr."
}

// InputSchema returns the JSON Schema for this skill's input, generated
// automatically from the ShellInput struct.
func (s *ExecuteShellSkill) InputSchema() map[string]any {
	return GenerateSchema(ShellInput{})
}

// Execute runs the command through the sandbox and returns the result.
func (s *ExecuteShellSkill) Execute(ctx context.Context, input SkillInput) (SkillOutput, error) {
	cmd, _ := input.Parameters["command"].(string)
	if cmd == "" {
		return SkillOutput{}, fmt.Errorf("shell_exec: missing required parameter 'command'")
	}

	result, err := s.executor.Execute(ctx, cmd)
	if err != nil {
		return SkillOutput{}, fmt.Errorf("shell_exec: %w", err)
	}

	output := SkillOutput{
		Result: map[string]any{
			"exit_code": result.ExitCode,
			"stdout":    result.Stdout,
			"stderr":    result.Stderr,
		},
		RawText: result.Stdout,
	}

	if result.Stderr != "" {
		output.RawText += "\n[stderr] " + result.Stderr
	}

	return output, nil
}
