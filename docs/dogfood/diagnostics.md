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
  **Presence is NOT solve health (OB-GAP-078):** `/tmp/pi/.git`, `package.json`,
  a non-empty `node_modules/.bin` and an executable wrapper can all be green
  while the solve path is dead — on 2026-09-18 four
  `node_modules/@earendil-works` workspace symlinks had vanished and every solve
  failed in <=1s with `ERR_MODULE_NOT_FOUND: Cannot find package
  '@earendil-works/pi-tui' imported from
  /tmp/pi/packages/coding-agent/dist/main.js`. `scripts/pi-agent-watchdog.sh`
  resolves every solve-path workspace package from `$PI_DIR` with node and NAMES
  the missing one(s) (`make pi-agent-watchdog-selftest`); a vanished link is
  invisible to the directory listing the old probe walked. **A timed-out probe
  is not a wipe (OB-GAP-079):** at loadavg 159 on 2026-09-18 the per-package
  probe hit its `PI_AGENT_RESOLVE_TIMEOUT` budget (node rc=124) on packages
  whose symlinks were on disk, and the verdict read `solve path BROKEN …
  (node TIMEOUT)` — the hollow-wipe alert for an UNVERIFIED outcome, which
  trains its readers to ignore the alert that matters. The resolution stage now
  reports three states: ok (silent, exit 0), unresolved (BROKEN alert + re-link
  recipe, exit 1), and unverifiable (a WARN naming the unverified packages and
  the budget in force, no wipe verdict and no recipe, exit 3) — re-run when load
  subsides.
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
4. **Test locally:** `make build && ./off-by-one --skip-sandbox
   --db /tmp/x.db --port 8877` — full API without bwrap/keys.
   Before you trust that binary, run `make check-binary-fresh`: `make build`
   stamps `./off-by-one` from `git describe`, and the check refuses an artifact
   the running server would serve with pre-fix behavior. The three failures it
   prints, verbatim:

   - `ERROR: ./off-by-one was built from a dirty tree (stamp: <stamp>) — the working tree had uncommitted changes when it was compiled; commit or stash them, then run 'make build'`
   - `ERROR: ./off-by-one carries no version stamp (<stamp>) — it was not built by 'make build'; run 'make build'`
   - `ERROR: ./off-by-one is stale — source changed since it was built; run 'make build'`

   Remedy for all three: commit or stash the working tree, then `make build`.
   `check-binary-fresh` only validates the on-disk artifact — a fresh
   `./off-by-one` does NOT prove the live process restarted. For the running
   service, run `make check-deploy` (section 8).
5. **Read-only catalog deployments:** fine for humans browsing; agent discovery
   works since OB-GAP-020 (discover is 200 in readonly mode).
6. **Commit hygiene:** GitReins guard blocks on secrets/build/tests; docs-only
   commits pass. Never commit API keys.

---

## 5. Second field-test run — 2026-08-19/20 (dogfood #2) — verdict SHIPPABLE

Full record: `docs/dogfood/2026-08-20-integration.md`. What was re-verified and
what was newly found.

### Verified fixed since run #1 (all live, not from test output)

- **Readonly discover (OB-GAP-020):** scratch `--readonly` instance on :8891 →
  `POST /discover` returns 200 `found:true`; submit → 403 with a clear message;
  chat → 403 "AI agent disabled in read-only catalog mode"; stats expose
  `readonly:true` + `solver_available:false`. Seeded instance honored
  `OFF_BY_ONE_DB` (OB-GAP-039/044) — 1095 classes / 1177 answers loaded.
- **Detail status (OB-GAP-024):** `GET /problems/so-nil-pointer-deref` →
  `"status":"verified"`.
- **Corpus counts (OB-GAP-021):** README has zero literal counts; `data/COUNTS.md`
  auto-stamped at export (1095/1177 at 2026-08-20 04:00 UTC).
- **Corpus hygiene (OB-GAP-025):** zero test/canary/dogfood patterns in
  `data/INDEX.md`; `answers.jsonl` = 1177 lines, all parse, all `verified`,
  1095 unique classes.
- **Solver-absent submit (OB-GAP-034):** submit on a solver-less instance →
  503 `solver_unavailable` (readonly probe; code path per board).
- **Security (OB-GAP-006/015):** live `ps` during a solve shows the bwrap/pi
  argv with NO `sk-` keys and no `--api-key`/`/usr/bin/env` shim.
- **Error paths:** 404s carry messages; bad cadence → 400 `invalid_request`;
  export/import → 501 `not_configured` with guidance; `/ws/chat` → 426 without
  upgrade headers (WS-only, by design).

### New findings (tasks OB-GAP-046..050, filed 2026-08-20)

