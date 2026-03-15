package graph

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// ErrHumanRejected is returned when a human declines a confirmation prompt.
var ErrHumanRejected = fmt.Errorf("operation rejected by human operator")

// Node is the basic unit of work inside the execution graph.
//
// Nodes do work and mutate State; routing decisions live on Edges.
type Node interface {
	// ID returns the unique identifier for this node.
	ID() string

	// Process runs the node's logic, potentially mutating the shared State.
	// Routing is handled by the engine's edge conditions, not by the node.
	Process(ctx context.Context, state *State) error
}

// ---------------------------------------------------------------------------
// ActionNode — a node that runs a single function.
// ---------------------------------------------------------------------------

// ActionFunc is the signature for the function an ActionNode wraps.
type ActionFunc func(ctx context.Context, state *State) error

// ActionNode wraps an arbitrary function as a graph Node.
type ActionNode struct {
	id string
	fn ActionFunc
}

// NewActionNode creates a node that will call fn when processed.
func NewActionNode(id string, fn ActionFunc) *ActionNode {
	return &ActionNode{id: id, fn: fn}
}

// ID returns the node identifier.
func (n *ActionNode) ID() string { return n.id }

// Process delegates to the wrapped function.
func (n *ActionNode) Process(ctx context.Context, state *State) error {
	return n.fn(ctx, state)
}

// ---------------------------------------------------------------------------
// RouterNode — a node that inspects state and sets routing hints.
// ---------------------------------------------------------------------------

// RouteFunc inspects the state, performs any logic, and may mutate state
// to influence edge conditions.
type RouteFunc func(ctx context.Context, state *State) error

// RouterNode runs a routing function that typically inspects and updates
// State so downstream edge conditions can decide where to go next.
type RouterNode struct {
	id      string
	routeFn RouteFunc
}

// NewRouterNode creates a routing node.
func NewRouterNode(id string, fn RouteFunc) *RouterNode {
	return &RouterNode{id: id, routeFn: fn}
}

// ID returns the node identifier.
func (n *RouterNode) ID() string { return n.id }

// Process calls the route function.
func (n *RouterNode) Process(ctx context.Context, state *State) error {
	return n.routeFn(ctx, state)
}

// ---------------------------------------------------------------------------
// HumanConfirmNode — pauses for human approval.
// ---------------------------------------------------------------------------

// HumanConfirmNode halts the engine and prompts a human operator for
// confirmation before allowing execution to continue. This is essential
// for sensitive operations like file deletion, firmware flashing, or
// irreversible network requests.
//
// On approval (y): sets state["human_approved"] = true, returns nil.
// On rejection (n): sets state["human_approved"] = false, returns ErrHumanRejected.
// The engine's error-edge system can route rejections gracefully.
type HumanConfirmNode struct {
	id     string
	prompt string
}

// NewHumanConfirmNode creates a confirmation gate.
// prompt is displayed to the operator (e.g. "About to flash firmware. Continue?").
func NewHumanConfirmNode(id, prompt string) *HumanConfirmNode {
	return &HumanConfirmNode{id: id, prompt: prompt}
}

// ID returns the node identifier.
func (n *HumanConfirmNode) ID() string { return n.id }

// Process blocks on stdin and waits for human input.
func (n *HumanConfirmNode) Process(_ context.Context, state *State) error {
	// Display prominent warning.
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════╗")
	fmt.Println("║  ⚠️  HUMAN CONFIRMATION REQUIRED          ║")
	fmt.Println("╚══════════════════════════════════════════╝")
	fmt.Println()

	// Show context from state if available.
	if lastAction := state.GetString("tool_call_name"); lastAction != "" {
		fmt.Printf("  Pending action: %s\n", lastAction)
	}
	if params, ok := state.Get("tool_call_params"); ok {
		fmt.Printf("  Parameters: %v\n", params)
	}
	fmt.Println()
	fmt.Printf("  📋 %s\n\n", n.prompt)
	fmt.Print("  Enter [y]es to proceed or [n]o to abort: ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("human confirm: failed to read input: %w", err)
	}

	answer := strings.TrimSpace(strings.ToLower(input))

	switch answer {
	case "y", "yes":
		state.Set("human_approved", true)
		fmt.Println("  ✅ Approved by operator.")
		return nil
	default:
		state.Set("human_approved", false)
		fmt.Println("  🛑 Rejected by operator.")
		return ErrHumanRejected
	}
}
