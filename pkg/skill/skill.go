// Package skill defines the core Skill abstraction for OpCodeX.
//
// A Skill is the smallest unit of executable capability in the engine —
// it wraps a single, well-defined operation (run a shell command, call an API,
// parse a file, etc.) and exposes a uniform Execute interface so the graph
// engine can orchestrate it without knowing the implementation details.
package skill

import "context"

// SkillInput carries incoming parameters for a Skill invocation.
type SkillInput struct {
	// Parameters is a free-form map of key-value pairs that the skill expects.
	Parameters map[string]any
}

// SkillOutput carries the result of a Skill invocation.
type SkillOutput struct {
	// Result holds the structured output from the skill execution.
	Result map[string]any
	// RawText is an optional human-readable summary of what happened.
	RawText string
}

// Skill is the fundamental capability interface.
// Every concrete skill (ShellExec, APKAnalyzer, etc.) must implement this.
type Skill interface {
	// Name returns a unique, machine-friendly identifier (e.g. "shell_exec").
	Name() string

	// Description returns a short natural-language summary that LLMs can use
	// when deciding which skill to invoke.
	Description() string

	// InputSchema returns the JSON Schema (as a map) describing the expected
	// parameters. The graph engine feeds this to the LLM for tool-call generation.
	InputSchema() map[string]any

	// Execute runs the skill with the given input and returns its output.
	Execute(ctx context.Context, input SkillInput) (SkillOutput, error)
}

// Registry is a simple in-memory skill catalog keyed by skill name.
type Registry struct {
	skills map[string]Skill
}

// NewRegistry creates an empty skill registry.
func NewRegistry() *Registry {
	return &Registry{skills: make(map[string]Skill)}
}

// Register adds a skill to the registry. Panics on duplicate names.
func (r *Registry) Register(s Skill) {
	if _, exists := r.skills[s.Name()]; exists {
		panic("skill already registered: " + s.Name())
	}
	r.skills[s.Name()] = s
}

// Get retrieves a skill by name. Returns nil if not found.
func (r *Registry) Get(name string) Skill {
	return r.skills[name]
}

// All returns every registered skill (useful for building tool manifests).
func (r *Registry) All() []Skill {
	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	return out
}