| ID | Severity | What | Evidence |
|---|---|---|---|
| OB-GAP-046 | P2 | Empty collections serialize as `null`, not `[]`: `GET /api/v1/problems?q=<no-match>` → `{"problems":null,"total":0}`; `GET /problems/<class>/related` → `{"related":null}` | My client crashed (`TypeError: 'NoneType' object is not iterable`) on the first no-match search; raw responses captured |
| OB-GAP-047 | P3 | `stats.avg_solve_time` always `""` (spec: string) on a lab with 1346 completed solves; submit's `estimated_time` is a fixed "5m0s" that clears to `""` at position 0 | Live stats + queue detail for `sub_e6bf1e` |
| OB-GAP-048 | P3 | `seed` exits 0 with an EMPTY 4KB DB when the corpus dir is unreadable (wrong CWD or `-dir`); only an info log line | `OFF_BY_ONE_DB=/tmp/x.db ./off-by-one seed` from /tmp → exit 0, empty db |
| OB-GAP-049 | P3 | Knowledge artifacts drift after fixes: `skills/off-by-one-usage/SKILL.md` pitfalls #1/#3/#4/#6 advertised OB-GAP-020/024/021/025 as active for days after they shipped | Board shows all complete; skill updated in this commit; tick-gate proposed |
| OB-GAP-050 | P2 | FTS search rows carry wrong metadata for EVERY class: `?q=<term>` → `status:"pending"`, `description:""`, `created_at:""`, while plain list + detail return `verified` + populated fields for the same ids (incl. so-nil-pointer-deref, verified since 07-24) | Probed 5 classes across the corpus on 2026-08-20 |

### How the live solve pipeline looks from the outside (right way to watch)

- Live server runs with `-cron-interval 60s`; a submit lands in SQLite, the cron
  loop dequeues when load is idle, `bwrap` spawns `pi-agent solve` with
  `problem.json`, output files (`solution.md`, `evidence.md`, `signatures.json`)
  land in the per-submission sandbox dir under `/tmp/off-by-one-sandbox-<id>`.
- One solve at a time (sequential), solves take 1–30m; the 300s bwrap cap is
  tunable via `OB1_BWRAP_TIMEOUT`.
- Queue API window is per-class — use the DB (or `/api/v1/queue/<id>`) for the
  true picture; don't read the 100-window as global state.

---

## 6. Sync-focused field-test run — 2026-08-30 (dogfood #3, target off-by-one-sync)

### What was tested

The scheduler project `off-by-one-sync` is a **DuckBrain focused-sync job**,
not a code repo: its workdir is intentionally empty and its product is the
namespace sync itself. This run executed the sync protocol end-to-end (auth →
preflight test-write → scan → verify) and probed the lab's core loop live.

### How the sync is built (right way)

- **Auth:** DuckBrain HTTP (`localhost:3000`) requires `X-API-Key`. Scoped
  tokens live in `~/.duckbrain/auth.json` (`apiKeys[]`); the off-by-one
  namespace token is named `off-by-one-foreman` (grant: ns `off-by-one`).
  The MCP server (`duckbrain` in `~/.hermes/config.yaml`) is **disabled** —
  sync agents use HTTP, not MCP.
- **Preflight:** test-write `/sync/write-test-YYYY-MM-DD` (domain `config`),
  verify by recall. 201 + recall hit = healthy. `/api/health` is NOT a valid
  liveness path (404) — use the keys API or a test-write.
- **Scan:** scheduler API (`127.0.0.1:9090`, nested `project` key), live
  server (`:8766/health`, `/api/v1/stats`), git (`origin/master..HEAD`),
  CI (`gh run list`), feeder proof keys.
- **Write:** batches ≤4, domains `person|event|concept|message|config|raw_note`,
  `/sync/last-run` LAST.

### Errors hit this run (and the right way)

| Error / friction | Right way |
|---|---|
| `GET /api/health` → 404 | Don't use it; liveness = keys API or test-write |
| `/api/keys` tree shape varies (object vs list) | Walker must handle both |
| Feeder proof keys stalled 25 days (Aug 5→30) while feeder runs 4×/day | Feeder prompt (jobs.json `3ac3112f61b5`) has NO DuckBrain write step; OB-GAP-063 filed |
| OB-GAP-060 fix committed Aug 30 00:24, live server runs Aug 24 binary (uptime 87h+) | Deploy-lag check = binary mtime vs server uptime; OB-GAP-062 filed |

### Live lab state (2026-08-30 ~11:00 local)

- 1377 problems / 1554 answers, all "verified", queue 0, hit_rate 1.0,
  coverage 1.13, solver available, uptime 87h45m.
