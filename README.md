# OpCodeX

<p align="center">
  <strong>可编程的图驱动自主执行引擎</strong><br>
  <em>Programmable Graph-Based Autonomous Execution Engine</em>
</p>

<p align="center">
  <code>Go 1.23</code> · <code>Gemini API</code> · <code>bbolt Persistence</code> · <code>Fleet Scheduling</code>
</p>

---

## 什么是 OpCodeX？

OpCodeX 是一个用纯 Go 编写的**底层自动化执行框架**。它通过有向图（DAG）编排一组原子技能（Skill），由大语言模型（LLM）充当决策大脑，驱动「思考 → 执行 → 观察 → 总结」的自主闭环。

**核心定位**：Project OpCodeX 的钛合金地基 —— 面向群控调度、逆向分析、安全审计等场景的底层引擎。

### 核心特性

| 特性 | 描述 |
|------|------|
| 图引擎 | 边驱动路由，支持条件分支、错误重试、人工确认 |
| 技能系统 | 4 个内置 Skill + JSON Schema 自动生成 |
| 沙盒隔离 | 命令黑名单 + 输出截断 + Docker/ADB/SSH 多后端 |
| 状态持久化 | bbolt 嵌入式数据库，断电重启后断点恢复 |
| LLM 适配 | Gemini REST API + MockAdapter，可扩展 |
| 并发群控 | Fleet 调度器，goroutine 级设备并发 |

---

## 架构总览

```
                        ┌──────────────────┐
                        │   Fleet 调度器    │
                        │ goroutine/device  │
                        └───┬────┬────┬────┘
                            │    │    │
               ┌────────────┘    │    └────────────┐
               ▼                 ▼                  ▼
┌──────────────────────────────────────────────────────────┐
│                      Graph Engine                         │
│                                                           │
│   Brain ──→ SystemOps ──→ Brain ──→ END                  │
│     ↑          │ error                                    │
│     └──────────┘ (自愈重试, ≤3次)                          │
│                                                           │
│   [HumanConfirmNode] ← 高危操作人工确认                     │
├───────────────────────────────────────────────────────────┤
│            Shared State (KV + 对话记忆)                    │
│            StateStore → ~/.opcodex/state.db               │
└────────────┬────────────────────────────┬─────────────────┘
             │                            │
     ┌───────▼───────┐           ┌────────▼────────┐
     │   Skill Layer  │           │   LLM Adapter   │
     ├───────────────┤           ├─────────────────┤
     │ shell_exec    │           │ GeminiAdapter   │
     │ read_file     │           │ MockAdapter     │
     │ write_file    │           └─────────────────┘
     │ http_request  │
     └───────┬───────┘
             │
     ┌───────▼───────┐
     │    Sandbox     │
     ├───────────────┤
     │ UbuntuShell   │
     │ Docker        │
     │ ADB (Android) │
     │ SSH (Remote)  │
     └───────────────┘
```

---

## 项目结构

```
OpCodeX/
├── cmd/opcodex/main.go           # 引擎入口 (--mode, --resume)
├── pkg/
│   ├── graph/
│   │   ├── engine.go             # 图引擎 (边路由 + 错误边 + 重试)
│   │   ├── node.go               # ActionNode / RouterNode / HumanConfirmNode
│   │   ├── state.go              # 线程安全 KV + 对话记忆
│   │   └── persist.go            # StateStore 接口 + BoltStateStore
│   ├── skill/
│   │   ├── skill.go              # Skill 接口 + Registry
│   │   ├── schema.go             # 反射式 JSON Schema 生成器
│   │   ├── shell.go              # shell_exec
│   │   ├── read_file.go          # read_file
│   │   ├── write_file.go         # write_file
│   │   └── http_request.go       # http_request
│   ├── sandbox/
│   │   ├── executor.go           # Executor 接口
│   │   ├── ubuntu_shell.go       # 宿主机 Shell (加固版)
│   │   ├── docker.go             # Docker 容器隔离
│   │   ├── adb_executor.go       # Android ADB
│   │   ├── ssh_executor.go       # SSH 远程执行
│   │   └── device.go             # Device 抽象 + DeviceFleet
│   └── fleet/
│       └── fleet.go              # 并发设备调度器
├── internal/llm/
│   ├── adapter.go                # Adapter 接口 + ToolDef
│   ├── gemini.go                 # Gemini REST API 适配器
│   └── mock.go                   # Mock 适配器 (离线测试)
├── examples/
│   ├── basic_shell/              # 纯图引擎 Demo (无需 LLM)
│   ├── apk_analyzer/             # APK 逆向分析流水线
│   └── fleet_demo/               # 并发群控 Demo
├── go.mod
├── ROADMAP.md
└── README.md
```

---

## 快速开始

### 环境要求

- **Go 1.23+**
- **Docker**（可选，用于 Docker 沙盒）
- **adb**（可选，用于 Android 设备控制）

### 编译

```bash
git clone https://github.com/dreamland-blog/OpCodeX.git
cd OpCodeX
go build ./...
```

### 运行引擎 (Mock 模式)

