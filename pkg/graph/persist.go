package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	bolt "go.etcd.io/bbolt"
)

// StateStore defines the interface for persisting engine state.
// This enables crash recovery — the engine can resume from the last
// checkpoint after an unexpected shutdown.
type StateStore interface {
	// Save persists the current state snapshot under the given run ID.
	Save(runID string, state *State) error

	// Load retrieves the state snapshot for the given run ID.
	// Returns nil, nil if no checkpoint exists.
	Load(runID string) (*State, error)

	// Close releases any resources held by the store.
	Close() error
}

// stateSnapshot is the serializable representation of a State.
type stateSnapshot struct {
	Data     map[string]any `json:"data"`
	Messages []MessageEntry `json:"messages"`
}

// ---------------------------------------------------------------------------
// BoltStateStore — bbolt-backed persistence
// ---------------------------------------------------------------------------

var bucketName = []byte("opcodex_state")

// BoltStateStore implements StateStore using bbolt (pure Go, no CGO).
type BoltStateStore struct {
	db *bolt.DB
}

// NewBoltStateStore opens (or creates) a bbolt database at the given path.
// Default path: ~/.opcodex/state.db
func NewBoltStateStore(path string) (*BoltStateStore, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("state store: get home dir: %w", err)
		}
		dir := filepath.Join(home, ".opcodex")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("state store: create dir: %w", err)
		}
		path = filepath.Join(dir, "state.db")
	}

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("state store: create parent dir: %w", err)
	}

	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("state store: open db: %w", err)
	}

	// Ensure bucket exists.
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucketName)
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("state store: create bucket: %w", err)
	}

	return &BoltStateStore{db: db}, nil
}

// Save serializes the state's KV data and message history to bbolt.
func (s *BoltStateStore) Save(runID string, state *State) error {
	state.mu.RLock()
	snap := stateSnapshot{
		Data:     copyMap(state.data),
		Messages: make([]MessageEntry, len(state.messages)),
	}
	copy(snap.Messages, state.messages)
	state.mu.RUnlock()

	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("state store: marshal: %w", err)
	}

	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Put([]byte(runID), data)
	})
}

// Load deserializes a state snapshot from bbolt.
// Returns nil, nil if no checkpoint exists for the given run ID.
func (s *BoltStateStore) Load(runID string) (*State, error) {
	var data []byte

	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		v := b.Get([]byte(runID))
		if v != nil {
			data = make([]byte, len(v))
			copy(data, v)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("state store: read: %w", err)
	}
	if data == nil {
		return nil, nil // No checkpoint found.
	}

	var snap stateSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("state store: unmarshal: %w", err)
	}

	state := &State{
		data:     snap.Data,
		messages: snap.Messages,
	}
	if state.data == nil {
		state.data = make(map[string]any)
	}

	return state, nil
}

// Close closes the underlying bbolt database.
func (s *BoltStateStore) Close() error {
	return s.db.Close()
}

// copyMap creates a shallow copy of a map.
func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
