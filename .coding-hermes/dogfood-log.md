# Off-by-One — Dogfood Log

Chronological record of dogfood field-test runs (real-use value checks, not test runs).

## 2026-08-10 — 🟡 PROMISING-BUT-ROUGH (field-test run #1)

- **Verdict:** 🟡 PROMISING-BUT-ROUGH — the core loop genuinely works (submit → queue → sandbox solve → verified answer → discover), answer quality is high, and operator ergonomics (WARN on missing solver, readonly 403s, error messages, persistence) are solid. Blockers are the read-only catalog discover gap (P1) and recurring docs drift.
- **Promise statement:** "An agent can submit a problem to the pre-solve lab, get it solved during idle cycles by Pi Agent in a bwrap sandbox, then discover the pre-verified answer via POST /api/v1/problems/discover (or browse the flat-file corpus in data/)." — **held up for the core loop; fell apart for read-only catalog deployments** (discover is POST-only and 403s under --readonly).
- **Time-to-first-success:** ~6 min (first successful documented workflow: POST discover on a real class → found:true with a full verified answer). Submit → in_progress/solver_running observed live in ~15 min (probe sub_0202bf; completion pending at write time — solves run up to 30m).
- **Top 3 findings:**
  1. **OB-GAP-020 (P1):** read-only mode blocks the ONLY discovery endpoint — a public catalog cannot serve the agent-discovery value prop; docs promise discovery works in readonly.
  2. **OB-GAP-021 (P2):** README corpus counts stale a 3rd time (812/948 vs live 874/1012, INDEX.md 864/1002).
  3. **OB-GAP-022 (P2):** README's own example class `go-nil-pointer-deref` 404s on discover — first documented call fails.
- **What worked (evidence):** submit 200 queued; queue poll shows pending→in_progress→solver_running; discover returns deep verified answers (raft, helios migrations, godot checksum — all with root cause + fix + test evidence); error paths all correct (400 missing class / 400 bad cadence / 501 export unconfigured / 404 bogus id / 404 unknown route); scratch instance: WARN when no solver (OB-GAP-005 live), data survives restart (sub_68c0a0 persisted), readonly 403s with clear messages, /ws/chat disabled in readonly; OpenAPI paths match README (15 routes).
- **Friction count:** 7 (see docs/dogfood/2026-08-10-integration.md for details).
- **Artifacts left:** 6 board tasks (OB-GAP-020..025), docs/dogfood/2026-08-10-integration.md, docs/dogfood/diagnostics.md, skills/off-by-one-usage/SKILL.md, this log. Commit: see git log.
- **Foreman:** not woken (cooldown 7200s < 14400 threshold; Enabled=true). Tasks will be picked up on the normal ~2h cadence.
