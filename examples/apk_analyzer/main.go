// apk_analyzer demonstrates a realistic reverse-engineering pipeline.
//
// It chains four nodes into an automated APK analysis workflow:
//
//	PullAPK → Decompile → ScanSecrets → GenerateReport → END
//
// Prerequisites:
//   - adb (Android Debug Bridge) in PATH
//   - jadx (Dex to Java decompiler) in PATH
//   - A connected Android device or running emulator
//
// Run:
//
//	go run ./examples/apk_analyzer --package com.example.app
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/dreamland-blog/OpCodeX/pkg/graph"
	"github.com/dreamland-blog/OpCodeX/pkg/sandbox"
	"github.com/dreamland-blog/OpCodeX/pkg/skill"
)

func main() {
	pkg := flag.String("package", "com.example.app", "Android package name to analyze")
	flag.Parse()

	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║   APK Analyzer — OpCodeX Demo        ║")
	fmt.Println("║   Reverse Engineering Pipeline        ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Printf("\n🎯 Target package: %s\n\n", *pkg)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	state := graph.NewState()
	state.Set("package", *pkg)
	state.Set("work_dir", "/tmp/opcodex/apk_analysis")

	// Skills
	executor := sandbox.NewUbuntuShellExecutor()
	shellSkill := skill.NewExecuteShellSkill(executor)
	writeSkill := skill.NewWriteTextFileSkill()

	// ─────────────────────────────────────────────────
	// Node 1: PullAPK — extract APK from device via adb
	// ─────────────────────────────────────────────────
	pullNode := graph.NewActionNode("PullAPK", func(ctx context.Context, s *graph.State) error {
		pkg := s.GetString("package")
		workDir := s.GetString("work_dir")
		fmt.Printf("📱 [PullAPK] Extracting %s from device...\n", pkg)

		// Create work directory
		shellSkill.Execute(ctx, skill.SkillInput{
			Parameters: map[string]any{"command": "mkdir -p " + workDir},
		})

		// Get APK path from device
		out, err := shellSkill.Execute(ctx, skill.SkillInput{
			Parameters: map[string]any{
				"command": fmt.Sprintf("adb shell pm path %s 2>/dev/null || echo 'package:%s.apk'", pkg, pkg),
			},
		})
		if err != nil {
			return fmt.Errorf("pull apk: %w", err)
		}

		apkPath := strings.TrimPrefix(strings.TrimSpace(out.RawText), "package:")
		apkLocal := workDir + "/target.apk"

		// Pull APK to local
		result, err := shellSkill.Execute(ctx, skill.SkillInput{
			Parameters: map[string]any{
				"command": fmt.Sprintf("adb pull '%s' '%s' 2>/dev/null || echo 'DEMO: would pull %s to %s'", apkPath, apkLocal, apkPath, apkLocal),
			},
		})
		if err != nil {
			return fmt.Errorf("pull apk: %w", err)
		}

		s.Set("apk_path", apkLocal)
		fmt.Printf("  ✓ APK: %s\n", strings.TrimSpace(result.RawText))
		return nil
	})

	// ─────────────────────────────────────────────────
	// Node 2: Decompile — jadx decompilation
	// ─────────────────────────────────────────────────
	decompileNode := graph.NewActionNode("Decompile", func(ctx context.Context, s *graph.State) error {
		apkPath := s.GetString("apk_path")
		workDir := s.GetString("work_dir")
		srcDir := workDir + "/source"
		fmt.Printf("🔧 [Decompile] Decompiling with jadx...\n")

		result, err := shellSkill.Execute(ctx, skill.SkillInput{
			Parameters: map[string]any{
				"command": fmt.Sprintf("jadx -d '%s' '%s' 2>/dev/null || echo 'DEMO: would decompile %s → %s'", srcDir, apkPath, apkPath, srcDir),
			},
		})
		if err != nil {
			return fmt.Errorf("decompile: %w", err)
		}

		s.Set("source_dir", srcDir)
		fmt.Printf("  ✓ %s\n", strings.TrimSpace(result.RawText))
		return nil
	})

	// ─────────────────────────────────────────────────
	// Node 3: ScanSecrets — grep for hardcoded secrets
	// ─────────────────────────────────────────────────
	scanNode := graph.NewActionNode("ScanSecrets", func(ctx context.Context, s *graph.State) error {
		srcDir := s.GetString("source_dir")
		fmt.Println("🔍 [ScanSecrets] Scanning for hardcoded secrets...")

		patterns := []string{
			"api[_-]?key",
			"secret[_-]?key",
			"password",
			"token",
			"Bearer ",
			"https?://[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}",
		}

		var findings []string
		for _, pat := range patterns {
			out, _ := shellSkill.Execute(ctx, skill.SkillInput{
				Parameters: map[string]any{
					"command": fmt.Sprintf(
						"grep -rniE '%s' '%s' 2>/dev/null | head -5 || echo '  (no matches for %s)'",
						pat, srcDir, pat,
					),
				},
			})
			text := strings.TrimSpace(out.RawText)
			if text != "" {
				findings = append(findings, fmt.Sprintf("### Pattern: `%s`\n```\n%s\n```", pat, text))
			}
		}

		s.Set("findings", findings)
		fmt.Printf("  ✓ Scanned %d patterns, found %d with results\n", len(patterns), len(findings))
		return nil
	})

	// ─────────────────────────────────────────────────
	// Node 4: GenerateReport — write Markdown report
	// ─────────────────────────────────────────────────
	reportNode := graph.NewActionNode("GenerateReport", func(ctx context.Context, s *graph.State) error {
		fmt.Println("📊 [GenerateReport] Writing analysis report...")

		pkg := s.GetString("package")
		raw, _ := s.Get("findings")
		findings, _ := raw.([]string)

		var report strings.Builder
		report.WriteString(fmt.Sprintf("# APK Analysis Report: %s\n\n", pkg))
		report.WriteString(fmt.Sprintf("**Generated**: %s\n\n", time.Now().Format(time.RFC3339)))
		report.WriteString("## Secrets Scan Results\n\n")

		if len(findings) == 0 {
			report.WriteString("✅ No hardcoded secrets found.\n")
		} else {
			for _, f := range findings {
				report.WriteString(f + "\n\n")
			}
		}

		reportPath := "/tmp/opcodex/apk_report.md"
		_, err := writeSkill.Execute(ctx, skill.SkillInput{
			Parameters: map[string]any{
				"path":    reportPath,
				"content": report.String(),
			},
		})
		if err != nil {
			return fmt.Errorf("report: %w", err)
		}

		s.Set("report_path", reportPath)
		s.Set("is_done", true)
		fmt.Printf("  ✓ Report saved to %s\n", reportPath)
		return nil
	})

	// ── Assemble graph ──
	engine := graph.NewEngine()
	engine.AddNode(pullNode)
	engine.AddNode(decompileNode)
	engine.AddNode(scanNode)
	engine.AddNode(reportNode)

	engine.AddEdge("PullAPK", "Decompile", graph.AlwaysRoute)
	engine.AddEdge("Decompile", "ScanSecrets", graph.AlwaysRoute)
	engine.AddEdge("ScanSecrets", "GenerateReport", graph.AlwaysRoute)
	engine.AddEdge("GenerateReport", graph.EndNodeID, func(s *graph.State) bool {
		return s.GetBool("is_done")
	})

	// ── Run ──
	if err := engine.Run(ctx, state, "PullAPK"); err != nil {
		log.Fatalf("❌ %v", err)
	}

	fmt.Println("\n✅ Analysis complete!")
	fmt.Printf("   Graph: PullAPK → Decompile → ScanSecrets → GenerateReport → END\n")
	fmt.Printf("   Report: %s\n", state.GetString("report_path"))
}
