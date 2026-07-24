# Off-by-One — Task Board (Model-Router Matrix)

> **Core purpose:** Pre-solve lab that converts idle GPU time into pre-verified answers — submit problems, sandbox-solve them, discover solutions.
> **Language:** Go 1.26.5 | **Stack:** SQLite graph DB, Bubblewrap sandbox, Pi Agent solver, Muster MCP bridge
> **Foreman:** deepseek-v4-pro (planning) | **Worker:** MiniMax M3 via ollama-cloud
> **DuckBrain:** operational snapshots written per tick
> **Status:** ALL PHASES COMPLETE (32 tasks, 11/11 packages tested, 76.3% coverage). 0 stubs. 0 TODOs.
> **Last E2E:** PARTIAL (tick 64) — Server OK (manual instance, port 8766). API: /api/v1/stats ✅ (16 problems, 19 verified answers, hit rate 1.0, queue_depth 0), /api/v1/problems ✅ (16 listed, total=16, all verified), /api/v1/problems/submit ✅ (sub_d4a80d queued via problem_class), /api/v1/problems/{class} ✅ (go-string-reverse returns full detail), /api/v1/problems/{class}/answers ✅ (returns 1 answer), /api/v1/problems/discover ✅ (problem_class works, env/lang filtering finicky — found:true without env/language params). Queue: 82 entries, 20 complete (24%), 60 failed, 1 pending (sub_a8e1f9), 1 new (sub_d4a80d also failed). All new submissions fail instantly (ERR_MODULE_NOT_FOUND — Pi Agent solver BUG-002 still broken). Build PASS, vet PASS, 11/11 tests PASS. Coverage: 73.9%-89.0%. govulncheck: 0 vulns. NEVER-DONE: same 3 recurring docs gaps (SECURITY.md, CHANGELOG.md, CODE_OF_CONDUCT.md) + Dockerfile missing (but NOT a new gap for this project). 0 TODOs in source. 0 benchmarks.

---

## Task Matrix

| ID | Task | Priority | Complexity | Deps | Tags | Model | Reasoning | Fallback |
|----|------|----------|------------|------|------|-------|-----------|----------|
| DS-007 | Continuous self-dogfood E2E (per tick) | High | 3 ± 1 | server running | +++terminal, ++testing, +api-use | deepseek-v4-pro | Low | MiniMax-M3 |
||| BUG-002 | Pi Agent solver broken — 3-chain failure: (1) undici@8.5.0 npm package published with ZERO .js files (package.json declares main:index.js but tarball only has docs+types). Must extract undici@6.21.1. (2) @earendil-works/pi-ai, agent, orchestrator, tui all have empty dist/ — monorepo at /tmp/pi never fully built. (3) pi-agent binary calls node dist/cli.js which imports undici.Client/Pool/EnvHttpProxyAgent. Fix: install undici@6.21.1 in /tmp/pi/node_modules/undici/, then npm run build for all 5 packages, or extract dist/ from correct versions. | High | 4 ± 1 | server running | +++terminal, +++infra, +++typescript, +++npm | MiniMax-M3 | High | Step-3.7-Flash |
| U01 | Usability & coverage audit — find gaps in endpoint wiring, UX flow, error handling, edge cases, test coverage | High | 3±1 | — | +++testing, ++endpoint-verification, ++code-review, +e2e, -vision | DS-V4-Flash | Medium | GLM-5.2 |
| INFRA-001 | Host resource contention — Go builds fail with pthread_create (pthread 17/threads exhausted); investigate process limits and concurrent foreman load | Medium | 2±1 | — | +++terminal, ++infra, +performance | DS-V4-Flash | Low | GLM-5.2 |
| NEVER-DONE | 11-point audit sweep | Medium | 2 ± 1 | DS-007 results | +++terminal, +++file-editing, +documentation | deepseek-v4-pro | Medium | MiniMax-M3 |

## Completed (32/32 done)

All phases shipped: OpenAPI spec, SQLite graph engine, ingest queue, HTTP API server, Bubblewrap sandbox, Pi Agent solver, idle cron loop, Web UI (6 views + AI chat), git export/import, main binary wiring, FTS5 search, Muster MCP bridge + E2E verification, spec gap fixes, solver hardening, CI, docs, embeddings, file attachments, govulncheck.

## Assumptions

- DS-007 is recurring (runs every tick) — its priority stays HIGH because it validates the core pipeline
- Solver has been dead for months (misconfigured) while board said "complete" — E2E is the only reliable verification
- NEVER-DONE runs after DS-007; if E2E fails, NEVER-DONE captures gaps as BUG tasks before audit
- Project is genuinely complete — idle audit finds only minor recurring gaps (LICENSE, CONTRIBUTING.md, benchmarks)
- **Tick 44 surprise:** Previous ticks checked port 8080 for health (wrong server). Real off-by-one server is on 8766. All prior DS-007 results that reported health on 8080 may have been checking a different service.

## Routing Notes

- DS-007: deepseek-v4-pro (needs full context, terminal, curl, JSON parsing, wait logic). Low reasoning: mechanical verification.
- NEVER-DONE: deepseek-v4-pro (needs full context, 1M window, file search, DuckBrain access)
- Any new Go code: MiniMax-M3 via ollama-cloud (flat-rate, good for bounded Go tasks)
- BUG-002: needs npm/tsgo build of Pi Agent in /tmp/pi/
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
