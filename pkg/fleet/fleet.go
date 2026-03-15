// Package fleet provides a concurrent task scheduler that dispatches
// OpCodeX engine instances across multiple devices in parallel.
//
// Each device gets its own Engine + State + Skill set, running in a
// dedicated goroutine. Results are collected and returned as a batch.
package fleet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dreamland-blog/OpCodeX/internal/llm"
	"github.com/dreamland-blog/OpCodeX/pkg/graph"
	"github.com/dreamland-blog/OpCodeX/pkg/sandbox"
	"github.com/dreamland-blog/OpCodeX/pkg/skill"
)

// Task describes a unit of work to be dispatched to a device.
type Task struct {
	// Objective is the natural-language instruction for the LLM.
	Objective string

	// DeviceID targets a specific device. If empty, the task is
	// dispatched to ALL devices in the fleet.
	DeviceID string
}

// TaskResult captures the outcome of a single device execution.
type TaskResult struct {
	DeviceID  string        `json:"device_id"`
	DevName   string        `json:"device_name"`
	RunID     string        `json:"run_id"`
	Report    string        `json:"report"`
	Turns     int           `json:"turns"`
	Duration  time.Duration `json:"duration"`
	Error     error         `json:"error,omitempty"`
}

// LLMFactory creates a fresh LLM Adapter for each goroutine.
// This avoids sharing mutable state across concurrent engine instances.
type LLMFactory func() llm.Adapter

// GraphBuilder is called once per device to set up the graph (nodes + edges).
// It receives the engine, the device's skill registry, and the LLM adapter.
// This lets callers customize the graph topology per device.
type GraphBuilder func(engine *graph.Engine, registry *skill.Registry, adapter llm.Adapter)

// Fleet is the concurrent device scheduler.
type Fleet struct {
	// Devices is the fleet of target devices.
	Devices *sandbox.DeviceFleet

	// NewLLM creates a fresh adapter per worker goroutine.
	NewLLM LLMFactory

	// BuildGraph sets up the graph nodes and edges.
	// If nil, a default Brain→SystemOps loop is used.
	BuildGraph GraphBuilder

	// MaxWorkers limits concurrency. 0 = one worker per device (no limit).
	MaxWorkers int
}

// Run dispatches tasks to devices and returns all results.
//
// For tasks with DeviceID set: run on that specific device.
// For tasks with DeviceID empty: run on ALL devices.
//
// All executions happen concurrently (up to MaxWorkers).
func (f *Fleet) Run(ctx context.Context, tasks []Task) []TaskResult {
	// Expand tasks: resolve "all devices" tasks.
	type dispatchItem struct {
		device    *sandbox.Device
		objective string
	}
	var items []dispatchItem

	for _, t := range tasks {
		if t.DeviceID != "" {
			dev := f.Devices.Get(t.DeviceID)
			if dev == nil {
				continue
			}
			items = append(items, dispatchItem{device: dev, objective: t.Objective})
		} else {
			for _, dev := range f.Devices.All() {
				items = append(items, dispatchItem{device: dev, objective: t.Objective})
			}
		}
	}

	// Concurrency control.
	workers := f.MaxWorkers
	if workers <= 0 {
		workers = len(items)
	}
	sem := make(chan struct{}, workers)

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results []TaskResult
	)

	for _, item := range items {
		wg.Add(1)
		go func(dev *sandbox.Device, objective string) {
			defer wg.Done()

			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release

			result := f.runOnDevice(ctx, dev, objective)

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(item.device, item.objective)
	}

	wg.Wait()
	return results
}

// runOnDevice creates a full Engine+State for one device and runs it.
func (f *Fleet) runOnDevice(ctx context.Context, dev *sandbox.Device, objective string) TaskResult {
	start := time.Now()
	runID := fmt.Sprintf("%s_%d", dev.ID, time.Now().UnixNano())

	result := TaskResult{
		DeviceID: dev.ID,
		DevName:  dev.Name,
		RunID:    runID,
	}

	// Create device-specific state.
	state := graph.NewState()
	state.Set("objective", objective)
	state.Set("device_id", dev.ID)
	state.Set("device_name", dev.Name)
	state.Set("device_type", string(dev.Type))
	state.AddMessage(graph.MessageEntry{
		Role: "user",
		Text: objective,
	})

	// Create device-specific skill registry with this device's executor.
	registry := skill.NewRegistry()
	registry.Register(skill.NewExecuteShellSkill(dev.Executor))
	registry.Register(skill.NewReadTextFileSkill())
	registry.Register(skill.NewWriteTextFileSkill())
	registry.Register(skill.NewHTTPRequestSkill())

	// Create LLM adapter for this goroutine.
	adapter := f.NewLLM()

	// Create and configure the engine.
	engine := graph.NewEngine()

	if f.BuildGraph != nil {
		f.BuildGraph(engine, registry, adapter)
	} else {
		f.buildDefaultGraph(engine, registry, adapter)
	}

	// Run.
	if err := engine.Run(ctx, state, "Brain"); err != nil {
		result.Error = err
	}

	result.Report = state.GetString("final_report")
	result.Turns = len(state.GetMessages())
	result.Duration = time.Since(start)

	return result
}

