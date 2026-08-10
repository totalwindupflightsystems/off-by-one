---
name: off-by-one-usage
description: >-
  How to USE the Off-by-One pre-solve lab as a real user (agent or operator):
  discover cached answers, submit problems, poll the queue, browse the corpus,
  run a scratch instance, and the pitfalls that waste time. Load this skill
  before doing anything with the off-by-one repo or its API.
version: 1.0.0
category: software-development
---

# Off-by-One Usage — Field-Tested Guide

**What it is:** a pre-solve lab. AI agents submit problems via a REST API; during
idle cycles the lab solves them (Pi Agent inside a bubblewrap sandbox) and caches
verified answers in a SQLite graph. Any agent hitting the same problem class later
discovers the pre-verified answer instead of debugging from scratch. Answers are
also published as flat files (`data/answers/`, `data/answers.jsonl`) and a web UI.

Field-tested 2026-08-10 (coding-hermes-dogfood). Verdict: 🟡 PROMISING-BUT-ROUGH.

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
- Use a real class from `data/INDEX.md` — the README's `go-nil-pointer-deref`
  example does not exist (404) (OB-GAP-022).
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

## Pitfalls (each cost real time on 2026-08-10)

1. **Readonly mode has no discovery** — `--readonly` 403s ALL POSTs, including the
   only discover endpoint. Catalog instances are human-browse-only until OB-GAP-020
   lands. Don't wire an agent to a readonly instance.
2. **`solver_available:false`** (stats) = submissions queue forever. Startup logs
   `WARN: cron loop not started — no solver available`. Check stats before relying
   on the queue.
3. **Detail endpoint lies about status** — `GET /api/v1/problems/{class}` returns
   `"status":""`; use list/discover for status (OB-GAP-024).
4. **README counts are stale** (812/948 vs live 874/1012 at last check) — trust
   `/api/v1/stats` or `data/INDEX.md`, not README (OB-GAP-021).
5. **Export/import are config-gated** — 501 unless started with `-export-dir`/
   `-import-dir`. Not a bug.
6. **Corpus has test junk** (`0009-test-self-dogfood`, `0016-test`,
   `docs-canary-*`) — filter when bulk-consuming (OB-GAP-025).
7. **The 300s bwrap-cap** failure pattern is normal fleet behavior, not a
   regression — don't chase it.

## Running a scratch instance (safe testing)

```bash
go build ./cmd/off-by-one
env -u DEEPSEEK_API_KEY OFF_BY_ONE_PORT=8877 OFF_BY_ONE_DB=/tmp/x.db \
  ./off-by-one --skip-sandbox --load-threshold -1
```

Full API on a throwaway DB, no sandbox/keys. Data survives restarts (SQLite WAL).

## Where the knowledge lives

- `docs/integration.md` + `docs/api-reference.md` — the maintained API docs
- `docs/dogfood/2026-08-10-integration.md` — this run's full evidence
- `docs/dogfood/diagnostics.md` — build anatomy, error history, right ways
- `.coding-hermes/dogfood-log.md` — verdict log per run
- Board: `.coding-hermes/board/tasks.jsonl` (JSONL-canonical; `tasks.md` is a
  frozen legacy log — append tasks as JSONL rows, not markdown)
