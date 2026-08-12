# Off-by-One — Diagnostics Trail

How the system is actually built, the errors encountered along the way (this run's
AND the project's own history), and the right way to do things. This is the record
that answers "does it work and why" without re-running the world.

---

## 1. How it is built

- **Go monorepo**, single binary `cmd/off-by-one`. SQLite via modernc.org/sqlite
  (pure-Go, embedded — no cgo). DB file defaults to `./off-by-one.db`
  (`OFF_BY_ONE_DB` / `--db`).
- **REST API** (`internal/api`, port 8766 by default) — 15 routes (see
  `/openapi.json`, which is embedded and served; path list matches README exactly).
- **Ingest queue** (`internal/ingest`): SQLite-backed priority queue; priority =
  cadence weight (pre-phase 1 / end-of-day 2 / post-debug 3) + recurrence × 0.5.
  Dedup on `(class, env, lang, version)` tuple → 409 with `existing_solutions`.
- **Sandbox** (`internal/sandbox`): bubblewrap (`bwrap`) namespaces; per-solve ro
  mounts of `/tmp/pi` (the pi-agent install) and `required_tools` (SBOX-002).
  `--skip-sandbox` for dev. **Env delivery was moved from a `/usr/bin/env KEY=VAL`
  argv shim to `--setenv`/envp in OB-GAP-015 (2026-08-10)** — API keys no longer
  appear in `ps`.
- **Solver** (`internal/solver`): spawns `pi-agent` inside the sandbox with a
  `problem.json`, parses stdout for the fix + evidence. Solver reads
  `DEEPSEEK_API_KEY` from env. `OFF_BY_ONE_SOLVE_TIMEOUT` default 30m (wired in
  OB-GAP-008). If bwrap/pi-agent are missing → **WARN at startup +
  `solver_available:false` in stats** (OB-GAP-005) and the cron loop does not run.
- **Cron loop** (`internal/cron`): wakes every `OFF_BY_ONE_CRON_INTERVAL` (5m),
  only dequeues when loadavg(1) < threshold (idle detection) and a solver exists.
- **Graph** (`internal/graph`): problem-class tree + FTS5 search + BFS related
  edges. Answers store solution, evidence, signatures (model, test result).
- **Web UI** (`internal/web` + `web/`): embedded SPA (go:embed), tabs Home/Search/
  Submit/Explore/Chat; WebSocket chat at `/ws/chat` (disabled in readonly).
- **Export/Import** (`internal/export|import`): git-repo distribution; config-gated
  (`-export-dir`/`-import-dir`) → 501 when unconfigured (NOT stubs — handlers are
  implemented; they were documented as functional before the gate existed, fixed in
  OB1-GAP-003).
- **Readonly mode** (`--readonly` / `OFF_BY_ONE_READONLY`): public catalog — all
  POSTs 403, chat disabled, GETs work. POST discover is allowed (read-only
  catalog op) since OB-GAP-020 (tick 285).
- **Board** (`.coding-hermes/`): JSONL-canonical since 2026-08-07
  (JSONL-NORM-001) — `board/tasks.jsonl` is the live board; `tasks.md` is a frozen
  legacy log; `board/events.jsonl` is the audit trail; `board.db` is derived and
  untracked. Foreman (deepseek-v4-flash) ticks every ~2h (cooldown 7200s).

## 2. Errors hit this run (and the right way)

| Error | Cause | Right way |
|---|---|---|
| `404 not_found` on discover `go-nil-pointer-deref` | Example class never existed in the corpus | Query classes that exist (`q=` search or INDEX.md) — tracked as OB-GAP-022 |
| `{"found":false}` for a class WITH a verified answer | env/lang/version are exact filters; queried with non-matching env | Discover class-only first; refine only if you need tuple-specific answers — OB-GAP-023 |
| `403 read_only` on discover from a catalog instance | Readonly guard blocked all POSTs | FIXED (OB-GAP-020, tick 285) — discover returns 200 in readonly mode; submit/export/import stay 403 |
| `501 not_configured` on export/import | Feature is config-gated; lab runs without `-export-dir` | Expected; enable via env/flag if you need git distribution |
| `426 WebSocket protocol violation` on `/ws/chat` | Plain curl without upgrade headers | Use a WS client; or accept that the endpoint is WS-only |
| Submission stuck `pending` on scratch instance | No solver (no keys / `--skip-sandbox`) → cron loop not started (WARN at boot) | Check `/api/v1/stats` → `solver_available` before relying on the queue |
| Solves failing at exactly 300s | Known bwrap-cap fleet pattern (`signal: killed` at 5m) | Not a regression; tuning candidate `DefaultBwrapTimeout` (needs restart, out of cron scope) |
| Detail endpoint `"status":""` | Detail handler didn't populate status (list does) | FIXED (OB-GAP-024, tick 285) — detail response populates status |

## 3. The project's own error history (from the board, ticks 254-284)

- **2026-08-04 pi-agent wipe #1:** 26,448 files in `/tmp/pi` truncated to 0 bytes
  (disk 98% full). Signature: `ERR_INVALID_PACKAGE_CONFIG /tmp/pi/package.json`.
  Rebuild recipe: clone pi-monorepo → `npm install --ignore-scripts` → `npm run
  build` (~50s), verified with `pi-agent --help` (`--problem-file required`). No
  server restart needed — bwrap ro-mounts per solve.
- **2026-08-06 pi-agent wipe #2:** `/tmp/pi` fully deleted between 19:49-20:06Z;
  84 consecutive `bwrap: Can't find source path /tmp/pi` instant-fails. Root cause
  of the deletion still unknown (watch item — no_agent watchdog recommended).
- **SBOX-002 deployment saga (Aug 2-7):** feature landed in code but the live
  binary was 100h+ old because restarts needed sudo (blocked in cron). Deployed
  2026-08-07 07:33Z. Lesson: schedule privileged restarts explicitly.
- **OB-GAP-006/015 (security):** API key first in `--api-key` argv, then in the
  `/usr/bin/env` shim argv — both visible in `ps`. Fixed: env-var auth only +
  `RunWithEnv`/`--setenv`. Regression tests assert the key value never enters argv.
- **Queue hygiene:** restart-killed `in_progress` rows inflated `queue_depth`
  (7→14); swept via direct SQL UPDATE (tick 271). Old-binary bwrap hangs also left
  strands.
- **Docs drift loop (recurring):** README corpus counts have gone stale 3×
  (437/535 → 575/690 → 812/948), env-var names drifted once (`_PATH` suffix),
  example classes drifted once. Each fixed manually; OB-GAP-021 asks for an
  automated stamp at export time.

## 4. The right way (cheat sheet)

1. **Consume answers:** `POST /discover` with class only → best answer. Verify
   freshness via `GET /api/v1/stats` (hit_rate, coverage).
2. **Submit:** always include `cadence` (`post-debug` = highest); expect 409 dedup
   if the tuple is already solved — then just discover.
3. **Operate:** check the startup WARN + `solver_available`; keep `/tmp/pi` intact
   (rebuild recipe above); restarts are safe (SQLite WAL, data persists).
4. **Test locally:** `go build ./cmd/off-by-one && ./off-by-one --skip-sandbox
   --db /tmp/x.db --port 8877` — full API without bwrap/keys.
5. **Read-only catalog deployments:** fine for humans browsing; agent discovery
   works since OB-GAP-020 (discover is 200 in readonly mode).
6. **Commit hygiene:** GitReins guard blocks on secrets/build/tests; docs-only
   commits pass. Never commit API keys.