- Discover probe on `go-linear-scan-register-allocator` → `found:true` with
  full solution/evidence/signatures — the core value prop holds.
- CI green (5/5 recent runs incl. pages deploy); 0 unpushed commits.
- Foreman tick 11:40Z idle (NO_CHANGES, commit 9e31f4a); board 60 complete /
  7 pending (incl. 2 new from this run).

### The right way to answer "does off-by-one work?"

1. `curl localhost:8766/api/v1/stats` — problems/answers/verified/queue.
2. `make check-deploy` — proves the RUNNING service serves HEAD's code
   (section 8; OB-GAP-077). Replaces the old `stat -c %y off-by-one` vs
   `/health` uptime heuristic, which was a lag proxy, not a proof.
3. `git log origin/master..HEAD` — unpushed work.
4. `gh run list -R totalwindupflightsystems/off-by-one` — CI.
5. Discover a real class — the loop's end-to-end proof.

## 7. Export/import-focused field-test run — 2026-09-07 evening (dogfood #4) — verdict PROMISING-BUT-ROUGH

**Why this run exists:** every prior dogfood run tested export/import only via the
`501 not_configured` default path. The corpus-sharing workflow — the reason the
export/import engines exist — had never been executed. This run spun a fully scratch
instance (fresh seeded DB, dedicated port 18901, per-leg temp dirs) and drove both legs.

**How this part is built (one paragraph):** both engines follow the same shape —
`Config{RepoURL, Branch, LocalDir, SubtreePrefix, GitPath}` + a `prepareClone` that
either fetches/pulls an existing clone in `LocalDir` or makes a fresh one, then walks
the `pre-solve-answers/` subtree. Export serializes graph answers → flat files →
commit → push; Import parses flat files → `importAnswer` upserts into the graph with
content-diff dedup. The HTTP handlers in `internal/api/handlers.go` translate JSON
requests into engine calls.

### Errors hit this run, and what each means

| Error | Meaning | Right way |
|---|---|---|
| `export_failed: export item class=0 answer=43: get problem class: graph: not found` (HTTP 500, EVERY request) | Handler bug, not your request: `handlers.go:788` builds `ExportItem{AnswerID}` with `ClassID` always 0; the engine then can't resolve class 0. Engine is fine — its tests pass `ClassID: pc.ID`. | None at API level as of `ad63507`. Use the graph → `scripts/export-answers.py` → git path, or fix DF-OFF-BY-ONE-6 (handler must resolve the answer's class before building items). |
| `import` → `200 {"skipped":1}` for a **nonexistent** `source_repo` | Stale-clone reuse: `prepareClone` (import `git.go:196`) fetches/pulls the PREVIOUS import's origin; new URL never validated. First-import-to-dead-URL correctly 500s; the bug bites long-lived multi-import servers. | One source repo per `ImportLocalDir`; wipe the dir between sources until DF-OFF-BY-ONE-7 lands. Treat `skipped:1` on a first import as a red flag, not a dedup. |
| `server error: listen tcp :PORT: bind: address already in use` (process dies after healthy-looking startup logs) | Port taken by another service. On this host 8766 = fleet daemon, 18767 = `crier`. | `ss -tlnp | grep <port>` BEFORE starting; pick a free port (`OFF_BY_ONE_PORT`). |
| `q=raft&env=linux` → 0 results | Not a search bug: list filters match STORED `env` values; corpus is overwhelmingly `env:"docker"` while README's submit example uses `"environment": "linux"`. | Filter with values that exist in the store (check `data/answers/*.json`), or read DF-OFF-BY-ONE-9 for the docs fix. |

### Layer-lesson (why "all tests green" hid a P0)

The export engine is covered by tests that always pass `ClassID: pc.ID` (they test the
engine through its real contract). The HTTP handler is covered by tests that only assert
400/501. Nobody tested handler + engine together on the success path — exactly the
L1-syntax → L2-runs → L3-works-for-a-user gap. The fix for the process, and the reason
this file exists: drive the DOCUMENTED workflow end-to-end on a scratch instance, as a
user would, and record what actually came back.

### What worked (the import leg is genuinely good)

Hand-authored community answer repo in the documented flat-file format → `POST
/api/v1/import` → `added:1` → `discover` → `found:true` with full solution/evidence/
signatures in ~90s start-to-finish. Re-import dedups via content diff (`skipped:1`).
The parse-upsert pipeline and the flat-file contract are solid and contributor-friendly.

## 8. Deploy check after any code commit (OB-GAP-077)

