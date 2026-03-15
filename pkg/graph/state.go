package graph

import "sync"

// ---------------------------------------------------------------------------
// MessageEntry — a single turn in the conversation history
// ---------------------------------------------------------------------------

// FuncCallEntry represents an LLM-issued function/tool call.
type FuncCallEntry struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args,omitempty"`
}

// FuncRespEntry represents the result of a function execution sent back
// to the LLM.
type FuncRespEntry struct {
	Name   string         `json:"name"`
	Result map[string]any `json:"result,omitempty"`
}

// MessageEntry is one turn in the multi-turn conversation log.
//
// Role uses Gemini-native names:
//   - "user"  — human or system prompt
//   - "model" — LLM response (text and/or functionCall)
//   - "user"  — also used for functionResponse (with FuncResp set)
type MessageEntry struct {
	Role     string         // "user" or "model"
	Text     string         // text content (may be empty)
	FuncCall *FuncCallEntry // non-nil when model issues a function call
	FuncResp *FuncRespEntry // non-nil when providing a function result
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

// State is a thread-safe key-value store that carries shared context
// between nodes as the engine walks the graph.
//
// It also maintains an ordered conversation history (MessageHistory) so
// the LLM adapter can reconstruct the full multi-turn context.
type State struct {
	mu       sync.RWMutex
	data     map[string]any
	messages []MessageEntry
}

// NewState creates an empty State.
func NewState() *State {
	return &State{data: make(map[string]any)}
}

// ---------------------------------------------------------------------------
// Key-Value store
// ---------------------------------------------------------------------------

// Set stores a value under the given key (goroutine-safe).
func (s *State) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Get retrieves a value by key. The second return is false if the key
// does not exist.
func (s *State) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.data[key]
	return v, ok
}

// GetString returns the value as a string, or "" if missing / wrong type.
func (s *State) GetString(key string) string {
	v, ok := s.Get(key)
	if !ok {
		return ""
	}
	str, _ := v.(string)
	return str
}

// GetBool returns the value as a bool, or false if missing / wrong type.
func (s *State) GetBool(key string) bool {
	v, ok := s.Get(key)
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// MustGet is like Get but panics when the key is missing.
func (s *State) MustGet(key string) any {
	v, ok := s.Get(key)
	if !ok {
		panic("state: missing key: " + key)
	}
	return v
}

// Delete removes a key from the state.
func (s *State) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

// Keys returns a snapshot of all keys currently in the state.
func (s *State) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// Conversation history (MessageHistory)
// ---------------------------------------------------------------------------

// AddMessage appends a turn to the conversation history.
func (s *State) AddMessage(entry MessageEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = append(s.messages, entry)
}

// GetMessages returns a snapshot of the full conversation history.
func (s *State) GetMessages() []MessageEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]MessageEntry, len(s.messages))
	copy(out, s.messages)
	return out
}

// ClearMessages resets the conversation history.
func (s *State) ClearMessages() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages = nil
}
