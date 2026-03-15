// Package graph implements the OpCodeX execution graph engine.
//
// The engine maintains a directed graph of nodes connected by conditional
// edges, and drives execution by walking from node to node. Routing
// decisions live on edges (not inside nodes), making the flow explicit
// and easy to reason about.
//
// Error edges enable self-healing: when a node fails, the engine can
// route to a recovery node (e.g. the LLM brain) instead of crashing.
package graph

import (
	"context"
	"errors"
	"fmt"
)

// EndNodeID is a sentinel value. When the engine resolves an edge whose
// target is EndNodeID, execution terminates successfully.
const EndNodeID = "__end__"

// DefaultMaxRetries is the maximum number of error-edge retries per run.
const DefaultMaxRetries = 3

// EdgeCondition is a function evaluated by the engine after processing a
// node. The first edge whose condition returns true determines the target.
type EdgeCondition func(state *State) bool

// AlwaysRoute is a convenience EdgeCondition that always returns true.
// Use it for unconditional transitions (e.g. SystemOps → Brain).
func AlwaysRoute(_ *State) bool { return true }

// Edge represents a directed connection between two nodes with a guard
// condition. Edges are evaluated in insertion order; first match wins.
type Edge struct {
	From      string
	To        string
	Condition EdgeCondition
}

// ErrorEdge is a fallback transition taken when a node returns an error.
// Unlike normal edges, error edges have no condition — they trigger only
// on failure.
type ErrorEdge struct {
	From string
	To   string
}

// Engine is the core graph executor.
type Engine struct {
	nodes      map[string]Node
	edges      []Edge
	errorEdges []ErrorEdge

	// MaxRetries limits the number of error-edge retries per run.
	// Zero means use DefaultMaxRetries.
	MaxRetries int

	// StateStore, if set, is called after each node transition to
	// checkpoint the state for crash recovery.
	StateStore StateStore

	// RunID identifies the current run for state persistence.
	RunID string
}

// NewEngine creates an Engine with no nodes or edges.
func NewEngine() *Engine {
	return &Engine{
		nodes: make(map[string]Node),
	}
}

// AddNode registers a node in the graph. Panics on duplicate IDs.
func (e *Engine) AddNode(n Node) {
	id := n.ID()
	if _, exists := e.nodes[id]; exists {
		panic("duplicate node id: " + id)
	}
	e.nodes[id] = n
}

// AddEdge registers a conditional transition from one node to another.
// Edges are evaluated in the order they are added; the first whose
// condition returns true is taken.
func (e *Engine) AddEdge(from, to string, cond EdgeCondition) {
	e.edges = append(e.edges, Edge{From: from, To: to, Condition: cond})
}

// AddErrorEdge registers a fallback transition for when a node fails.
// When the node identified by 'from' returns an error, the engine
// stores the error in state and follows this edge instead of crashing.
func (e *Engine) AddErrorEdge(from, to string) {
	e.errorEdges = append(e.errorEdges, ErrorEdge{From: from, To: to})
}

// Run starts the engine at the given node and walks the graph.
//
// The execution loop:
//  1. Process the current node (which may mutate State).
//  2. If the node fails AND an error edge exists (and retries remain):
//     store error info in State and follow the error edge.
//  3. Otherwise evaluate outgoing edges in insertion order.
//  4. The first edge whose condition is true determines the next node.
//  5. If the next node is EndNodeID, execution completes successfully.
//  6. If no edge matches, an error is returned.
//  7. After each transition, checkpoint state if StateStore is set.
//
// The provided State is shared across all nodes for the lifetime of the run.
func (e *Engine) Run(ctx context.Context, state *State, startNodeID string) error {
	if len(e.nodes) == 0 {
		return errors.New("engine: no nodes registered")
	}

	maxRetries := e.MaxRetries
	if maxRetries <= 0 {
		maxRetries = DefaultMaxRetries
	}
	retryCount := 0

	currentID := startNodeID

	for {
		// Check for context cancellation.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Lookup and process the current node.
		node, ok := e.nodes[currentID]
		if !ok {
			return fmt.Errorf("engine: node %q not found", currentID)
		}

		err := node.Process(ctx, state)

		if err != nil {
			// ── Error path: try error edge recovery ──
			errorTarget := e.resolveErrorEdge(currentID)
			if errorTarget != "" && retryCount < maxRetries {
				retryCount++
				state.Set("last_error", err.Error())
				state.Set("error_node", currentID)
				state.Set("retry_count", retryCount)
				fmt.Printf("  ⚠️  [Engine] Node %q failed: %v (retry %d/%d → %s)\n",
					currentID, err, retryCount, maxRetries, errorTarget)
				currentID = errorTarget
				e.checkpoint(state, currentID)
				continue
			}
			// No error edge or retries exhausted — hard fail.
			return fmt.Errorf("engine: node %q failed: %w", currentID, err)
		}

		// ── Success path: clear any previous error state ──
		state.Delete("last_error")
		state.Delete("error_node")

		// Evaluate outgoing edges to find the next node.
		nextID, found := e.resolveEdge(currentID, state)
		if !found {
			return fmt.Errorf("engine: no matching edge from node %q", currentID)
		}

		// Terminal?
		if nextID == EndNodeID {
			e.checkpoint(state, EndNodeID)
			return nil
		}

		currentID = nextID
		e.checkpoint(state, currentID)
	}
}

// resolveEdge finds the first matching outgoing edge from the given node.
func (e *Engine) resolveEdge(fromID string, state *State) (string, bool) {
	for _, edge := range e.edges {
		if edge.From == fromID && edge.Condition(state) {
			return edge.To, true
		}
	}
	return "", false
}

// resolveErrorEdge finds the error-edge target for a given node, if any.
func (e *Engine) resolveErrorEdge(fromID string) string {
	for _, ee := range e.errorEdges {
		if ee.From == fromID {
			return ee.To
		}
	}
	return ""
}

// checkpoint persists the state if a StateStore is configured.
func (e *Engine) checkpoint(state *State, nextNodeID string) {
	if e.StateStore == nil {
		return
	}
	state.Set("__checkpoint_node__", nextNodeID)
	runID := e.RunID
	if runID == "" {
		runID = "default"
	}
	if err := e.StateStore.Save(runID, state); err != nil {
		fmt.Printf("  ⚠️  [Engine] Checkpoint failed: %v\n", err)
	}
}