**The problem it closes:** on 2026-09-19 the live server (systemd unit
`off-by-one.service`, MainPID 2774764) was running stamp `19a9a9c` while HEAD
`85bbbe6` already contained code commit `4a3ff39`
(internal/api/handlers.go — the OB-GAP-080 filter fix). The stale window
spanned two board ticks: `make check-binary-fresh` alone cannot catch it,
because it validates the on-disk `./off-by-one`, not the process that has been
serving traffic since before the commit landed.

**What `make check-deploy` proves** — one command, fail-fast, every failure
exits non-zero and names the remedy:

1. the on-disk artifact is fresh (chains `make check-binary-fresh`);
2. the unit is running (`systemctl show off-by-one -p MainPID --value`, empty/0
   = FAIL "unit off-by-one not running");
3. `/proc/<MainPID>/exe` IS this repo's artifact (mismatch = FAIL naming both
   paths);
4. the RUNNING process's `--version` stamp resolves to a commit whose code
   paths (`cmd/ internal/ web/ sql/ pkg/ go.mod go.sum Makefile`) match HEAD.
   Data-only drift (corpus syncs) is tolerated — the same stamp-resolution
   semantics as `check-binary-fresh`, mirrored in
   `scripts/check-deploy --resolve-stamp <stamp>` (the unit-testable seam; it
   touches neither systemctl nor /proc).

**The one command, run after ANY code commit on master:**

```
cd ~/off-by-one && make check-deploy
```

**Failure modes and remedies**

| Leg | Failure output | Remedy |
|---|---|---|
| 1 | `make check-binary-fresh rejected ./off-by-one` (stale / dirty-tree / no stamp) | commit or stash the tree, `make build`, relaunch the service, re-run |
| 2 | `unit off-by-one not running (MainPID empty/0)` | start the service, re-run |
| 3 | `running process (pid N) executes '<path>', not the repo artifact` | `make build` and relaunch so `/proc/<MainPID>/exe` points at the repo binary |
| 4 | `running service (pid N, stamp S) does not serve HEAD's code` | `cd ~/off-by-one && make build && kill N` — systemd `Restart=always` relaunches the service; re-run `make check-deploy` |

Self-test (fresh / divergence / garbage / live verdict-presence):
`bash scripts/check-deploy-test.sh`.

### Tick close-out enforcement (OB-GAP-085) — `make gate-deploy`

**The problem it closes:** OB-GAP-077 built the probe; NOTHING called it. On
2026-09-20 the live unit (MainPID 1665810, started 2026-09-19 12:39:44) was
serving the `a8d8b67` artifact while HEAD carried fifteen-plus changed code files
(`internal/api/handlers.go`, `internal/ingest`, …) — ~13h of undeployed drift in
which CI (3/3 runs), GitReins Tier 1 (4/4) and the Tier 2 judge all reported
green. The judge is not a deploy gate: it builds its OWN binary from HEAD instead
of probing the service. At tick start `make check-binary-fresh` exited 2 and that
was the only signal — and it was somebody else's to read.

**The enforced step: `make gate-deploy`** (Makefile target → `scripts/gate-deploy`),
run at tick close-out after ANY commit touching `cmd/ internal/ web/ sql/ pkg/`,
`go.mod` or `go.sum`. Its exit status IS the probe's exit status — an untripped
leg is never reported as a pass.

It deliberately does more than chain the probe, because a close-out gate must
refuse to lie about a deployment it does not own:

- **Worktree / foreign unit → SKIP, loudly.** Run from a git worktree (this
  repo's parallel workers) or on a host without the unit, the artifact you built
  is not what the service runs; the gate prints the checkout that DOES own the
  unit and exits 0 without claiming a verdict. (`git rev-parse --absolute-git-dir`
  vs `--git-common-dir` distinguishes a worktree from the ordinary checkout.)
- **Unit bound elsewhere → FAIL.** The unit's `WorkingDirectory` (or the
  directory of its `ExecStart` binary) must be THIS checkout; otherwise the gate
  is about to measure someone else's deployment. Note the unit FILE location is
  irrelevant — this host's unit lives at
  `/etc/systemd/system/off-by-one.service` while running
  `/home/kara/off-by-one/off-by-one`, so `FragmentPath` can never be the test;
  `WorkingDirectory`/`ExecStart` are the authoritative evidence. Override the
  expected root with `OB1_UNIT_SOURCE` for split-checkout deployments.
- **Missing/unexecutable probe → FAIL.** A broken gate is never a pass.
- **Not a git checkout → FAIL.** Without git the artifact cannot be tied to HEAD.

**Close-out checklist (after any code commit):**

1. commit or stash board/gitreins state — a dirty tree stamps the artifact
   `<rev>-dirty`, which `check-binary-fresh` refuses by design;
