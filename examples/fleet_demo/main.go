// fleet_demo demonstrates concurrent multi-device task execution.
//
// It creates 3 simulated "devices" (all using local shell as the
// executor) and dispatches different objectives to each one in parallel.
//
// Run:
//
//	go run ./examples/fleet_demo
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/dreamland-blog/OpCodeX/internal/llm"
	"github.com/dreamland-blog/OpCodeX/pkg/fleet"
	"github.com/dreamland-blog/OpCodeX/pkg/sandbox"
)

func main() {
	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║   Fleet Demo — Concurrent Devices    ║")
	fmt.Println("║   Project Taiyi Scheduling Engine     ║")
	fmt.Println("╚══════════════════════════════════════╝")
	fmt.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// ── Build device fleet ──
	devices := sandbox.NewDeviceFleet()

	devices.Add(&sandbox.Device{
		ID:       "device-alpha",
		Name:     "Alpha (磁盘探测)",
		Type:     sandbox.DeviceLocal,
		Executor: sandbox.NewUbuntuShellExecutor(),
		Tags:     []string{"local", "monitor"},
	})

	devices.Add(&sandbox.Device{
		ID:       "device-beta",
		Name:     "Beta (内存探测)",
		Type:     sandbox.DeviceLocal,
		Executor: sandbox.NewUbuntuShellExecutor(),
		Tags:     []string{"local", "monitor"},
	})

	devices.Add(&sandbox.Device{
		ID:       "device-gamma",
		Name:     "Gamma (网络探测)",
		Type:     sandbox.DeviceLocal,
		Executor: sandbox.NewUbuntuShellExecutor(),
		Tags:     []string{"local", "network"},
	})

	fmt.Printf("📱 Fleet size: %d devices\n", devices.Count())
	for _, d := range devices.All() {
		fmt.Printf("   • %s [%s] (%s)\n", d.Name, d.ID, d.Type)
	}
	fmt.Println()

	// ── Configure fleet scheduler ──
	scheduler := &fleet.Fleet{
		Devices: devices,
		NewLLM: func() llm.Adapter {
			return llm.NewMockAdapter()
		},
		MaxWorkers: 3, // all 3 in parallel
	}

	// ── Dispatch tasks ──
	// Each device gets the same objective (in real usage, you'd vary these).
	tasks := []fleet.Task{
		{Objective: "检查当前系统信息并返回一份简短的中文报告"},
	}

	fmt.Println("🚀 Dispatching tasks to all devices...")
	start := time.Now()

	results := scheduler.Run(ctx, tasks)

	elapsed := time.Since(start)
	fmt.Println()

	// ── Print results ──
	fmt.Println("════════════════════════════════════════")
	fmt.Println("         Fleet Execution Results        ")
	fmt.Println("════════════════════════════════════════")
	fmt.Println()

	for i, r := range results {
		status := "✅"
		if r.Error != nil {
			status = "❌"
		}
		fmt.Printf("─── Device %d: %s ───\n", i+1, r.DevName)
		fmt.Printf("  %s Status: %s\n", status, func() string {
			if r.Error != nil {
				return r.Error.Error()
			}
			return "Success"
		}())
		fmt.Printf("  🆔 Run:     %s\n", r.RunID)
		fmt.Printf("  💬 Turns:   %d\n", r.Turns)
		fmt.Printf("  ⏱  Duration: %s\n", r.Duration.Round(time.Millisecond))
		if r.Report != "" {
			fmt.Printf("  📊 Report:  %s\n", r.Report)
		}
		fmt.Println()
	}

	fmt.Printf("📊 Total time: %s (parallel execution)\n", elapsed.Round(time.Millisecond))
	fmt.Printf("   Speedup: %.1fx vs sequential\n", func() float64 {
		var totalDuration time.Duration
		for _, r := range results {
			totalDuration += r.Duration
		}
		if elapsed == 0 {
			return 1.0
		}
		return float64(totalDuration) / float64(elapsed)
	}())
}
