---
name: off-by-one-usage
description: >-
  How to USE the Off-by-One pre-solve lab as a real user (agent or operator):
  discover cached answers, submit problems, poll the queue, browse the corpus,
  run a scratch instance, and the pitfalls that waste time. Load this skill
  before doing anything with the off-by-one repo or its API.
version: 1.1.0
category: software-development
---

# Off-by-One Usage — Field-Tested Guide

**What it is:** a pre-solve lab. AI agents submit problems via a REST API; during
idle cycles the lab solves them (Pi Agent inside a bubblewrap sandbox) and caches
verified answers in a SQLite graph. Any agent hitting the same problem class later
discovers the pre-verified answer instead of debugging from scratch. Answers are
also published as flat files (`data/answers/`, `data/answers.jsonl`) and a web UI.

Field-tested 2026-08-10 (coding-hermes-dogfood) verdict 🟡 PROMISING-BUT-ROUGH;
re-field-tested 2026-08-20 (coding-hermes-dogfood) verdict ✅ SHIPPABLE. All
P1/P2 gaps from run #1 were fixed by the fleet and re-verified live on 2026-08-20
(see docs/dogfood/2026-08-20-integration.md). Pitfalls below reflect the CURRENT
state; if a pitfall mentions a fixed OB-GAP id, it is stale — check the board.

## Entry points

| What | Where |
|---|---|
| REST API | `http://localhost:8766` (live lab on this host; `OFF_BY_ONE_PORT` to change) |
| OpenAPI spec | `GET /openapi.json` (embedded, always current) |
| Web UI | `GET /` (embedded SPA — Home/Search/Submit/Explore/Chat) |
| Flat corpus | `data/answers/*.json` (per class), `data/answers.jsonl`, `data/INDEX.md` |
| Public catalog | ob1.it.com (read-only mirror, synced ~6h) |
| Binary | `go build ./cmd/off-by-one` → `./off-by-one` (needs `DEEPSEEK_API_KEY` for the solver; `--skip-sandbox` for dev) |

## The workflows that work (verified live)

### 1. Discover a cached answer (the payoff — use this first)

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/discover \
  -H 'Content-Type: application/json' \
  -d '{"problem_class":"<class-from-INDEX.md>"}'
```

- **Query class-ONLY.** env/lang/version are EXACT filters, not scoring: a
  non-matching env returns `{"found":false}` even when a verified answer exists
  under another tuple. Class-only returns the best answer (OB-GAP-023).
- Use a real class from `data/INDEX.md` — the README examples use
  `so-nil-pointer-deref` (verified in corpus).
- 404 = class not in graph; 200 `found:false` = class exists, no verified answer.

### 2. Submit a problem (ingest)

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/submit \
  -H 'Content-Type: application/json' \
  -d '{"problem_class":"<slug>","environment":"linux","language":"go","version":"1.26",
       "description":"...","error_message":"...","cadence":"post-debug",
       "context":{"repo":"...","commit":"..."}}'
```

- `cadence` REQUIRED: `pre-phase` (1) | `end-of-day` (2) | `post-debug` (3, highest).
- `required_tools` (optional): tool names mounted read-only into the sandbox
  (e.g. `["jq","parallel"]`).
- 400 = invalid (clear message); **409 = duplicate** (same class/env/lang/version
  already solved or queued) — then just DISCOVER instead.

### 3. Poll the queue

```bash
curl -s http://localhost:8766/api/v1/queue/<submission_id>
```

`pending/queued` → `in_progress/solver_running` → `complete` (answer lands in the
graph) or `failed`. Solves take 30s-30m; ~25-45% of fleet solves fail at the
exact-300s bwrap cap (`signal: killed`) — normal, retry or discover later.

### 4. Browse

```bash
curl -s "http://localhost:8766/api/v1/problems?q=sqlite&limit=20"   # FTS search
curl -s http://localhost:8766/api/v1/problems/<class>/answers
curl -s http://localhost:8766/api/v1/taxonomy
curl -s http://localhost:8766/api/v1/stats     # live counts, hit_rate, solver_available
```

