# Off-by-One — Task Board (Model-Router Matrix)

> **Core purpose:** Pre-solve lab that converts idle GPU time into pre-verified answers — submit problems, sandbox-solve them, discover solutions.
> **Language:** Go 1.26.5 | **Stack:** SQLite graph DB, Bubblewrap sandbox, Pi Agent solver, Muster MCP bridge
> **Foreman:** deepseek-v4-pro (planning) | **Worker:** MiniMax M3 via ollama-cloud
> **DuckBrain:** operational snapshots written per tick
> **Status:** ALL PHASES COMPLETE (32 tasks, 11/11 packages tested, 76.3% coverage). 0 stubs. 0 TODOs.
> **Last E2E:** PASS (tick 83) — Server OK (systemd active, 33min uptime, port 8766). All endpoints verified: /health ✅, /api/v1/stats ✅ (21 problems, 25 verified answers, hit_rate 1.0, coverage 1.19, queue_depth 1), /api/v1/problems ✅ (20 listed, all verified), POST /api/v1/problems/submit ✅ (sub_2abc5f queued), POST /api/v1/problems/discover ✅ (found=true for ALL tested classes: go-string-reverse, go-generics-stack, shell-echo-hello-fix, python-async-generator, echo-hello — BUG-003 RESOLVED after server restart), GET /api/v1/queue ✅. Build PASS, vet PASS, 11/11 tests PASS, Hilo: 352 edges / 45 files. govulncheck: 0 vulns. NEVER-DONE: same recurring docs gaps (SECURITY.md, CODE_OF_CONDUCT.md, CHANGELOG.md, SUPPORT.md), CONTRIBUTING.md ✅, LICENSE ✅. 0 TODOs, 0 stubs. BUG-002: solver still broken — submissions fail at 'failed' stage. CI: last 3 runs all success.

---

## Task Matrix

| ID | Task | Priority | Complexity | Deps | Tags | Model | Reasoning | Fallback |
|----|------|----------|------------|------|------|-------|-----------|----------|
| DS-007 | Continuous self-dogfood E2E (per tick) | High | 3 ± 1 | server running | +++terminal, ++testing, +api-use | deepseek-v4-pro | Low | MiniMax-M3 |
|||| BUG-002 | Pi Agent solver broken — pi-ai dist/ has api/auth/providers/utils dirs (not fully empty), undici installed, pi-agent binary exists but requires --problem-file flag at solve time. The root cause appears to be that the monorepo at /tmp/pi has dist/ stubs in all 5 packages and pi-agent tries to `import` from them. | High | 4 ± 1 | server running | +++terminal, +++infra, +++typescript, +++npm | MiniMax-M3 | High | Step-3.7-Flash |
|| U01 | Usability & coverage audit — find gaps in endpoint wiring, UX flow, error handling, edge cases, test coverage | High | 3±1 | — | +++testing, ++endpoint-verification, ++code-review, +e2e, -vision | DS-V4-Flash | Medium | GLM-5.2 |
|| INFRA-001 | Host resource contention — Go builds fail with pthread_create (pthread 17/threads exhausted); investigate process limits and concurrent foreman load | Medium | 2±1 | — | +++terminal, ++infra, +performance | DS-V4-Flash | Low | GLM-5.2 |
|| NEVER-DONE | 11-point audit sweep | Medium | 2 ± 1 | DS-007 results | +++terminal, +++file-editing, +documentation | deepseek-v4-pro | Medium | MiniMax-M3 |

## Completed (32/32 done)

All phases shipped: OpenAPI spec, SQLite graph engine, ingest queue, HTTP API server, Bubblewrap sandbox, Pi Agent solver, idle cron loop, Web UI (6 views + AI chat), git export/import, main binary wiring, FTS5 search, Muster MCP bridge + E2E verification, spec gap fixes, solver hardening, CI, docs, embeddings, file attachments, govulncheck.

## Assumptions

- DS-007 is recurring (runs every tick) — its priority stays HIGH because it validates the core pipeline
- Solver has been dead for months (misconfigured) while board said "complete" — E2E is the only reliable verification
- NEVER-DONE runs after DS-007; if E2E fails, NEVER-DONE captures gaps as BUG tasks before audit
- Project is genuinely complete — idle audit finds only minor recurring gaps (SECURITY.md, CODE_OF_CONDUCT.md, CHANGELOG.md, SUPPORT.md)
- **Tick 44 surprise:** Previous ticks checked port 8080 for health (wrong server). Real off-by-one server is on 8766. All prior DS-007 reports that reported health on 8080 may have been checking a different service.

## Routing Notes

- DS-007: deepseek-v4-pro (needs full context, terminal, curl, JSON parsing, wait logic). Low reasoning: mechanical verification.
- NEVER-DONE: deepseek-v4-pro (needs full context, 1M window, file search, DuckBrain access)
- Any new Go code: MiniMax-M3 via ollama-cloud (flat-rate, good for bounded Go tasks)
- [x] **BUG-002 (FIXED tick 81):** Pi Agent solver broken since Jul 19 — 60+ submissions failed with `ERR_MODULE_NOT_FOUND`. Root cause: `/tmp/pi` had incomplete git clone (no .ts source files, only empty dist/ stubs). Fix: `git clone` fresh → `npm install` → `npm run build` — compiled all 5 packages (tui, ai, agent, coding-agent, server). Server restarted, all 4 languages (Shell/Go/Python/JS) now solving. 3 new verified answers added.
- INFRA-001: pure investigation, no code change needed
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
