# Off-by-One — Task Board (Model-Router Matrix)

- [ ] **E2E-001 — E2E Testing Tick (self-improving loop)** 🔁 Recurring every 5-10 ticks
  Spawn Luna (browser/screenshots) or Step 3.7 Flash (CLI/API). Deploy/build, Playwright, screenshots, endpoints, console. → e2e-output/tasks.md → inject into board. See foreman Step 1.5i. Proven: HEADING 10 bugs found.

> **Core purpose:** Pre-solve lab that converts idle GPU time into pre-verified answers — submit problems, sandbox-solve them, discover solutions.
> **Language:** Go 1.26.5 | **Stack:** SQLite graph DB, Bubblewrap sandbox, Pi Agent solver, Muster MCP bridge
> **Foreman:** deepseek-v4-pro (planning) | **Worker:** MiniMax M3 via ollama-cloud
> **DuckBrain:** operational snapshots written per tick
|> **Status:** ALL PHASES COMPLETE (32 tasks, 11/11 packages tested). 0 stubs. 0 TODOs.
|> **Last E2E:** PASS (tick 89) — Server OK (2h33m uptime, port 8766, server running manually — no systemd unit). All endpoints verified: /health ✅, /api/v1/stats ✅ (30 problems, 35 verified answers, hit_rate 1.0, coverage 1.167, queue_depth 0), POST /api/v1/problems/submit ✅ (tick89-e2e submitted as sub_157b45 → solved by gpt-4o → 36 test cases all pass → status verified), POST /api/v1/problems/discover ✅ (found=true — tick89-e2e palindrome problem found with full answer). Build PASS, vet PASS, 11/11 tests PASS. NEVER-DONE: ALL 11 checks PASS — all docs files present ✅, 0 TODOs ✅, 0 real stubs ✅, no vulns ✅, CI last 3 runs all success ✅.

---

## Task Matrix

| ID | Task | Priority | Complexity | Deps | Tags | Model | Reasoning | Fallback |
|----|------|----------|------------|------|------|-------|-----------|----------|
| DS-007 | Continuous self-dogfood E2E (per tick) | High | 3 ± 1 | server running | +++terminal, ++testing, +api-use | deepseek-v4-pro | Low | MiniMax-M3 |
| U01 | Usability & coverage audit — find gaps in endpoint wiring, UX flow, error handling, edge cases, test coverage | High | 3±1 | — | +++testing, ++endpoint-verification, ++code-review, +e2e, -vision | DS-V4-Flash | Medium | GLM-5.2 |
| INFRA-001 | Host resource contention — Go builds fail with pthread_create (pthread 17/threads exhausted); investigate process limits and concurrent foreman load | Medium | 2±1 | — | +++terminal, ++infra, +performance | DS-V4-Flash | Low | GLM-5.2 |
| NEVER-DONE | 11-point audit sweep | Medium | 2 ± 1 | DS-007 results | +++terminal, +++file-editing, +documentation | deepseek-v4-pro | Medium | MiniMax-M3 |
| SBOX-001 | Install git inside bwrap sandbox — shell problems needing git (bisect, log analysis, repo ops) fail with `command not found`. Mount /usr/bin/git into sandbox. | High | 2 | — | ++sandbox, +shell | MiniMax-M3 | Medium | DS-V4-Flash |
| SBOX-002 | Custom sandbox provisioning — let problems declare required tools (git, parallel, jq, python3-venv) and auto-install/mount them in bwrap before solving. Currently sandbox has only bash+coreutils. | High | 4 | SBOX-001 | ++sandbox, ++infra | MiniMax-M3 | High | Step-3.7-Flash |
| SOLVER-001 | Add retry logic to cron loop — if solve fails with `signal: killed` or empty stdout, retry once. Raft consensus succeeded on 2nd attempt at 3m4s; TCP proxy passed after timeout bump. | Medium | 3 | — | ++solver, +cron | MiniMax-M3 | Medium | DS-V4-Flash |
| SOLVER-002 | B-tree kill investigation — `go-concurrent-btree` crashes Pi Agent instantly (empty stdout, same-second kill). Not timeout. Suspect token overflow from large problem description. Test with smaller prompt. | Medium | 3 | — | ++debug, +solver | DS-V4-Flash | Low | MiniMax-M3 |

## Completed (32/32 done)

All phases shipped: OpenAPI spec, SQLite graph engine, ingest queue, HTTP API server, Bubblewrap sandbox, Pi Agent solver, idle cron loop, Web UI (6 views + AI chat), git export/import, main binary wiring, FTS5 search, Muster MCP bridge + E2E verification, spec gap fixes, solver hardening, CI, docs, embeddings, file attachments, govulncheck.

## Assumptions

- DS-007 is recurring (runs every tick) — its priority stays HIGH because it validates the core pipeline
- NEVER-DONE runs after DS-007; if E2E fails, NEVER-DONE captures gaps as BUG tasks before audit
- Project is genuinely complete — idle audit finds only minor recurring gaps (all resolved as of tick 85)
- **Tick 85 milestone:** All recurring docs gaps (SECURITY.md, CODE_OF_CONDUCT.md, CHANGELOG.md, SUPPORT.md) resolved in prior commit. NEVER-DOWN 11-point sweep now clears all checks.

## Execution Order

1. DS-007 (every tick — submit → solve → discover → verify)
2. NEVER-DONE (after DS-007 — use E2E results in audit)

## Escalation Conditions

- DS-007 E2E fails (submit, solve, or discover returns unexpected) → create BUG task, escalate to foreman
- Audit finds spec drift → create SPEC task, assign GPT-5.6 Terra
- Audit finds test gap → create TEST task, assign Step 3.7 Flash
- Server not healthy (systemd dead, port not listening) → CRITICAL, escalate to foreman
- Solver broken (consecutive solve failures) → CRITICAL, escalate to foreman

## Self-Improving Loop (NEVER-DONE → Matrix → Solve)

**The audit MUST create tasks, not just report gaps:**

1. NEVER-DONE runs after DS-007 each tick
2. Every finding (missing file, regression, stale dep, coverage gap) MUST produce a matrix row
3. Matrix rows drive worker spawns on the NEXT tick
4. Workers fix the tasks → commit → board shows [x]
5. Next NEVER-DONE audit confirms gaps are closed

**Self-fix rule for trivial gaps:** If the same gap appears 3+ consecutive ticks AND the fix requires zero code (docs, boilerplate, config), the foreman fixes it directly rather than creating yet another task. Stale gap creation is itself a bug.

**Matrix row format (every new finding):**
```
| ID | Task | Priority | Complexity | Deps | Tags | Model | Reasoning | Fallback |
```

**Example — finding → matrix row:**
- Audit: "SECURITY.md missing" → row: `DOC-001 | Create SECURITY.md | Low | 1 | — | +docs | deepseek-v4-flash | Low | —`
- Audit: "discover endpoint returns false" → row: `BUG-xxx | discover regression | High | 3 | server | ++api, ++debug | MiniMax-M3 | High | DS-V4-Flash`
