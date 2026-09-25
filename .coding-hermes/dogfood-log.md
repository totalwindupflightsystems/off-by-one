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

2026-09-04 | PROMISING-BUT-ROUGH | 66s t2fs | friction 8 | 5 findings
2026-09-07 | SHIPPABLE | n/a t2fs | friction 0 | 5 findings
2026-09-07 (evening) | PROMISING-BUT-ROUGH | ~90s t2fs (scratch :18901 up → import 200 added:1) | friction 4 | 4 findings (1 P0)
  Focus: export/import git round trip — never exercised by prior runs (they hit only the 501-unconfigured path).
  Promise: docs/integration.md §Import/Export corpus sharing. Import leg HOLDS end-to-end; export leg FAILS on every request (handler drops ClassID → 500; DF-OFF-BY-ONE-6). Import from bogus source_repo silently 200s via stale-clone reuse (DF-OFF-BY-ONE-7). commit_message accepted-then-ignored + internal-leaking 500 (DF-OFF-BY-ONE-8). integration.md example = the 500ing shape; env-filter vocabulary nuance (DF-OFF-BY-ONE-9).
  What worked: scratch seed 14s → server on 18901 → hand-authored community answer repo → import 200 added:1 → discover found:true instantly → re-import dedup skipped:1. q= search matches answer bodies. Web UI 200. No code fixed (user rule); findings on board.
  SKIPPED-install-bunker: bunker-las-03 offline 16h (ssh timeout ×2 + tailscale state). Install leg unproven this run.
  Artifacts: docs/dogfood/2026-09-07b-integration.md, diagnostics.md §7, tasks.md evening section, board DF-6..9, this log.

2026-09-19 | SHIPPABLE (1 P1 open: DF-OFF-BY-ONE-10 queue-list hides pending) | t2fs ~90s (build+seed+serve+discover) | friction 3 (port collision, export remote preconditions, queue list) | 2 findings + INSTALL row
  Promise: run a pre-solve lab end-to-end — submit, discover, browse/filter, export/import. HELD; OB-GAP-080/081 fixes live-verified; export/import round trip green for the first time (commit 0fdd5a6, re-import updated:1).
  Install: bunker=las-bunker-03 agent=adeac425 install_seconds=114 smoke=ok (after manual Go tarball — DF-OFF-BY-ONE-11); agent destroyed, list empty.
  Artifacts: docs/dogfood/2026-09-19-integration.md, diagnostics.md §8, board DF-OFF-BY-ONE-10/11, this log.

