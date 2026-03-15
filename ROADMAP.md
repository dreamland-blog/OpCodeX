# OpCodeX Development Roadmap

> This document tracks the evolution of OpCodeX from a basic engine scaffold to a
> production-grade autonomous execution framework.

---

##  Stage 1 — Memory Awakening & Real API Integration `COMPLETE`

- [x] Conversation memory (`MessageEntry` + `AddMessage`/`GetMessages`)
- [x] Real Gemini adapter (multi-turn + function calling REST API)
- [x] First real-environment ignition: Gemini → shell → observe → report

##  Stage 2 — Sandbox Hardening & Skill Expansion `COMPLETE`

- [x] Command blacklist (11 patterns), 64KB output cap
- [x] `ReadTextFileSkill`, `WriteTextFileSkill`, `HTTPRequestSkill`
- [x] Docker sandbox executor

##  Stage 3 — Advanced Graph Control `COMPLETE`

- [x] Error edges + auto-retry (up to 3 retries)
- [x] `HumanConfirmNode` (stdin y/n gate)
- [x] State persistence via bbolt (`--resume` flag)

## Stage 4 — Device Fleet Scheduling (Project Taiyi) `COMPLETE`

- [x] Device abstraction (Local/ADB/SSH)
- [x] Fleet scheduler (goroutine-per-device with semaphore limiting)
- [x] Fleet demo (3 concurrent devices, 3.0x speedup)

---

## Future

- [ ] WebSocket real-time monitoring dashboard
- [ ] Plugin system for custom skill packages
- [ ] Device health monitoring + auto-reconnect
- [ ] Multi-LLM routing (Gemini + Claude + local models)
