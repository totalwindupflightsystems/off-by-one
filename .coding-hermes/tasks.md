# Off-by-One — Model Router Task Matrix

> **Core purpose:** Pre-solve lab that converts idle GPU time into pre-verified answers — submit problems, sandbox-solve them, discover solutions.
> **Language:** Go 1.26.5 | **Stack:** SQLite graph DB, Bubblewrap sandbox, Pi Agent solver, Muster MCP bridge
> **Status:** ALL PHASES COMPLETE (33 tasks, 11/11 packages tested). 0 stubs, 0 TODOs.

## Active Tasks

| ID | Task | Pri | Cpx | Deps | Tags | Model | Lvl | Fallback |
|----|------|-----|-----|------|------|-------|-----|----------|
| DS-007 | Continuous self-dogfood E2E (per tick) | High | 3 | server running | ++terminal, ++testing, +api-use | DeepSeek V4 Pro | Low | MiniMax-M3 — **tick 91 ✅** |
| BUG-002 | ✅ RESOLVED — Solver now works end-to-end via bwrap + Pi Agent wrapper | — | — | — | — | — | — | — |
| SBOX-002 | Custom sandbox provisioning — let problems declare required tools (git, parallel, jq, python3-venv) and auto-install them in bwrap | High | 4 | — | ++sandbox, ++infra | MiniMax-M3 | High | Step 3.7 Flash |
| SOLVER-001 | Add retry logic to cron loop — if solve fails with signal: killed or empty stdout, retry once | Medium | 3 | — | ++solver, +cron | MiniMax-M3 | Medium | DeepSeek V4 Flash |
| SOLVER-002 | B-tree kill investigation — go-concurrent-btree crashes Pi Agent instantly (empty stdout). Suspect token overflow. | Medium | 3 | — | ++debug, +solver | DeepSeek V4 Flash | Low | MiniMax-M3 |
| UI-001 | LaTeX + Markdown answer rendering — spectral theorem answers contain raw LaTeX. Add MathJax/KaTeX + full markdown renderer | High | 3 | — | ++ui, ++javascript, +css | MiniMax-M3 | Medium | DeepSeek V4 Flash |
| PERF-001 | DB load optimization — taxonomy page loads all 51+ problems in single request. Add pagination, lazy loading, compression | Medium | 3 | — | ++ui, ++sql, +performance | DeepSeek V4 Flash | Medium | MiniMax-M3 |
| OSS-001 | Open source launch readiness — CI badge, version badge, Go report card, pkg.go.dev link, goreleaser | Medium | 2 | — | ++docs, +ci, +github | DeepSeek V4 Flash | Low | MiniMax-M3 |
| CONFIG-001 | Custom Pi Agent config support — let users bring their own Pi config (~/.pi/credentials.json) or pass --pi-config flag | High | 4 | — | ++config, ++docs, +solver | MiniMax-M3 | High | DeepSeek V4 Flash |
| E2E-001 | Browser-based UI verification — spawn Luna with browser tools to load web UI, screenshot every view, check JS errors | High | 4 | UI-001 | ++browser, ++screenshots, ++verification | GPT-5.6 Luna | High | Step 3.7 Flash |
| NEVER-DONE | 11-point audit sweep | Medium | 2 | DS-007 results | ++terminal, ++file-editing, +documentation | DeepSeek V4 Pro | Medium | MiniMax-M3 |
| INFRA-001 | Host resource contention — Go builds fail with pthread_create (pids.max=512). Investigate process limits. | Medium | 2 | — | ++terminal, ++infra, +performance | DeepSeek V4 Flash | Low | GLM-5.2 |

## Completed

All phases shipped: OpenAPI spec, SQLite graph engine, ingest queue, HTTP API server, Bubblewrap sandbox, Pi Agent solver, idle cron loop, Web UI (6 views + AI chat), git export/import, FTS5 search, Muster MCP bridge + E2E verification, embeddings, govulncheck, CI, docs.

## Assumptions

- DS-007 is recurring (runs every tick) — priority stays HIGH because it validates core pipeline
- NEVER-DONE runs after DS-007; if E2E fails, NEVER-DONE captures gaps as BUG tasks before audit
- Project is genuinely complete — idle audit finds only minor recurring gaps

## Routing Notes

- **Go implementation (bounded):** MiniMax-M3 primary (flat-rate prepaid)
- **Go debugging/complex:** DeepSeek V4 Pro
- **UI/JavaScript:** MiniMax-M3 for bounded, DeepSeek V4 Flash for mechanical
- **E2E browser testing:** GPT-5.6 Luna (vision + screenshots, $100/mo flat)
- **INFRA tasks:** DeepSeek V4 Flash — investigation only, no code changes expected

## Execution Order

1. DS-007 (every tick — submit → solve → discover → verify)
2. NEVER-DONE (after DS-007 — use E2E results in audit)
3. SBOX-002 → SOLVER-001/SOLVER-002/UI-001/PERF-001 (parallel — independent subsystems)
4. CONFIG-001 (after solver hardening)
5. OSS-001 (final — release readiness)
6. E2E-001 (periodic, after UI-001 for LaTeX verification)

## Escalation Conditions

- DS-007 E2E fails → create BUG task, escalate to foreman
- Audit finds spec drift → create SPEC task, assign GPT-5.6 Terra
- Audit finds test gap → create TEST task, assign Step 3.7 Flash
- Server not healthy → CRITICAL, escalate to foreman
- Solver broken (consecutive failures) → CRITICAL, escalate to foreman
