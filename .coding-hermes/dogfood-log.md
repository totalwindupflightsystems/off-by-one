# Off-by-One — Dogfood Log

Chronological record of dogfood field-test runs (real-use value checks, not test runs).

## 2026-08-20 — ✅ SHIPPABLE (field-test run #2)

- **Verdict:** ✅ SHIPPABLE — the full agent-user loop worked end-to-end on the
  live lab: submitted a REAL problem (python-pip-interpreter-mismatch-pep668),
  watched the idle-cycle solver pick it up in bwrap, answer cached + discoverable.
  Every P1/P2 gap from run #1 (OB-GAP-020/021/022/024/025/034) re-verified FIXED
  live; docs/API polish is now at the "finds real bugs only at P2/P3" level.
- **Promise statement:** "An agent can submit a problem to the pre-solve lab,
  get it solved during idle cycles by Pi Agent in a bwrap sandbox, then discover
  the pre-verified answer via POST /api/v1/problems/discover (or browse the
  flat-file corpus in data/)." — **held up end-to-end**, including readonly
  catalog discovery (previously broken).
- **Time-to-first-success:** ~4 min (first documented workflow: discover on
  so-nil-pointer-deref → found:true with full verified answer). Submit → solver
  pickup: 18 min behind live fleet traffic (04:06:40Z → 04:24:43Z); solve → 
  complete in 69s; discover → found:true (answer id 1347). Full loop closed.
- **Top 3 findings (new):**
  1. **OB-GAP-046 (P2):** empty collections serialize as `null` not `[]` —
     zero-match search `{"problems":null}` and `related:null`; my client crashed
     iterating None (the lab's consumers are agents — this is the class of bug
     that bites them).
  2. **OB-GAP-050 (P2):** FTS search rows (`?q=`) show `status:"pending"`,
     empty description/created_at for EVERY class while list/detail show
     `verified` — the search path agents use to find answers is wrong about
     status on every row.
  3. **OB-GAP-047/048 (P3):** `stats.avg_solve_time` always `""` (submit's
     estimated_time is a fixed "5m0s"); `seed` exits 0 with an EMPTY db when the
     corpus dir is unreadable — silent empty catalog. Plus OB-GAP-049 (P3):
     stale knowledge artifacts advertised fixed bugs as active (skill refreshed
     in this commit).
- **What worked (evidence):** discover returns deep verified answers; submit
  → queued → picked up by cron loop (live `ps`: bwrap → pi-agent solve, argv
  carries NO API keys — security fixes hold); error paths clean (400/404/501/503
  all with messages); readonly instance: discover 200, submit 403, chat 403,
  stats readonly:true; seed honors OFF_BY_ONE_DB (1095/1177 loaded); corpus
  answers.jsonl = 1177 lines all verified, zero test junk; web UI SPA serves
  deep-linkable problem pages; /openapi.json live.
- **Friction count:** 5 (null-vs-[] crash, FTS search wrong status, seed
  silent-empty, avg_solve_time dead field, per-class queue window confusing the
  API view — all filed or documented).
- **Artifacts left:** 5 board tasks (OB-GAP-046..050), docs/dogfood/2026-08-20-integration.md,
  diagnostics.md §5 appended, skills/off-by-one-usage/SKILL.md refreshed
  (pitfalls updated to current state), this log. Commit: see git log.
- **Foreman:** not woken (cooldown 21600s ≥ 14400 threshold — see notes below).
  Fleet.toml pins cooldown at 21600 (operator pin per OB-GAP-039-era reasoning:
  "never PUT below operator pin"); the 4 new tasks will be picked up on the
  normal ~6h cadence.

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
