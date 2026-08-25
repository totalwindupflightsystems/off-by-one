# Off-by-One — Integration Guide

This guide is for AI agents and developers who want to submit problems to the pre-solve lab, poll the queue, and discover cached answers over HTTP.

Base URL: `http://localhost:8766` (or wherever the binary is listening).

## Table of Contents

1. [Quick Start: Run the Server](#quick-start-run-the-server)
2. [Submit a Problem](#submit-a-problem)
3. [Poll the Queue](#poll-the-queue)
4. [Discover Cached Solutions](#discover-cached-solutions)
5. [Browse Taxonomy and Stats](#browse-taxonomy-and-stats)
6. [Import / Export Corpora](#import--export-corpora)
7. [Deduplication and Queue Behavior](#deduplication-and-queue-behavior)
8. [Read-Only Catalog Mode](#read-only-catalog-mode)
9. [MCP / Muster Auto-Discovery](#mcp--muster-auto-discovery)
10. [Seeding the Answer Corpus](#seeding-the-answer-corpus)

---

## Quick Start: Run the Server

```bash
# Build
go build ./cmd/off-by-one

# Run with defaults (port 8766, ./off-by-one.db, solver requires bwrap + pi-agent)
./off-by-one

# Or run for dev without a sandbox
./off-by-one --skip-sandbox
```

Required environment variables and flags are documented in `README.md` and summarized at the bottom of this file. The lab needs `DEEPSEEK_API_KEY` in the environment when the solver is enabled (bwrap + pi-agent). `OPENROUTER_API_KEY` is optional and only used for embeddings.

Health check:

```bash
curl -s http://localhost:8766/health
# {"status":"ok","uptime":"..."}
```

---

## Submit a Problem

`POST /api/v1/problems/submit` accepts a JSON body (or `multipart/form-data` with a `data` field containing JSON and optional file attachments).

### Cadence values

The queue ranks submissions by cadence. Higher-weight cadences are solved first.

| Cadence | Weight | When to use |
|---------|--------|-------------|
| `pre-phase` | 1 | Before starting a new phase of work (lowest urgency) |
| `end-of-day` | 2 | Nightly batch of edge cases |
| `post-debug` | 3 | Right after debugging a failure (highest urgency) |

Priority formula: `priority = cadence_weight + recurrence * 0.5`, where recurrence is the number of times the same problem class has already been submitted.

### Request fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `problem_class` | string | Yes | Slugified problem identifier (e.g. `so-nil-pointer-deref`) |
| `cadence` | string | Yes | One of `pre-phase`, `end-of-day`, `post-debug` |
| `environment` | string | No | Runtime environment (e.g. `linux`, `docker`) |
| `language` | string | No | Programming language (e.g. `go`, `python`) |
| `version` | string | No | Toolchain version (e.g. `1.26.1`) |
| `description` | string | No | Human-readable description |
| `error_message` | string | No | Error text from the failing run |
| `stack_trace` | string | No | Stack trace or log excerpt |
| `context` | object | No | Arbitrary key-value metadata (e.g. repo, commit, file path) |
| `required_tools` | string[] | No | Tools the sandbox should mount read-only (e.g. `jq`, `parallel`) |

### Example

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/submit \
  -H "Content-Type: application/json" \
  -d '{
    "problem_class": "so-nil-pointer-deref",
    "environment": "linux",
    "language": "go",
    "version": "1.26.1",
    "description": "Nil pointer dereference in HTTP handler",
    "error_message": "runtime error: invalid memory address",
    "cadence": "post-debug"
  }'
```

### Response

```json
{
  "submission_id": "sub_...",
  "problem_class": "so-nil-pointer-deref",
  "status": "queued",
  "position": 1,
  "estimated_time": "30s",
  "existing_solutions": 0,
  "related_problems": ["so-nil-pointer-deref"]
}
```

Possible statuses: `queued`, `deduplicated`, `rejected`. A 400 response means the submission was invalid (missing `problem_class`, unknown `cadence`, etc.). A 409 response means the same `(class, environment, language, version)` tuple is already pending or already has a verified answer — see [Deduplication](#deduplication-and-queue-behavior).

---

## Poll the Queue

Use the submission ID returned by `submit` to track progress.

### Check one submission

```bash
curl -s http://localhost:8766/api/v1/queue/sub_...
```

```json
{
  "submission_id": "sub_...",
  "problem_class": "so-nil-pointer-deref",
  "status": "in_progress",
  "stage": "sandbox_solve",
  "position": 1,
  "estimated_time": "30s",
  "started_at": "...",
  "completed_at": ""
}
```

Possible statuses: `pending`, `in_progress`, `complete`, `failed`. Stages reported in `stage` include `queued`, `sandbox_prepare`, `sandbox_solve`, and `done`/`failed`.

### List all submissions

```bash
# all
curl -s http://localhost:8766/api/v1/queue

# filtered by status
curl -s "http://localhost:8766/api/v1/queue?status=pending"
```

The `position` field in a list response is the 1-based place in the queue. Positions are recalculated on each request.

---

## Discover Cached Solutions

`POST /api/v1/problems/discover` searches the graph for a verified answer. When only `problem_class` is provided, the best verified answer for that class is returned. `environment`, `language`, and `version` — when provided — are **exact-match filters, not scoring hints**: a query whose tuple matches no stored answer returns `found:false` even if the class has verified answers under other tuples. Omit the tuple fields for the broadest match.

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/discover \
  -H "Content-Type: application/json" \
  -d '{
    "problem_class": "so-nil-pointer-deref"
  }'
```

### Response

```json
{
  "found": true,
  "answer": {
    "id": 42,
    "problem_class": "so-nil-pointer-deref",
    "env": "linux",
    "lang": "go",
    "version": "1.26",
    "solution": "...",
    "evidence": "...",
    "signatures": {},
    "status": "verified",
    "created_at": "..."
  },
  "related": [
    {
      "problem_class": "so-nil-pointer-deref",
      "relationship": "similar",
      "relevance": 0.85
    }
  ],
  "version_warnings": []
}
```

A 404 response means the problem class is not in the graph. If `found` is `false` but the response is 200, the class exists but has no verified answer yet.

Pass `"include_related": false` to skip the related-problems graph edges.

---

## Browse Taxonomy and Stats

### Taxonomy

```bash
curl -s http://localhost:8766/api/v1/taxonomy
```

Returns the full problem-class tree. Each node contains a `title`, `description`, optional `children`, and any cached `answers`.

### Stats

```bash
curl -s http://localhost:8766/api/v1/stats
```

```json
{
  "total_problems": 1281,
  "total_answers": 1457,
  "verified_answers": 1457,
  "queue_depth": 0,
  "hit_rate": 1,
  "coverage": 1.137,
  "avg_solve_time": "2m16s",
  "readonly": false,
  "solver_available": true
}
```

`coverage` = `verified_answers / total_problems` — it can exceed 1.0 because a single problem class may accumulate multiple verified answers; a value above 1 is normal, not corruption. `hit_rate` = `verified_answers / total_answers` (0..1). `readonly` and `solver_available` tell you whether the running instance is a read-only catalog or has an active solver.

---

## Import / Export Corpora

These endpoints require the binary to be started with `-export-dir` / `-import-dir` (or `OFF_BY_ONE_EXPORT_DIR` / `OFF_BY_ONE_IMPORT_DIR`). If the directory is not configured, the endpoint returns `501 Not Implemented`.

### Export verified answers to a git repo

```bash
curl -s -X POST http://localhost:8766/api/v1/export \
  -H "Content-Type: application/json" \
  -d '{
    "target_repo": "https://github.com/example/pre-solve-answers",
    "answer_ids": [42, 43],
    "branch": "main",
    "commit_message": "Add verified answers from Off-by-One"
  }'
```

```json
{
  "commit_sha": "abc123...",
  "pr_url": "...",
  "files_changed": 2
}
```

### Import answers from a git repo

```bash
curl -s -X POST http://localhost:8766/api/v1/import \
  -H "Content-Type: application/json" \
  -d '{
    "source_repo": "https://github.com/example/pre-solve-answers",
    "branch": "main",
    "conflict_strategy": "skip"
  }'
```

```json
{
  "added": 2,
  "updated": 0,
  "skipped": 1,
  "conflicted": 0
}
```

`conflict_strategy` can be `skip`, `replace`, or `manual`.

---

## Deduplication and Queue Behavior

### Deduplication rules

A submission is considered a duplicate if the same `(problem_class, environment, language, version)` tuple is already:

1. In the queue with status `pending` or `in_progress`, OR
2. Already answered by a verified (or `ci_passed`) answer node.

Deduplication does **not** span different environments, languages, or versions — those are treated as different problems.

### Duplicate response

When a duplicate is detected, the API returns HTTP `409 Conflict` with a body like:

```json
{
  "submission_id": "sub_...",
  "problem_class": "so-nil-pointer-deref",
  "status": "deduplicated",
  "position": 1,
  "estimated_time": "30s",
  "existing_solutions": 3,
  "related_problems": ["so-nil-pointer-deref"]
}
```

`existing_solutions` is the count of answers already cached for that problem class. The client should call `POST /api/v1/problems/discover` to retrieve the existing answer instead of waiting for the queue.

### Queue ordering

Pending entries are ordered by priority descending, then `created_at` ascending. The highest-priority pending entry is dequeued during idle cycles, moved to `in_progress`, sandboxed, solved by Pi Agent, and either marked `complete` (with an answer node) or `failed`.

If `solver_available` is false in `/api/v1/stats`, the cron loop is not running and submissions will stay in the queue until the server is restarted with a working solver.

---

## Read-Only Catalog Mode

Start the binary with `--readonly` (or `OFF_BY_ONE_READONLY=1`) to serve a public catalog. In this mode:

- All mutating `POST /api/v1/*` endpoints (`submit`, `export`, `import`) return `403 Forbidden`.
- The WebSocket AI chat endpoint (`/ws/chat`) is disabled.
- `GET` endpoints for problems, taxonomy, stats, and answers still work.
- `POST /api/v1/problems/discover` remains available: discovery is a pure read (cached-answer lookup, no mutation), so agents can still discover pre-verified answers from a read-only catalog.

Use this for `ob1.it.com`-style public deployments where the corpus is pre-solved and no solver keys are present on the box.

---

## MCP / Muster Auto-Discovery

The server exposes the OpenAPI 3.0.3 spec at `/openapi.json`:

```bash
curl -s http://localhost:8766/openapi.json | head -c 200
```

Muster can consume this spec to auto-generate MCP tools for the lab. The spec is also the source of truth for route definitions, request/response schemas, and status codes.

---

## Seeding the Answer Corpus

The `seed` subcommand loads the bundled flat answer corpus (`data/answers/*.json`) into the SQLite graph store, so fresh installs get working discovery immediately instead of 404ing on an empty database. It is idempotent: re-running imports only the corpus delta, so it is safe to re-run after corpus updates.

```bash
# Load the bundled corpus into the default DB (./off-by-one.db)
./off-by-one seed

# Custom corpus dir and DB path
./off-by-one seed -dir ./data -db /var/lib/off-by-one/off-by-one.db
```

Database path resolution: the `-db` flag wins, then the `OFF_BY_ONE_DB` environment variable (see [Configuration Reference](#configuration-reference)), then the default `./off-by-one.db`. The `-dir` flag defaults to `./data` and only needs to change if the corpus lives elsewhere.

---

## Configuration Reference

### Environment variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DEEPSEEK_API_KEY` | Yes (for solver) | — | DeepSeek API key for Pi Agent |
| `OPENROUTER_API_KEY` | No | — | Optional embeddings key |
| `OFF_BY_ONE_PORT` | No | `8766` | HTTP port |
| `OFF_BY_ONE_HOST` | No | — | HTTP listen host (empty = all interfaces) |
| `OFF_BY_ONE_DB` | No | `./off-by-one.db` | SQLite path |
| `OFF_BY_ONE_BWRAP` | No | `/usr/bin/bwrap` | Bubblewrap binary path |
| `OFF_BY_ONE_PI_AGENT` | No | `pi-agent` | Pi Agent binary path |
| `OFF_BY_ONE_CRON_INTERVAL` | No | `5m` | Cron wake interval |
| `OFF_BY_ONE_LOAD_THRESHOLD` | No | `1` | Max loadavg(1) for idle detection (negative = always idle) |
| `OFF_BY_ONE_SOLVE_TIMEOUT` | No | `30m` | Per-solve timeout (handled by solver) |
| `OFF_BY_ONE_EXPORT_DIR` | No | — | Working dir for git export clones (empty = export disabled) |
| `OFF_BY_ONE_IMPORT_DIR` | No | — | Working dir for git import clones (empty = import disabled) |
| `OFF_BY_ONE_READONLY` | No | `false` | Public catalog mode (set `1`/`true`/`yes`) |
| `OFF_BY_ONE_SKIP_SANDBOX` | No | `false` | Skip bwrap for dev/testing (set `1`/`true`/`yes`) |

### Command-line flags

```bash
go run ./cmd/off-by-one --help
```

| Flag | Description |
|------|-------------|
| `--port` | HTTP listen port |
| `--host` | HTTP listen host (empty = all interfaces) |
| `--db` | SQLite database path |
| `--bwrap` | Path to bubblewrap binary |
| `--pi-agent` | Path to Pi Agent binary |
| `--cron-interval` | Cron loop wake interval |
| `--load-threshold` | Max loadavg(1) for idle detection (negative = always idle) |
| `--skip-sandbox` | Skip bwrap sandbox (for dev/testing) |
| `--solve-timeout` | Per-solve timeout cap (env `OFF_BY_ONE_SOLVE_TIMEOUT`) |
| `--readonly` | Public catalog mode |
| `--export-dir` | Working directory for git export clones |
| `--import-dir` | Working directory for git import clones |
| `--version` | Print version and exit |

Every flag has a corresponding environment variable. Flags override environment variables.