// buildDefaultGraph sets up the standard Brain→SystemOps loop.
func (f *Fleet) buildDefaultGraph(engine *graph.Engine, registry *skill.Registry, adapter llm.Adapter) {
	// Build tool definitions.
	var toolDefs []llm.ToolDef
	for _, s := range registry.All() {
		toolDefs = append(toolDefs, llm.ToolDef{
			Name:        s.Name(),
			Description: s.Description(),
			Parameters:  s.InputSchema(),
		})
	}

	// Brain node.
	brainNode := graph.NewActionNode("Brain", func(ctx context.Context, s *graph.State) error {
		devName := s.GetString("device_name")

		// Handle error recovery.
		if lastErr := s.GetString("last_error"); lastErr != "" {
			s.AddMessage(graph.MessageEntry{
				Role: "user",
				Text: fmt.Sprintf("[SYSTEM ERROR] %s. Try a different approach.", lastErr),
			})
		}

		history := s.GetMessages()
		var messages []llm.Message
		for _, entry := range history {
			switch {
			case entry.FuncCall != nil:
				messages = append(messages, llm.Message{
					Role:    llm.RoleAssistant,
					Content: fmt.Sprintf("[calling %s(%v)]", entry.FuncCall.Name, entry.FuncCall.Args),
				})
			case entry.FuncResp != nil:
				messages = append(messages, llm.Message{
					Role:    llm.RoleTool,
					Content: fmt.Sprintf("%v", entry.FuncResp.Result),
				})
			default:
				role := llm.RoleUser
				if entry.Role == "model" {
					role = llm.RoleAssistant
				}
				messages = append(messages, llm.Message{Role: role, Content: entry.Text})
			}
		}

		fmt.Printf("  [%s] 📨 %d messages → LLM\n", devName, len(messages))

		resp, err := adapter.Chat(ctx, messages, toolDefs)
		if err != nil {
			return fmt.Errorf("brain(%s): %w", devName, err)
		}

		if len(resp.ToolCalls) > 0 {
			tc := resp.ToolCalls[0]
			s.Set("next_action", tc.Name)
			s.Set("tool_call_name", tc.Name)
			s.Set("tool_call_params", tc.Parameters)
			s.Set("is_task_completed", false)
			s.AddMessage(graph.MessageEntry{
				Role:     "model",
				FuncCall: &graph.FuncCallEntry{Name: tc.Name, Args: tc.Parameters},
			})
		} else {
			s.Set("next_action", "")
			s.Set("is_task_completed", true)
			s.Set("final_report", resp.Content)
			s.AddMessage(graph.MessageEntry{Role: "model", Text: resp.Content})
		}
		return nil
	})

	// SystemOps node.
	opsNode := graph.NewActionNode("SystemOps", func(ctx context.Context, s *graph.State) error {
		devName := s.GetString("device_name")
		skillName := s.GetString("tool_call_name")
		target := registry.Get(skillName)
		if target == nil {
			return fmt.Errorf("systemops(%s): unknown skill %q", devName, skillName)
		}

		params, _ := s.Get("tool_call_params")
		paramsMap, _ := params.(map[string]any)

		fmt.Printf("  [%s] ⚡ %s\n", devName, skillName)
		output, err := target.Execute(ctx, skill.SkillInput{Parameters: paramsMap})
		if err != nil {
			return fmt.Errorf("systemops(%s): %w", devName, err)
		}

		s.Set("observation", output.RawText)
		s.AddMessage(graph.MessageEntry{
			Role:     "user",
			FuncResp: &graph.FuncRespEntry{Name: skillName, Result: output.Result},
		})
		return nil
	})

	engine.AddNode(brainNode)
	engine.AddNode(opsNode)

	engine.AddEdge("Brain", "SystemOps", func(s *graph.State) bool {
		action := s.GetString("next_action")
		return action != "" && registry.Get(action) != nil
	})
	engine.AddEdge("SystemOps", "Brain", graph.AlwaysRoute)
	engine.AddEdge("Brain", graph.EndNodeID, func(s *graph.State) bool {
		return s.GetBool("is_task_completed")
	})
	engine.AddErrorEdge("SystemOps", "Brain")
}