2. `make build`;
3. `kill <MainPID>` — systemd `Restart=always` relaunches the service;
4. `make gate-deploy` — must PASS before the tick is closed out.

**Red/green proof, captured hermetically** (throwaway `--shared` clone +
PATH-stubbed `systemctl` + a throwaway instance on port 18998; the live service
was never touched — its MainPID was unchanged before and after):

```
# RED — artifact stamped dc69717 (the last code commit's parent), HEAD e1ba282
$ make gate-deploy
./scripts/gate-deploy
make[1]: Entering directory '<scratch>/repo'
ERROR: ./off-by-one is stale — source changed since it was built; run 'make build'
make[1]: *** [Makefile:43: check-binary-fresh] Error 1
make[1]: Leaving directory '<scratch>/repo'
check-deploy: FAIL — leg 1: make check-binary-fresh rejected ./off-by-one (see ERROR above)
remedy: commit or stash the working tree if needed, then run 'make build', relaunch the service, and re-run make check-deploy

gate-deploy: FAIL — the running off-by-one service does not serve HEAD's code (check-deploy exit 1; see its output above).
gate-deploy: remedy (run in <scratch>/repo):
    1. commit or stash board/gitreins state — a dirty tree stamps the artifact '<rev>-dirty', which check-binary-fresh refuses by design
    2. make build
    3. kill 1005021          # systemd Restart=always relaunches the service
    4. make gate-deploy        # must pass before this tick is closed out
make: *** [Makefile:108: gate-deploy] Error 1      # MAKE_GATE_DEPLOY_EXIT=2

# GREEN — `make build` from HEAD, then the gate against a throwaway instance
$ make gate-deploy
./scripts/gate-deploy
make[1]: Entering directory '<scratch>/repo'
./off-by-one is up to date with source (version stamp changed, but code paths are unchanged)
make[1]: Leaving directory '<scratch>/repo'
PASS: stamp 'fb45b6e' resolves to fb45b6e — code paths match HEAD (data-only drift tolerated)
check-deploy: PASS — running service (pid 1008727, stamp fb45b6e) serves HEAD's code
gate-deploy: PASS — artifact and running service both serve HEAD's code; deploy rotation satisfied for tick close-out.
# MAKE_GATE_DEPLOY_EXIT=0
```

The dirty-tree half is real, not theoretical: build the artifact while the tree is
dirty and `check-binary-fresh` rejects it —

```
ERROR: ./off-by-one was built from a dirty tree (stamp: 8b69a47-dirty) — the working tree had uncommitted changes when it was compiled; commit or stash them, then run 'make build'
```

Self-test, extended for the gate (hermetic; scratch clones, stub systemd unit,
throwaway instance — never the live service): `make check-deploy-test` /
`bash scripts/check-deploy-test.sh` — cases GATE-WORKTREE (SKIP), GATE-NOTAREPO
(FAIL), GATE-NOWORK (missing probe, FAIL), GATE-RED (stale artifact → FAIL with
remedy), GATE-NOSTAMP (bare `go build` → FAIL), GATE-FOREIGN (unit bound
elsewhere → FAIL), GATE-GREEN (rebuilt from HEAD → PASS).


### Transcript #1 — stale live server, captured pre-deploy (2026-09-19)

At capture time: HEAD `85bbbe6`, last code commit `4a3ff39`
(internal/api/handlers.go), running server pid 2774764 with stamp `19a9a9c`.
The check fails on leg 1 (`./off-by-one` predates the code commit) — exactly
the divergence this task exists to catch. Host paths shortened to `~`.

```
$ make check-deploy
./scripts/check-deploy
make[1]: Entering directory '~/off-by-one'
ERROR: ./off-by-one is stale — source changed since it was built; run 'make build'
make[1]: *** [Makefile:43: check-binary-fresh] Error 1
make[1]: Leaving directory '~/off-by-one'
check-deploy: FAIL — leg 1: make check-binary-fresh rejected ./off-by-one (see ERROR above)
remedy: commit or stash the working tree if needed, then run 'make build', relaunch the service, and re-run make check-deploy
make: *** [Makefile:93: check-deploy] Error 1
exit code: 2
```

### Transcript #2 — after deploy

Captured live 2026-09-19 immediately after `make build` + relaunch (systemd
Restart=always re-executed the unit as pid 1665810):

```
./scripts/check-deploy
make[1]: Entering directory '~/off-by-one'
./off-by-one is up to date with source (version stamp changed, but code paths are unchanged)
make[1]: Leaving directory '~/off-by-one'
PASS: stamp 'a8d8b67' resolves to a8d8b67 — code paths match HEAD (data-only drift tolerated)
check-deploy: PASS — running service (pid 1665810, stamp a8d8b67) serves HEAD's code
exit code: 0
```

