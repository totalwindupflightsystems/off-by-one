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

## 2026-08-30 — 🟡 PROMISING-BUT-ROUGH (field-test run #3, target off-by-one-sync)

- **Verdict:** 🟡 PROMISING-BUT-ROUGH — the sync workflow itself works
  end-to-end (auth → preflight test-write → scan → verify → report) and the
  lab's core loop is healthy (discover → found:true with full verified answer),
  but the run surfaced two real gaps: a committed-but-undeployed fix and a
  25-day-stalled feeder proof-key series.
- **Promise statement:** "A sync agent can, every ~6h, verify the off-by-one
  namespace is healthy and record fresh facts (server status, scheduler state,
  git activity, findings) into DuckBrain." — **held up**; the sync ran clean
  and the namespace is actively maintained (last-run 11:11Z today, 4 keys
  written, all verified).
- **Time-to-first-success:** ~8 min (auth discovery → successful test-write →
  first verified recall). Friction count: 3 (auth key location undocumented,
  `/api/health` 404 as liveness probe, `/api/keys` tree shape varies).
- **Top 3 findings:**
  1. **OB-GAP-062 (P1):** OB-GAP-060 fix (ee79fea, Aug 30 00:24) committed but
     never deployed — live :8766 runs the Aug 24 binary (uptime 87h+), stats
     still serve pre-fix verified counts. DuckBrain finding flagged it at
     11:12Z but no board task existed.
  2. **OB-GAP-063 (P2):** feeder proof keys stalled 25 days (last
     /off-by-one/feeder/submissions/2026-08-05) while obo-problem-feeder runs
     4×/day (last 05:06 today, 3 problems queued) — the feeder prompt has no
     DuckBrain write step; the cheat sheet's "ingest-activity proof" is
     missing while ingestion demonstrably continues.
  3. **Docs gap (P2, no task):** the sync cheat sheet never documents where the
     namespace auth key lives (`~/.duckbrain/auth.json`, token
     `off-by-one-foreman`) or that the DuckBrain MCP server is disabled —
     a new sync agent must reverse-engineer auth.
- **What worked (evidence):** test-write 201 + recall verified; namespace sweep
  100 keys (72 project / 15 sync / 12 findings / 1 test); live stats
  1377/1554/queue 0/hit_rate 1.0; discover probe found:true; CI green 5/5;
  0 unpushed; feeder cron alive (jobs.json 3ac3112f61b5, last_status ok).
- **Artifacts left:** 2 board tasks (OB-GAP-062/063, JSONL format),
  docs/dogfood/2026-08-30-integration.md, diagnostics.md §6 appended,
  this log. Commit: see git log.
- **Foreman:** not woken (cooldown 21600s ≥ 14400 threshold; fleet.toml pins
  cooldown at 21600 — operator pin, "never PUT below operator pin" per
  OB-GAP-039-era reasoning). The 2 new tasks will be picked up on the normal
  ~6h cadence.
2026-09-01 | PROMISING-BUT-ROUGH | 42s t2fs | friction 9 | 5 findings