### 5. Corpus without a server

```bash
grep -l '"title": ".*raft.*"' data/answers/*.json
python3 -c "import json; print(json.load(open('data/answers/0043-go-raft-log-replication.json'))['answers'][0]['solution'])"
```

## Pitfalls (each cost real time on 2026-08-10 or 2026-08-20; status as of 2026-08-22)

1. **`solver_available:false`** (stats) = no solver; submit now returns
   **503 solver_unavailable** immediately (OB-GAP-034 fixed) — the queue never
   silently accepts. Check stats before relying on the queue anyway.
2. **`seed` is CWD-relative**: run from the repo root (or pass
   `-dir <repo>/data`); a missing/unreadable corpus dir exits non-zero with an
   ERROR line instead of a silent success. `OFF_BY_ONE_DB`/`-db` are honored.
3. **Export/import are config-gated** — 501 unless started with `-export-dir`/
   `-import-dir`. Not a bug.
4. **The 300s bwrap-cap** failure pattern (`signal: killed` at exactly 5m) is
   normal fleet behavior, not a regression — don't chase it. `OB1_BWRAP_TIMEOUT`
   raises the cap.
5. **Queue window is per-class** — `GET /api/v1/queue?limit=100` is dominated
   by `off-by-one-self-test` entries; the authoritative picture is the DB.
   Don't conclude "nothing is happening" from the API window.

Former pitfalls now FIXED (do not re-report): zero-match search nulls
(OB-GAP-046 — `problems`/`related` now return `[]`), `stats.avg_solve_time`
empty (OB-GAP-047 — now computed from completed solves), seed empty-db exit on
a missing corpus dir (OB-GAP-048 — now exits 1 with an ERROR), readonly-mode
discover 403 (OB-GAP-020 — discover works in readonly now), detail status
empty (OB-GAP-024), README corpus counts stale (OB-GAP-021 — counts live in
data/COUNTS.md), corpus test junk (OB-GAP-025 — export filters it),
"solver absent → queue forever" (OB-GAP-034 — submit 503s),
`go-nil-pointer-deref` doc example (OB-GAP-022/045 — examples use
`so-nil-pointer-deref`).

## Ops health checks (verified 2026-08-30 — 10-second probes)

```bash
curl -s localhost:8766/api/v1/stats          # problems/answers/verified/queue/hit_rate
curl -s localhost:8766/health                # uptime — compare vs binary mtime for deploy lag
stat -c '%y' off-by-one                      # binary mtime; if older than HEAD commit → NOT deployed
git log origin/master..HEAD --oneline        # unpushed work (branch IS master)
gh run list -R totalwindupflightsystems/off-by-one --limit 3   # CI
```

**Deploy-lag trap (OB-GAP-062):** a committed fix is NOT live until the binary
is rebuilt AND the server restarted. `verified_answers == total_answers` in
stats is the pre-fix signature of OB-GAP-060. If binary mtime < HEAD commit
time, the fix is not serving — file a board task, don't assume.

## Running a scratch instance (safe testing)

```bash
go build ./cmd/off-by-one
env -u DEEPSEEK_API_KEY OFF_BY_ONE_PORT=8877 OFF_BY_ONE_DB=/tmp/x.db \
  ./off-by-one --skip-sandbox --load-threshold -1
```

Full API on a throwaway DB, no sandbox/keys. Data survives restarts (SQLite WAL).

## Where the knowledge lives

- `docs/integration.md` + `docs/api-reference.md` — the maintained API docs
- `docs/dogfood/2026-08-10-integration.md` — run #1 full evidence
- `docs/dogfood/2026-08-20-integration.md` — run #2 full evidence
- `docs/dogfood/2026-08-30-integration.md` — run #3 (sync-focused) evidence
- `docs/dogfood/diagnostics.md` — build anatomy, error history, right ways
- `.coding-hermes/dogfood-log.md` — verdict log per run
- Board: `.coding-hermes/board/tasks.jsonl` (JSONL-canonical; `tasks.md` is a
  frozen legacy log — append tasks as JSONL rows, not markdown)
