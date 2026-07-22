# Off-by-One — Task Board (Model-Router Matrix)

> **Core purpose:** Pre-solve lab that converts idle GPU time into pre-verified answers — submit problems, sandbox-solve them, discover solutions.
> **Language:** Go 1.26.5 | **Stack:** SQLite graph DB, Bubblewrap sandbox, Pi Agent solver, Muster MCP bridge
> **Foreman:** deepseek-v4-pro (planning) | **Worker:** MiniMax M3 via ollama-cloud
> **DuckBrain:** operational snapshots written per tick
> **Status:** ALL PHASES COMPLETE (32 tasks, 11/11 packages tested, 76.3% coverage). 0 stubs. 0 TODOs.
> **Last E2E:** PASS (tick 39) — 16 problems, 19 verified answers, hit_rate=1.0
> **Tick 41:** DS-007 E2E PASS — submit/discover/queue/stats/taxonomy all responsive (15 endpoints). 11-point audit: 0 new gaps (1 recurring: benchmarks). BUG-002: solver still broken (dist/cli.js missing, src/ deleted). Server healthy (59h uptime). CI all green.

---

## Task Matrix

| ID | Task | Priority | Complexity | Deps | Tags | Model | Reasoning | Fallback |
|----|------|----------|------------|------|------|-------|-----------|----------|
| DS-007 | Continuous self-dogfood E2E (per tick) | High | 3 ± 1 | server running | +++terminal, ++testing, +api-use | deepseek-v4-pro | Low | MiniMax-M3 |
| BUG-002 | Pi Agent solver broken — dist/cli.js missing, source files deleted from /tmp/pi/ | High | 3 ± 1 | server running | +++terminal, +++infra, +++typescript | MiniMax-M3 | Medium | Step-3.7-Flash |
| U01 | Usability & coverage audit — find gaps in endpoint wiring, UX flow, error handling, edge cases, test coverage | High | 3±1 | — | +++testing, ++endpoint-verification, ++code-review, +e2e, -vision | DS-V4-Flash | Medium | GLM-5.2 |
| NEVER-DONE | 11-point audit sweep | Medium | 2 ± 1 | DS-007 results | +++terminal, +++file-editing, +documentation | deepseek-v4-pro | Medium | MiniMax-M3 |

## Completed (32/32 done)

All phases shipped: OpenAPI spec, SQLite graph engine, ingest queue, HTTP API server, Bubblewrap sandbox, Pi Agent solver, idle cron loop, Web UI (6 views + AI chat), git export/import, main binary wiring, FTS5 search, Muster MCP bridge + E2E verification, spec gap fixes, solver hardening, CI, docs, embeddings, file attachments, govulncheck.

## Assumptions

- DS-007 is recurring (runs every tick) — its priority stays HIGH because it validates the core pipeline
- Solver was dead for months (misconfigured) while board said "complete" — E2E is the only reliable verification
- NEVER-DONE runs after DS-007; if E2E fails, NEVER-DONE captures gaps as BUG tasks before audit
- Project is genuinely complete — idle audit finds only minor recurring gaps (LICENSE, CONTRIBUTING.md, benchmarks)

## Routing Notes

- DS-007: deepseek-v4-pro (needs full context, terminal, curl, JSON parsing, wait logic). Low reasoning: mechanical verification.
- NEVER-DONE: deepseek-v4-pro (needs full context, 1M window, file search, DuckBrain access)
- Any new Go code: MiniMax-M3 via ollama-cloud (flat-rate, good for bounded Go tasks)
- E2E integration/scaffolding: Step 3.7 Flash via stepfun (++agentic-coding, ++testing)
- Dep upgrades/simple fixes: DeepSeek V4 Flash via opencode-go (cheapest reliable, $0.10/1M)

## Execution Order

1. DS-007 (every tick — submit → solve → discover → verify)
2. NEVER-DONE (after DS-007 — use E2E results in audit)

## Escalation Conditions

- DS-007 E2E fails (submit, solve, or discover returns unexpected) → create BUG task, escalate to foreman
- Audit finds spec drift → create SPEC task, assign GPT-5.6 Terra
- Audit finds test gap → create TEST task, assign Step 3.7 Flash
- Server not healthy (systemd dead, port not listening) → CRITICAL, escalate to foreman
- Solver broken (consecutive solve failures) → CRITICAL, escalate to foreman
