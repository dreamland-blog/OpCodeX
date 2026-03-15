# basic_shell Example

A minimal, **self-contained** OpCodeX graph that queries system information
using shell skills — **no LLM or API key required**.

## What it demonstrates

- Creating and configuring an `UbuntuShellExecutor` + `ExecuteShellSkill`
- Building a 3-node pipeline: `CollectInfo → Format → Print`
- Edge-based routing with `AlwaysRoute` and conditional edges
- Reading/writing to shared `State` between nodes

## Graph Flow

```
┌─────────────┐    ┌──────────┐    ┌─────────┐
│ CollectInfo  │───▶│  Format  │───▶│  Print  │───▶ END
│ (shell cmds) │    │ (report)  │    │(output)  │
└─────────────┘    └──────────┘    └─────────┘
```

## Usage

```bash
go run ./examples/basic_shell
```

## Expected Output

```
⚡ [CollectInfo] Running system commands...
  ✓ Collected 4 data points
📝 [Format] Building report...

═══════════════════════════════════
       System Status Report
═══════════════════════════════════
  OS Info: Darwin hostname 24.0.0 ...
  Disk Usage: /dev/disk3s1 ...
  Memory: ...
  Uptime: ...
═══════════════════════════════════

✅ Done! Graph walk: CollectInfo → Format → Print → END
```