## §8 — 2026-09-19 dogfood run (HEAD c5b245c)
Build: `go build ./cmd/off-by-one` on go1.25.x, seed 14s, serve :18903.
Errors hit and what they mean:
- `bind: address already in use` — a leftover serve from a prior leg held the
  port; the sandbox/cron legs of other runs also leave bwrap processes. Check
  `pgrep -af off-by-one` before picking a port.
- Export 400 `unknown field "limit"` / `answer_ids must be non-empty` — the
  request schema is `target_repo` + `answer_ids[]` + `branch` +
  `commit_message`; there is no server-side limit field.
- Export chain of 500s on the way to success: (1) target dir not a git repo →
  `git clone ... exit 128`; (2) repo without origin → `git remote get-url
  origin: exit 2`; (3) origin != target_repo → "RepoURL does not match the
  existing clone"; (4) branch missing on remote → `git checkout main` +
  `origin/main` fallback both fail. The engine is strict-by-design: bare
  remote + clone with matching origin + existing branch = green in one shot.
- Queue list vs per-id asymmetry (DF-OFF-BY-ONE-10): pending rows show in
  /queue/{id} but /queue returns null entries — reproducible by submitting
  twice and listing immediately.
Bunker install leg (las-bunker-03, agent adeac425, destroyed after):
- git clone over ssh fails on a fresh agent (no credential) → tar-stream
  fallback (documented in the dogfood skill; no permissions widened).
- `make build` → `go: command not found`; fixed by go1.25.1 tarball to
  ~/toolchain (no sudo). Documented path then green end-to-end:
  INSTALL_SECONDS=114, seed OK, /health 200, discover found:true.

## §9 — 2026-09-22 dogfood run (HEAD 859081c) — the distribution layer under the microscope

Angle: prior runs exercised the HTTP API; this run consumed the project the
way an outside user does — the flat-file corpus (no server) and the public
catalog ob1.it.com.

How the distribution pipeline actually works (and where it silently ends):
- `export-answers.py` reads SQLite, keeps `status='verified'` answers, drops
  probe/canary classes via `EXCLUDED_CLASS_PATTERNS` (regex on the class
  title), writes `data/answers.jsonl`, `data/answers/<id>-<slug>.json`,
  INDEX.md, COUNTS.md (auto-stamped — the fix for the old count-drift class
  of bugs), and a generated `data/README.md` consumer guide.
- `ob1-distribute.sh` (cron 4×/day, canonical copy in repo, deployed copy at
  `~/.hermes/scripts/` — re-deploy after edits, they were md5-identical on
  09-22) PART 1: regenerate `data/`, commit, push. PART 2:
  `publish-catalog.sh` ships the **binary + DB snapshot** to the catalog box.
- The **static HTML tree (`site/`) is NOT in either half**: only
  `sync-answers.sh` regenerates it (via `generate-static-site.py`), and that
  script has no active caller since the 08-27 cron merge — its own header
  says it absorbed `ob1-sync-answers.sh`, whose site-refresh half was dropped.
  Net effect: ob1.it.com froze at the Aug 18 generation (1062/1144 advertised
  vs 2051/2141 real). Any future fix must re-wire `sync-answers.sh` (or fold
  `generate-static-site.py` into PART 1) AND redeploy the cron copy.

Numbers that reconcile (recompute, don't trust): 2142 live classes − 76
regex-excluded probes − 15 failed-only classes (checked in SQLite read-only)
= 2051 corpus classes; 2307 stats-verified − 166 failed-solve answers = 2141
corpus answers. The corpus is a strict SUBSET of the stats-verified set —
README's "one further exclusion" phrasing inverts the relationship.

Bunker install leg #2 (agent 92f4b025, destroyed after, list clean):
- Public HTTPS clone works on a fresh user (no credentials needed) — 15s.
  Run #7's private-clone limitation is gone for consumers (README Quick Start
  now documents the access requirement for contributors, and the corpus is
  the no-clone path anyway).
- README verbatim install green: Go tarball (~/toolchain) → `make build` rc=0
  → seed 34s (2051/2141/8510 edges) → serve :18766 → discover found:true 10ms.
- Two cross-agent `/tmp` collisions cost real time: (1) a stale `/tmp/go.tgz`
  from the Sep-19 agent (different uid, sticky /tmp) turned the README recipe
  into `curl: (23)` — the recipe should use `$HOME`; (2) fixed log paths
  (`/tmp/build.log`, `/tmp/ob1_site.log`) are shared with concurrent agents
  on the box — a sibling's Rust build overwrote `build.log` mid-read. Lesson:
  on multi-agent hosts, every scratch path needs the agent id in it.

