// Package llm provides a unified adapter interface for calling large language
// models. This is an internal package — external consumers should not import
// it directly; instead they interact with the engine which owns the adapter.
package llm

import "context"

// Role identifies who authored a message in a conversation.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single turn in a chat conversation.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

// ToolCall represents the model requesting execution of a tool/skill.
type ToolCall struct {
	Name       string         `json:"name"`
	Parameters map[string]any `json:"parameters"`
}

// Response is the model's reply to a Chat call.
type Response struct {
	// Content is the text portion of the reply (may be empty when ToolCalls
	// are present).
	Content string `json:"content,omitempty"`

	// ToolCalls is non-nil when the model wants the engine to invoke skills.
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`
}

// ToolDef describes a tool/skill that the LLM can invoke.
// Passed to Chat so the model knows what functions are available.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// Adapter is the interface every LLM backend must implement.
type Adapter interface {
	// Chat sends a conversation and returns the model's response.
	// tools describes the available functions; pass nil if no tools.
	Chat(ctx context.Context, messages []Message, tools []ToolDef) (Response, error)
}