无需 API Key，使用内置 MockAdapter 模拟 LLM 行为：

```bash
go run ./cmd/opcodex
```

**输出示例：**
```
╔══════════════════════════════════════╗
║        OpCodeX Engine v0.3.0         ║
╚══════════════════════════════════════╝

State store: ~/.opcodex/state.db
LLM: MockAdapter (offline testing)

──────────── Brain Node ────────────
  Sending 1 messages to LLM (4 tools available)
  → Tool call: shell_exec(uname -a && uptime)
──────────── SystemOps Node ────────
  ⚡ Executing skill [shell_exec]...
  ✓ Skill output captured (219 bytes)
──────────── Brain Node ────────────
  → Task completed. Report received.

Task Completed Successfully!
State checkpointed as: run_1773551404
```

### 运行引擎 (Gemini 模式)

接入真实的 Gemini API：

```bash
export GEMINI_API_KEY="你的 API Key"
export GEMINI_MODEL="gemini-2.0-flash"    # 可选，默认值

go run ./cmd/opcodex --mode=gemini
```

### 从崩溃中恢复

引擎每次跳转后自动 checkpoint 到 `~/.opcodex/state.db`。如果进程意外退出：

```bash
go run ./cmd/opcodex --resume=run_1773551404
```

---

## 示例

### basic_shell — 纯图引擎 Demo

3 节点流水线，采集系统信息并生成报告，**无需 LLM**：

```bash
go run ./examples/basic_shell
```

```
CollectInfo → Format → Print → END
```

### apk_analyzer — APK 逆向分析

4 节点逆向流水线（adb pull → jadx 反编译 → grep 搜密钥 → 生成报告）：

```bash
go run ./examples/apk_analyzer --package com.tiktok.android
```

```
PullAPK → Decompile → ScanSecrets → GenerateReport → END
Report: /tmp/opcodex/apk_report.md
```

### fleet_demo — 并发群控

3 台设备并行执行，演示 goroutine 级调度：

```bash
go run ./examples/fleet_demo
```

```
Fleet size: 3 devices
Dispatching tasks to all devices...

─── Alpha ─── 4 turns, 12ms
─── Beta  ─── 4 turns, 12ms
─── Gamma ─── 4 turns, 12ms

Total: 12ms | Speedup: 3.0x vs sequential
```

---

## 技能清单

| 技能 | 描述 | 安全措施 |
|------|------|----------|
| `shell_exec` | 执行 Shell 命令 | 11 条正则黑名单 + 64KB 输出截断 + 超时 |
| `read_file` | 读取文本文件 | 路径遍历拦截 + `/proc/` `/sys/` 禁止 + 200 行限制 |
| `write_file` | 写入文本文件 | 限制写入 `/tmp/opcodex/` + 自动创建目录 |
| `http_request` | HTTP 请求 | 10s 超时 + 64KB 响应截断 |

---

## 沙盒后端

| 后端 | 用途 | 隔离级别 |
|------|------|----------|
| `UbuntuShellExecutor` | 宿主机直接执行 | 命令黑名单 + 输出截断 |
| `DockerExecutor` | Docker 容器内执行 | 进程/文件系统/网络隔离 + 128MB 内存限制 |
| `ADBExecutor` | Android 设备 (`adb shell`) | 设备级隔离 |
| `SSHExecutor` | 远程主机 (`ssh`) | 网络级隔离 |

---

## 图引擎特性

### 错误自愈 (Error Routes)

当节点执行失败时，引擎不会直接崩溃，而是：

1. 存储错误信息到 `state["last_error"]`
2. 沿 Error Edge 路由回 Brain 节点
3. LLM 分析错误日志并自动修正参数
4. 最多重试 3 次 (`MaxRetries`)

```go
engine.AddErrorEdge("SystemOps", "Brain")
```

### 人工确认 (Human-in-the-Loop)

对高危操作设置人工审批关卡：

```go
confirm := graph.NewHumanConfirmNode("Confirm", "即将删除目标文件，是否继续？")
engine.AddNode(confirm)
engine.AddEdge("Brain", "Confirm", isHighRisk)
engine.AddEdge("Confirm", "SystemOps", graph.AlwaysRoute)
```

### 状态持久化

```go
store, _ := graph.NewBoltStateStore("")          // ~/.opcodex/state.db
engine.StateStore = store
engine.RunID = "my_run_001"
```

---

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `GEMINI_API_KEY` | Gemini API 密钥 | (gemini 模式必填) |
| `GEMINI_MODEL` | Gemini 模型名 | `gemini-2.0-flash` |

## 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--mode` | LLM 后端：`mock` 或 `gemini` | `mock` |
| `--resume` | 从 checkpoint 恢复的 Run ID | (空) |

---

## 开发状态

| 阶段 | 内容 | 状态 |
|------|------|------|
| Stage 1 | 对话记忆 + Gemini API 联调 
| Stage 2 | 沙盒加固 + 技能扩展 
| Stage 3 | 错误自愈 + 人工确认 + 持久化 
| Stage 4 | 并发设备调度 (Project Taiyi) 

详见 → [ROADMAP.md](ROADMAP.md)

---