Right way to answer "is the public catalog fresh?" in 10 seconds:
`curl -s https://ob1.it.com/ | grep -o '<title>[^<]*'` and compare the number
against `curl -s http://localhost:8766/api/v1/stats`. A title older than one
sync cycle = the site leg is orphaned again.

## §10 — 2026-09-23 dogfood run (HEAD 239cbdb) — the solve pipeline, the chat, and the release binary

**Why this run exists:** runs #1-8 hit the API, the corpus, and the distribution
layer — never the thing the product is named for: submitting a real problem and
getting it SOLVED. This run turned the crank on the full pipeline and on the two
remaining untouched surfaces (WS chat, release-binary consumption).

**How the solve pipeline actually works, learned by driving it:**
1. Submit → `ingest.Queue.Submit` dedups on `(class, env, lang, version)` (409
   with `existing_solutions` when the class already has a verified answer).
2. The idle cron (`--cron-interval`, default 5m; `--load-threshold -1` disables
   the idle gate for testing) dequeues in `priority DESC, created_at ASC`.
3. The solver wraps pi-agent in bwrap; `--skip-sandbox` is dev-only. Real solves
   measured 24s and 58s with correct, executed-verification solutions.
4. On completion the answer lands in the graph (status verified) and discover/
   browse serve it — EXCEPT when the title trips a placeholder regex (§ the P1).

**The P1 lesson (DF-OFF-BY-ONE-15): probe filters must be anchored.**
`internal/graph/placeholder.go`'s regexes exist to keep the lab's own
self-test/canary probes out of the public surface. Unanchored (`(?i)dogfood`),
they quarantine real users' classes: submit 200, solve complete, browse
visible, discover 404, queue list empty (SQL variant `NotPlaceholderClassSQL`
hides the row from three list paths while the by-id path returns it). The
failure is silent and looks like data loss. The right way: anchor probe
patterns to the actual probe prefixes (off-by-one-self-test-*, docs-canary-*,
dogfood-field-test-*) and add a submit→solve→discover round-trip test with a
probe-word in the title. This is the third dogfood run to find a "green
everywhere, wrong for one whole user class" defect — the pattern repeats
because serve-surface filters are never round-trip tested against the
ingest surface that feeds them.

**WS chat mechanics (measured, probe scripts in /tmp/dogfood-ob-2026-09-23):**
server sends pings every 15s and expects pongs — a naive client that skips
pongs is dropped at the first ping. The answer arrives as ONE frame after the
full sandbox+pi-agent cycle (74s measured); there are no interim progress
frames (DF-OFF-BY-ONE-16). A browser user sees a frozen chat; the fix is a
"thinking" frame on receipt plus stage updates.

**Release-binary consumer path (first proof):** v0.1.1 ships raw binaries +
SHA256SUMS (not tarballs — the download name in a README recipe of the form
`off-by-one-v0.1.1-linux-amd64.tar.gz` would 404; nothing in the README points
at the release artifacts at all, which is why no doc update is contradicted).
Fresh Debian: curl 2 assets + verify 6s; seed needs the corpus, which the
release does NOT bundle — clone `--depth 1` the public repo and pass
`-dir <repo>/data` (the seed path search order is CWD/data, ./data, /data —
see the 2026-09-07 CWD finding). Total 58s to a serving lab with discover green.

**Right way to test the solver without touching production:** dedicated port,
dedicated `-db`, `--load-threshold -1` (idle gate off), `--cron-interval 20s`
(fast pickup), `--solve-timeout 300s`. Never point a scratch instance at the
live off-by-one.db — the seed is idempotent but solves write real rows.

## §11 — 2026-09-24 dogfood run (HEAD 474a86e) — the Muster consumer side, first exercise

Prior runs proved the lab from the REST/curl side and the operator side (seed, solve, export,
catalog). This run took the README's core-loop step 1 literally — "Agents push problems via
Muster API/MCP/CLI" — and tried to become that agent. The wire protocol held; the distribution
did not.

**What is real (proven, not claimed):** `pkg/api/openapi.yaml` is a genuine machine contract.
A fresh MusterFlow instance consumed it with zero project-specific code: one `connect` command
produced 15 typed CLI verbs AND a working HTTP MCP endpoint; MCP `initialize` / `tools/list` /
`tools/call discoverSolution` all returned live data; submissions through both surfaces landed
in the same queue the REST API serves. This is the strongest integration evidence the repo
has produced — the spec is not decorative, it interoperates with an independent implementation.

