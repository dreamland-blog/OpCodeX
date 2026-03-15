// basic_shell demonstrates the simplest possible OpCodeX graph.
//
// It chains three nodes WITHOUT an LLM — pure graph-engine mechanics:
//
//	CollectInfo → Format → Print → END
//
// Run:
//
//	go run ./examples/basic_shell
package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/dreamland-blog/OpCodeX/pkg/graph"
	"github.com/dreamland-blog/OpCodeX/pkg/sandbox"
	"github.com/dreamland-blog/OpCodeX/pkg/skill"
)

func main() {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║   basic_shell — OpCodeX Demo         ║")
	fmt.Println("║   Pure Graph Walk (no LLM needed)    ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	ctx := context.Background()
	state := graph.NewState()

	// Set up the shell skill.
	executor := sandbox.NewUbuntuShellExecutor()
	shellSkill := skill.NewExecuteShellSkill(executor)

	// ── Define commands to gather system info ──
	commands := []struct {
		Label   string
		Command string
	}{
		{"OS Info", "uname -a"},
		{"Disk Usage", "df -h / | tail -1"},
		{"Memory", "vm_stat | head -5 2>/dev/null || free -m 2>/dev/null || echo 'memory info unavailable'"},
		{"Uptime", "uptime"},
	}

	// ─────────────────────────────────────────────────
	// Node 1: CollectInfo — runs all shell commands
	// ─────────────────────────────────────────────────
	collectNode := graph.NewActionNode("CollectInfo", func(ctx context.Context, s *graph.State) error {
		fmt.Println("⚡ [CollectInfo] Running system commands...")
		var results []string

		for _, cmd := range commands {
			output, err := shellSkill.Execute(ctx, skill.SkillInput{
				Parameters: map[string]any{"command": cmd.Command},
			})
			if err != nil {
				results = append(results, fmt.Sprintf("  %s: ⚠️ %v", cmd.Label, err))
				continue
			}
			text := strings.TrimSpace(output.RawText)
			results = append(results, fmt.Sprintf("  %s: %s", cmd.Label, text))
		}

		s.Set("raw_results", results)
		fmt.Printf("  ✓ Collected %d data points\n", len(results))
		return nil
	})

	// ─────────────────────────────────────────────────
	// Node 2: Format — builds a human-readable report
	// ─────────────────────────────────────────────────
	formatNode := graph.NewActionNode("Format", func(_ context.Context, s *graph.State) error {
		fmt.Println("📝 [Format] Building report...")
		raw, _ := s.Get("raw_results")
		results, _ := raw.([]string)

		var report strings.Builder
		report.WriteString("═══════════════════════════════════\n")
		report.WriteString("       System Status Report\n")
		report.WriteString("═══════════════════════════════════\n\n")
		for _, line := range results {
			report.WriteString(line + "\n")
		}
		report.WriteString("\n═══════════════════════════════════")

		s.Set("report", report.String())
		return nil
	})

	// ─────────────────────────────────────────────────
	// Node 3: Print — outputs the final report
	// ─────────────────────────────────────────────────
	printNode := graph.NewActionNode("Print", func(_ context.Context, s *graph.State) error {
		fmt.Println()
		fmt.Println(s.GetString("report"))
		s.Set("is_done", true)
		return nil
	})

	// ── Assemble the graph ──
	engine := graph.NewEngine()
	engine.AddNode(collectNode)
	engine.AddNode(formatNode)
	engine.AddNode(printNode)

	engine.AddEdge("CollectInfo", "Format", graph.AlwaysRoute)
	engine.AddEdge("Format", "Print", graph.AlwaysRoute)
	engine.AddEdge("Print", graph.EndNodeID, func(s *graph.State) bool {
		return s.GetBool("is_done")
	})

	// ── Run ──
	if err := engine.Run(ctx, state, "CollectInfo"); err != nil {
		log.Fatalf("❌ %v", err)
	}

	fmt.Println("\n✅ Done! Graph walk: CollectInfo → Format → Print → END")
}
