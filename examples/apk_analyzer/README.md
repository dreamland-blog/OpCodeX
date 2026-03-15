# apk_analyzer Example

An OpCodeX graph that automates Android APK reverse-engineering through a
4-node pipeline, demonstrating multi-skill orchestration.

## What it demonstrates

- Chaining 4 nodes: `PullAPK → Decompile → ScanSecrets → GenerateReport`
- Using `shell_exec` and `write_file` skills together
- Generating a structured Markdown report
- Context propagation via shared `State`

## Graph Flow

```
┌──────────┐   ┌───────────┐   ┌─────────────┐   ┌────────────────┐
│ PullAPK  │──▶│ Decompile │──▶│ ScanSecrets │──▶│ GenerateReport │──▶ END
│ (adb)    │   │  (jadx)   │   │   (grep)    │   │  (write_file)  │
└──────────┘   └───────────┘   └─────────────┘   └────────────────┘
```

## Prerequisites

| Tool | Purpose |
|------|---------|
| `adb` | Pull APK from connected Android device |
| `jadx` | Decompile DEX bytecode to Java source |

> **Note**: If tools are not installed, the example runs in demo mode
> and prints what it *would* do.

## Usage

```bash
# Analyze a specific package
go run ./examples/apk_analyzer --package com.tiktok.android

# Default demo
go run ./examples/apk_analyzer
```

## Output

The pipeline generates `/tmp/opcodex/apk_report.md` containing:
- Package metadata
- Hardcoded secrets scan results (API keys, tokens, URLs)