**What is fiction (DF-OFF-BY-ONE-17):** the "muster" binary the Quick Start's own ecosystem
points at is `github.com/wojons/muster` — private module, no tag, no binary — so the bridge's
consumer half cannot be installed by anyone, ever, from the docs. `connect-muster.sh` papers
over this by printing "Muster binary not found", then `exit 0` with "=== Integration
Complete ===". The right way: publish a muster release binary (the project already has a
release-binary lane for off-by-one itself — same recipe), or document MusterFlow as the
supported consumer and drop the dead `go install` line from the script output.

**Port drift trap (DF-OFF-BY-ONE-18):** the script's step-4 health check probes :8767 while
`muster-config.yaml` transports stdio and pins no HTTP port — two hardcoded ports that agree
with no config file. On any port-collision host (the README's own documented scenario) the
script would nohup-start a *second* off-by-one against the foreign daemon's DB. Rule: scripts
that manage daemons must derive ports from the same config they pass to the daemon.

**Chain economics (the value question):** through the full generated chain, discover costs
21ms where direct REST costs 1.2ms — ~18x overhead, entirely irrelevant (both are noise). The
chain's value is not latency, it is that an agent that speaks only MCP gets the lab's entire
answer graph for one `connect`. That value was demonstrated this run with real calls.

**Install leg this run:** clone 3s → make build 71s → seed 33s (2138/2229/9020) → health 200
→ discover found:true 10ms, all on a bare Debian bunker agent with a manual Go tarball. The
Go-toolchain gap (DF-OFF-BY-ONE-11) is now the single remaining install friction across three
consecutive runs; a bootstrap line in the Quick Start (`curl go tarball || apt install golang`)
would close it permanently.

## §12 — 2026-09-25 dogfood run (HEAD 3722faa→273604c) — the public deployment as the consumer endpoint

Every prior run started a local server. This one used what an outside agent actually uses:
ob1.it.com through the Cloudflare Worker's bot/browser split, public REST reads, and the
SPA's deep-linkable problem pages (the `7501497` feature). It also live-verified the two
fixes from 09-24 (DF-12 site regen, DF-18 connect-muster ports).

**The bot/browser split is real and correct on both sides.** Bot UA → static site from
raw.githubusercontent master: fresh numbers (2189 == git tree == sitemap, generated the day
before), robots/sitemap served, extension-less class URLs mapped to `.html`. Browser UA →
SPA shell + proxied readonly API. The trap this run avoided: a 200 on the SPA shell proves
nothing — headless Chrome `--dump-dom` confirmed the answer body and its syntax-highlighted
code blocks actually render client-side. **Rule: for SPAs, verify the DOM, never the status
code.** The one unreproducible wrinkle: a batch of raw.githubusercontent requests 404'd once
and never again (transient CDN behavior); the worker does not retry a failed origin fetch —
a missed static page renders as a 404 to a crawler until the next bot request. Not filed
(one observation, no repro); worth remembering if ob1.it.com ever shows phantom 404s in
search console.

**The one defect (DF-OFF-BY-ONE-21, P2):** `GET /api/v1/taxonomy` silently caps at 1000
classes — `handlers.go:681` hardcodes `ListProblemClassesWithCounts(ctx, 1000, 0)` with no
pagination, no total, no doc note. A consumer walking the public catalog sees 1000/2189
classes (1011/2281 answers) with nothing signalling truncation; reproduced identically on a
local instance, so it is the handler, not the deployment. The endpoint is also 8MB / ~1.3s
public (one ListAnswers query per class). **Lesson: any `limit` hardcoded in a handler is a
silent data cliff the moment the dataset outgrows it — the site advertises 2189 while its
own discovery endpoint returns 1000, and nothing disagreed loudly.**

**Live-verified fixes:** DF-12 — site/ regenerated in-repo 09-24 17:48, pushed, and the
public worker serves it; the frozen-since-08-18 catalog is fixed in production. DF-18 —
connect-muster.sh now derives PORT from SERVER_URL (explicit `:<port>` wins), refuses to
spawn a daemon for remote URLs, exposes MUSTER_HEALTH_URL; `--check-only` green. Rule from
both: a dogfood fix is not done when it merges — it is done when a later run proves it on
the live surface it affects.

**Install leg (release-binary path, verbatim README):** clone 20.7s → download+sha256 8.6s
→ seed 36s (2189/2281/9280) → serve → health 200 → discover found:true 10ms ×3 → destroy
exit 0. Zero toolchain, zero sudo, no docs deviation — the DF-11-era "manual Go tarball"
friction is gone from the documented path because the release-binary recipe now leads.
Bunker-host quirk: `/tmp` is shared across agents on that box (a stale sibling file caused
EACCES on a log redirect) — use `$HOME` for scratch on bunker agents.
