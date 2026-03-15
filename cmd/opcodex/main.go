// Package main is the entry point for the OpCodeX execution engine.
//
// This wires all subsystems together and runs the complete
// Think → Execute → Observe → Summarize loop with full conversation memory,
// error-edge self-healing, and optional state persistence.
//
// Usage:
//
//	go run ./cmd/opcodex                        # mock mode (default)
//	go run ./cmd/opcodex --mode=gemini          # real Gemini API
//	go run ./cmd/opcodex --resume=<runID>       # resume from checkpoint
//
// Environment:
//
//	GEMINI_API_KEY   — required in gemini mode
//	GEMINI_MODEL     — optional (default: gemini-2.0-flash)
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/dreamland-blog/OpCodeX/internal/llm"
	"github.com/dreamland-blog/OpCodeX/pkg/graph"
	"github.com/dreamland-blog/OpCodeX/pkg/sandbox"
	"github.com/dreamland-blog/OpCodeX/pkg/skill"
)

func main() {
	// ── Flags ──────────────────────────────────────────────────────────
	mode := flag.String("mode", "mock", "LLM backend: mock or gemini")
	resumeID := flag.String("resume", "", "Resume from a checkpoint (run ID)")
	flag.Parse()

	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║        OpCodeX Engine v0.3.0         ║")
	fmt.Println("║   Autonomous Execution Framework     ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	ctx := context.Background()

	// ── 1. State Persistence ───────────────────────────────────────────
	store, err := graph.NewBoltStateStore("")
	if err != nil {
		log.Fatalf("Failed to open state store: %v", err)
	}
	defer store.Close()
	fmt.Println("State store: ~/.opcodex/state.db")

	// ── 2. State ────────────────────────────────────────────────────────
	var state *graph.State
	var startNode string
	runID := fmt.Sprintf("run_%d", time.Now().Unix())

	if *resumeID != "" {
		// Resume mode: load from checkpoint.
		loaded, err := store.Load(*resumeID)
		if err != nil {
			log.Fatalf("Failed to load checkpoint: %v", err)
		}
		if loaded == nil {
			log.Fatalf("No checkpoint found for run ID: %s", *resumeID)
		}
		state = loaded
		runID = *resumeID
		startNode = state.GetString("__checkpoint_node__")
		fmt.Printf("Resuming run [%s] from node [%s]\n", runID, startNode)
		fmt.Printf("Restored %d conversation turns\n", len(state.GetMessages()))
	} else {
		// Fresh run.
		state = graph.NewState()
		objective := "检查当前系统信息并返回一份简短的中文报告"
		state.Set("objective", objective)
		startNode = "Brain"
		fmt.Println("Objective:", objective)

		// Seed the conversation with the user's objective.
		state.AddMessage(graph.MessageEntry{
			Role: "user",
			Text: objective,
		})
	}

	fmt.Printf("Run ID: %s\n", runID)

	// ── 3. Sandbox ─────────────────────────────────────────────────────
	shellExecutor := sandbox.NewUbuntuShellExecutor()

	// ── 4. Skill Registry ──────────────────────────────────────────────
	registry := skill.NewRegistry()
	registry.Register(skill.NewExecuteShellSkill(shellExecutor))
	registry.Register(skill.NewReadTextFileSkill())
	registry.Register(skill.NewWriteTextFileSkill())
	registry.Register(skill.NewHTTPRequestSkill())

	// Build tool definitions from the registry for the LLM.
	var toolDefs []llm.ToolDef
	for _, s := range registry.All() {
		toolDefs = append(toolDefs, llm.ToolDef{
			Name:        s.Name(),
			Description: s.Description(),
			Parameters:  s.InputSchema(),
		})
	}

	// ── 5. LLM Adapter ────────────────────────────────────────────────
	var brain llm.Adapter
	switch *mode {
	case "gemini":
		apiKey := os.Getenv("GEMINI_API_KEY")
		if apiKey == "" {
			log.Fatal(" GEMINI_API_KEY environment variable is required in gemini mode")
		}
		model := os.Getenv("GEMINI_MODEL")
		if model == "" {
			model = "gemini-2.0-flash"
		}
		adapter := llm.NewGeminiAdapter(apiKey, model)
		adapter.SystemInstruction = "你是 OpCodeX 自动化引擎的大脑。你拥有一组系统工具（如 shell_exec），可以在宿主机上执行命令。当你认为需要获取系统信息时，请调用对应的工具。如果之前有报错信息（last_error），请分析错误并尝试用修正后的参数重试。完成任务后，请直接用中文输出最终报告。"
		brain = adapter
		fmt.Printf("LLM: Gemini (%s)\n", model)
	default:
		brain = llm.NewMockAdapter()
		fmt.Println("LLM: MockAdapter (offline testing)")
	}
	fmt.Println()

	// ── 6. Build Graph ─────────────────────────────────────────────────
	engine := graph.NewEngine()
	engine.StateStore = store
	engine.RunID = runID

	// ─── Brain Node ────────────────────────────────────────────────────
	brainNode := graph.NewActionNode("Brain", func(ctx context.Context, s *graph.State) error {
		fmt.Println("──────────── Brain Node ────────────")

		// If we arrived via an error edge, inject the error into conversation.
		if lastErr := s.GetString("last_error"); lastErr != "" {
			errNode := s.GetString("error_node")
			retryCount, _ := s.Get("retry_count")
			fmt.Printf("  Self-healing: error from [%s], retry #%v\n", errNode, retryCount)

			s.AddMessage(graph.MessageEntry{
				Role: "user",
				Text: fmt.Sprintf("[SYSTEM ERROR] Node '%s' failed: %s. Please analyze the error and try a different approach.", errNode, lastErr),
			})
		}

		// Build messages from conversation history.
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
				resultJSON := fmt.Sprintf("%v", entry.FuncResp.Result)
				messages = append(messages, llm.Message{
					Role:    llm.RoleTool,
					Content: resultJSON,
				})
			default:
				role := llm.RoleUser
				if entry.Role == "model" {
					role = llm.RoleAssistant
				}
				messages = append(messages, llm.Message{
					Role:    role,
					Content: entry.Text,
				})
			}
		}

		fmt.Printf("   Sending %d messages to LLM (%d tools available)\n", len(messages), len(toolDefs))

		resp, err := brain.Chat(ctx, messages, toolDefs)
		if err != nil {
			return fmt.Errorf("brain: %w", err)
		}

		if len(resp.ToolCalls) > 0 {
			tc := resp.ToolCalls[0]
			s.Set("next_action", tc.Name)
			s.Set("tool_call_name", tc.Name)
			s.Set("tool_call_params", tc.Parameters)
			s.Set("is_task_completed", false)

			s.AddMessage(graph.MessageEntry{
				Role: "model",
				FuncCall: &graph.FuncCallEntry{
					Name: tc.Name,
					Args: tc.Parameters,
				},
			})
			fmt.Printf("  → Tool call: %s(%v)\n", tc.Name, tc.Parameters)
		} else {
			s.Set("next_action", "")
			s.Set("is_task_completed", true)
			s.Set("final_report", resp.Content)

			s.AddMessage(graph.MessageEntry{
				Role: "model",
				Text: resp.Content,
			})
			fmt.Println("  → Task completed. Report received.")
		}
		return nil
	})

	// ─── SystemOps Node ────────────────────────────────────────────────
	opsNode := graph.NewActionNode("SystemOps", func(ctx context.Context, s *graph.State) error {
		fmt.Println("──────────── SystemOps Node ────────")

		skillName := s.GetString("tool_call_name")
		target := registry.Get(skillName)
		if target == nil {
			return fmt.Errorf("systemops: unknown skill %q", skillName)
		}

		params, _ := s.Get("tool_call_params")
		paramsMap, _ := params.(map[string]any)

		fmt.Printf("  Executing skill [%s]...\n", skillName)
		output, err := target.Execute(ctx, skill.SkillInput{Parameters: paramsMap})
		if err != nil {
			return fmt.Errorf("systemops: skill %q failed: %w", skillName, err)
		}

		s.Set("observation", output.RawText)

		s.AddMessage(graph.MessageEntry{
			Role: "user",
			FuncResp: &graph.FuncRespEntry{
				Name:   skillName,
				Result: output.Result,
			},
		})

		fmt.Printf("  Skill output captured (%d bytes)\n", len(output.RawText))
		return nil
	})

	engine.AddNode(brainNode)
	engine.AddNode(opsNode)

	// ── 7. Wire Edges ──────────────────────────────────────────────────
	// Brain → SystemOps : when the LLM wants to invoke any registered tool
	engine.AddEdge("Brain", "SystemOps", func(s *graph.State) bool {
		action := s.GetString("next_action")
		return action != "" && registry.Get(action) != nil
	})

	// SystemOps → Brain : always loop back for observation
	engine.AddEdge("SystemOps", "Brain", graph.AlwaysRoute)

	// Brain → END : when the LLM declares the task complete
	engine.AddEdge("Brain", graph.EndNodeID, func(s *graph.State) bool {
		return s.GetBool("is_task_completed")
	})

	// ── Error Edges (self-healing) ──
	// SystemOps fails → route back to Brain with error context
	engine.AddErrorEdge("SystemOps", "Brain")

	// ── 8. Ignition ────────────────────────────────────────────────────
	fmt.Printf("Engine starting from [%s] node...\n\n", startNode)

	if err := engine.Run(ctx, state, startNode); err != nil {
		log.Fatalf("Engine halted with error: %v", err)
	}

	// ── Summary ────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("════════════════════════════════════════")
	fmt.Println("Task Completed Successfully!")
	fmt.Println("════════════════════════════════════════")
	fmt.Printf("Final Report:\n   %s\n", state.GetString("final_report"))

	history := state.GetMessages()
	fmt.Printf("\nConversation history: %d turns\n", len(history))
	fmt.Printf("State checkpointed as: %s\n", runID)
}
