package llm

import (
	"context"
	"fmt"
)

// MockAdapter simulates LLM responses for testing the graph walk without
// consuming real API tokens.
//
// It uses a step counter to return different responses on each call:
//
//	Turn 1 (Think)     → issues a ToolCall to "shell_exec" with a sample command
//	Turn 2 (Summarize) → returns a text summary and no tool calls
type MockAdapter struct {
	step int
}

// NewMockAdapter creates a fresh mock adapter starting at step 0.
func NewMockAdapter() *MockAdapter {
	return &MockAdapter{}
}

// Chat simulates a multi-turn LLM conversation.
// The tools parameter is accepted for interface compatibility but ignored.
func (m *MockAdapter) Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error) {
	m.step++

	// Log the conversation depth for debugging.
	fmt.Printf("  🧠 [MockLLM] Turn %d — received %d messages in context\n", m.step, len(messages))

	switch m.step {
	case 1:
		// Turn 1: LLM "decides" to call the shell_exec skill.
		fmt.Println("  🧠 [MockLLM] Thinking... deciding to execute a system command")
		return Response{
			Content: "",
			ToolCalls: []ToolCall{
				{
					Name: "shell_exec",
					Parameters: map[string]any{
						"command": "uname -a && uptime",
					},
				},
			},
		}, nil

	case 2:
		// Turn 2: LLM "observes" the tool result and produces a summary.
		fmt.Println("  🧠 [MockLLM] Observing results and writing report...")
		return Response{
			Content: "✅ 系统状态报告：主机运行正常，已获取系统信息和负载数据。任务完成。",
		}, nil

	default:
		return Response{}, fmt.Errorf("mock adapter: unexpected step %d", m.step)
	}
}