2026-09-22 | SHIPPABLE (1 open P1: DF-OFF-BY-ONE-12 public catalog frozen) | t2fs ~2min corpus path (clone 3s + scan 62ms + fix applied), ~5min fresh install | friction 3 (frozen catalog, docs inversion, /tmp collisions) | 3 findings (12/13/14) + INSTALL row + DF-10 fix live-verified
  Promise under test: no-server corpus + public catalog (surfaces no prior run touched; runs #1-7 hit only the API). Corpus promise HELD as L3: solved the host's own PEP 668 problem from answer 1347 (python-pip-interpreter-mismatch-pep668) by applying the stored fix verbatim — venv on python3.12, clean pip install, zero error. Catalog promise FAILED: ob1.it.com frozen at 2026-08-18 (advertises 1062/1144 vs real 2051/2141); root cause = sync-answers.sh (only site/ regenerator) lost its caller in the 08-27 cron merge. Freshness triangle reconciled to the answer: 2142 live classes − 76 probe regex − 15 failed-only = 2051 corpus classes (no content lost; README's corpus-vs-stats wording inverted → DF-14).
  Install: bunker=las-bunker-03 agent=92f4b025 clone=15s (public HTTPS, fresh-user path) build=rc0 seed=34s (2051/2141/8510) smoke=discover found:true 10ms; agent destroyed, list clean. README instructions green VERBATIM this time (DF-11 fixes hold); /tmp collision frictions filed as DF-13.
  Perf: no row filed — corpus scan 62ms warm, grep 5ms, discover 10ms; nothing a user would feel.
  Artifacts: docs/dogfood/2026-09-22-integration.md, diagnostics.md §9, board DF-OFF-BY-ONE-12/13/14, skills/off-by-one-usage/SKILL.md v1.3.0, this log.

2026-09-23 | SHIPPABLE (1 open P1: DF-OFF-BY-ONE-15 unanchored placeholder regexes) | t2fs ~7min scratch (seed 45s + server + first solve 24s) | friction 2 (discover-404 trap, chat silence) | 2 findings (15/16) + INSTALL row (release-binary leg)
  Angle: surfaces runs #1-8 never touched — solve pipeline (submit→bwrap→pi-agent→store), WS chat, release-binary consumer install.
  Promise: "submit a real problem, get a verified answer; a fresh box consumes the lab without building". HELD for pipeline+install (solves 24s/58s, correct solutions; release-binary install 58s, sha256 ok, discover 9ms); FAILED for probe-word users: discover 404s any class whose title contains 'dogfood'/'canary'/etc. (unanchored regexes, placeholder.go:34 + NotPlaceholderClassSQL in queue list paths).
  Install: bunker=las-bunker-03 agent=8e3d704f download+sha256=6s clone+seed=46s serve+discover=3s smoke=found:true 9ms×3; agent destroyed, list clean. First install leg via RELEASE BINARY (no toolchain needed).
  Perf: no PERF row — discover 9-12ms warm, search 32ms, install 58s, solves 24-58s; only user-noticeable wait is the 74s chat silence (filed as UX DF-16, not perf).
  Artifacts: docs/dogfood/2026-09-23-integration.md, diagnostics.md §10, board DF-OFF-BY-ONE-15/16, skills/off-by-one-usage/SKILL.md v1.4.0, this log.

2026-09-24 | SHIPPABLE (lab; Muster consumer path install-unreal — 2 open P2s: DF-17/18) | t2fs n/a (live instance; scratch-chain discover 21ms warm) | friction 3 (unbuildable muster binary, port hardcoding, truncated table output) | 4 findings (17/18/19/20) + INSTALL row
  Angle (first exercise in 10 runs): the Muster MCP bridge's CONSUMER side — README core loop step 1. A real independent client (MusterFlow) consumed /openapi.json: one connect → 15 typed CLI verbs + an HTTP MCP endpoint; initialize/tools/list/tools/call discoverSolution live; CLI + MCP submissions (sub_ea3d97, sub_b0b4bc) landed in the same queue REST serves; estimated_time now derived (3m29s AvgSolveTime), not the old fixed 30s. The spec interops — strongest integration evidence the repo has.
  What failed: the bridge's own 'muster' binary is unbuildable from docs (private module, no tag) and connect-muster.sh exits 0 'Integration Complete' without it (DF-17); script hardcodes :8766/:8767 disagreeing with all config (DF-18); MusterFlow table truncates answers (DF-19, musterflow-side); q= semantics unstated (DF-20). DF-OFF-BY-ONE-15 fix (6046258) live-verified — probe-word classes discover normally.
  Install: bunker=las-bunker-03 agent=f0557da1 clone=3s build=71s seed=33s (2138/2229/9020) serve+health=200 discover=found:true 10ms; connect-muster.sh dry-run green steps 1-3, step 4 = the DF-17 dead end; agent destroyed, list clean.
  Perf: no PERF row — headline op (discover via full generated chain) 21.3ms ±0.9ms warm, 0.19s cold incl. process start; direct REST 1.2-30ms. Nothing a user would feel.
  Artifacts: docs/dogfood/2026-09-24-integration.md, diagnostics.md §11, board DF-17..20, skills/off-by-one-usage/SKILL.md v1.5.0 (pitfalls 15-18), musterflow board DF-038, this log.

2026-09-25 | SHIPPABLE (1 new P2: DF-OFF-BY-ONE-21 taxonomy caps at 1000/2189) | t2fs n/a (public endpoint; local discover 10ms) | friction 1 (discover 400 doesn't name required field) | 1 finding (21) + INSTALL row (release-binary leg) + DF-12/DF-18 live-verified
  Angle (first exercise in 11 runs): the PUBLIC deployment (ob1.it.com) as the consumer endpoint — Cloudflare Worker bot/browser split, public REST reads, SPA deep-link rendering — plus live-verification of the 09-24 fixes (DF-12 site regen, DF-18 connect-muster ports).
  Promise: "anyone can use the verified answers without running a server". HELD: bot UA gets the fresh static site (2189 == git tree == sitemap; DF-12 live), browser UA passes to the SPA whose deep-linked class pages fully render (headless DOM dump: answer body + highlighted code blocks), public discover found:true 0.51s, q= search exact, /openapi.json 17.4KB. NEW DEFECT: GET /api/v1/taxonomy returns 1000/2189 classes / 1011/2281 answers with no pagination/total/docs — handlers.go:681 hardcodes limit 1000 (reproduced locally). DF-21 filed (P2). Wrinkle not filed (no repro): one transient raw.githubusercontent 404 batch — worker does not retry failed origin fetches.
  Install: bunker=las-bunker-03 agent=ea8d84b3, README release-binary path VERBATIM: clone 20.7s → download+sha256 8.6s (v0.1.1 OK) → seed 36s (2189/2281/9280) → serve health 200 → discover found:true 10ms ×3 → destroy exit 0, list clean. Zero toolchain, zero sudo. Quirk: /tmp shared across that host's agents → use $HOME.
  Perf: no PERF row — public discover 646ms±376 warm (n=10, CF+TLS RTT-dominated), bot class page 902ms±678, local discover 10ms; nothing a user would feel. Taxonomy 8MB/1.3s is a shape problem → part of DF-21.
  Artifacts: docs/dogfood/2026-09-25-integration.md, diagnostics.md §12, board DF-21 (commit 273604c), tasks.md findings section, skills/off-by-one-usage/SKILL.md v1.6.0 (pitfalls 19-21, DF-17/18 status refresh), this log.
