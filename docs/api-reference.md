# Off-by-One — HTTP API Reference

Base URL: `http://localhost:8766`

All timestamps are RFC 3339 strings. All endpoints return JSON unless otherwise noted. Error responses use the shape:

```json
{
  "error": "not_found",
  "message": "problem class not found"
}
```

---

## Table of Contents

1. [Problems](#problems)
2. [Discovery](#discovery)
3. [Queue](#queue)
4. [Export / Import](#export--import)
5. [Taxonomy / Stats](#taxonomy--stats)
6. [System](#system)
7. [Solve timeouts](#solve-timeouts)

---

## Problems

### `POST /api/v1/problems/submit`

Submit a problem to the pre-solve queue. The request body is JSON; `multipart/form-data` with a `data` field is also accepted for file attachments.

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `problem_class` | string | Yes | Slugified problem identifier |
| `cadence` | string | Yes | `pre-phase`, `end-of-day`, or `post-debug` |
| `environment` | string | No | Runtime environment (e.g. `linux`, `docker`) |
| `language` | string | No | Programming language (e.g. `go`, `python`) |
| `version` | string | No | Toolchain version |
| `description` | string | No | Human-readable description |
| `error_message` | string | No | Error text from the failing run |
| `stack_trace` | string | No | Stack trace or log excerpt |
| `context` | object | No | Arbitrary key-value metadata |
| `required_tools` | string[] | No | Tools to mount read-only in the sandbox |

**Response `200 OK`**

```json
{
  "submission_id": "sub_abc123",
  "problem_class": "so-nil-pointer-deref",
  "status": "queued",
  "position": 1,
  "estimated_time": "30s",
  "existing_solutions": 0,
  "related_problems": ["so-nil-pointer-deref"]
}
```

`status` is one of `queued`, `deduplicated`, or `rejected`.

**Status codes:** `200` (queued/duplicate), `400` (invalid submission), `409` (duplicate — same tuple already queued or answered), `500` (internal error), `503` (`solver_unavailable` — no solver wired up; see below).

**Duplicate (`409`) response.** A submission is deduplicated on `(problem_class, environment, language, version)`. When that tuple is already queued/in progress or already has a verified answer, no second job is created: the response carries `"status": "deduplicated"` and the **existing** `submission_id` (the same id the original submit returned) with its queue `position`:

```json
{
  "submission_id": "sub_525a7d",
  "problem_class": "so-nil-pointer-deref",
  "status": "deduplicated",
  "position": 2,
  "existing_solutions": 1
}
```

**Invalid `cadence` (`400`).** An unknown `cadence` is rejected with a message that lists the accepted values and echoes the rejected one:

```json
{
  "error": "invalid_request",
  "message": "ingest: invalid cadence (accepted: pre-phase, end-of-day, post-debug): got \"weekly\""
}
```

**Solver unavailable (`503`).** When the server has no working solver — `bwrap` + `pi-agent` not configured, the same condition `GET /api/v1/stats` reports as `solver_available: false` — the handler rejects the submission up front, **before the queue is touched**: no `submission_id` is issued, no queue entry is created, and there is nothing to poll on `GET /api/v1/queue`.

```json
{
  "error": "solver_unavailable",
  "message": "solver is not available (bwrap + pi-agent not configured); submissions cannot be queued — use POST /api/v1/problems/discover to look up existing answers"
}
```

The check runs first, ahead of body parsing, so this response is returned for both JSON and `multipart/form-data` submissions regardless of body validity. Clients should fall back to `POST /api/v1/problems/discover` to read pre-verified answers from a catalog-only deployment. The condition clears when the server is restarted with `bwrap` and `pi-agent` available.

**Example**

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

---

### `GET /api/v1/problems`

List or search problem classes. Supports full-text search via `q` and filtering via `env`, `lang`, and `status`.

**Query parameters**

| Parameter | Type | Description |
|-----------|------|-------------|
| `q` | string | Full-text search query |
| `env` | string | Filter by environment — matched **exactly** against the `env` stored on the class's answer rows |
| `lang` | string | Filter by language — matched **exactly** against the `lang` stored on the class's answer rows |
| `status` | string | Filter by status (`pending`, `verified`, `failed`, `ci_passed`) |
| `limit` | integer | Page size (default 20, max 100) |
| `offset` | integer | Pagination offset (default 0) |

`env` and `lang` are exact matches (`a.env = ?`, `a.lang = ?`) against the value stored on the class's answer rows — not substring, case-insensitive, or alias matches — and a class is filtered out unless one of its answers carries that exact value. A filter copied from the [README submit example](../README.md#example-submit-a-problem) (`environment: "linux"`, `language: "go"`) can therefore legitimately return 0 rows: the stored values are whatever the corpus recorded, and they are not normalized (observed live: `GET /api/v1/problems?q=raft&env=linux` → `{"problems":[],"total":0}`, while `q=raft` alone returns the matching classes, whose answers carry `env` values such as `go1.26`). Check the stored values first — `GET /api/v1/problems/{class}/answers` returns `env` and `lang` for every answer. The same exact-match rule governs `POST /api/v1/problems/discover`'s `environment` / `language` filters — see [Discover Cached Solutions](integration.md#discover-cached-solutions).

**Response `200 OK`**

```json
{
  "problems": [
    {
      "id": 1,
      "title": "so-nil-pointer-deref",
      "description": "Nil pointer dereference in Go code",
      "answer_count": 3,
      "status": "verified",
      "created_at": "..."
    }
  ],
  "total": 1
}
```

**Status codes:** `200`, `500`.

**Example**

```bash
curl -s "http://localhost:8766/api/v1/problems?q=pointer&limit=10&offset=0"
```

---

### `GET /api/v1/problems/{class}`

Get one problem class by slugified title.

**Path parameter**

| Parameter | Description |
|-----------|-------------|
| `class` | Problem class slug (e.g. `so-nil-pointer-deref`) |

**Response `200 OK`**

Same `ProblemClass` shape as the list endpoint.

**Status codes:** `200`, `404` (problem class not found), `500`.

**Example**

```bash
curl -s http://localhost:8766/api/v1/problems/so-nil-pointer-deref
```

---

### `GET /api/v1/problems/{class}/answers`

List answers for a problem class.

**Path/query parameters**

| Parameter | In | Description |
|-----------|----|-------------|
| `class` | path | Problem class slug |
| `limit` | query | Page size (default 20) |
| `offset` | query | Pagination offset (default 0) |

**Response `200 OK`**

```json
{
  "answers": [
    {
      "id": 42,
      "problem_class": "so-nil-pointer-deref",
      "env": "linux",
      "lang": "go",
      "version": "1.26.1",
      "solution": "...",
      "evidence": "...",
      "signatures": {},
      "status": "verified",
      "created_at": "..."
    }
  ],
  "total": 1
}
```

`status` is one of `pending`, `verified`, `failed`, `ci_passed`.

**Status codes:** `200`, `404` (problem class not found), `500`.

**Example**

```bash
curl -s "http://localhost:8766/api/v1/problems/so-nil-pointer-deref/answers?limit=5"
```

---

### `GET /api/v1/problems/{class}/answers/{id}`

Get a specific answer by ID within a problem class. The answer must belong to the requested class; otherwise a 404 is returned.

**Path parameters**

| Parameter | Description |
|-----------|-------------|
| `class` | Problem class slug |
| `id` | Answer ID (integer) |

**Response `200 OK`**

Same `Answer` shape as the list endpoint.

**Status codes:** `200`, `404` (answer not found or wrong class), `400` (invalid ID), `500`.

**Example**

```bash
curl -s http://localhost:8766/api/v1/problems/so-nil-pointer-deref/answers/42
```

---

## Discovery

### `POST /api/v1/problems/discover`

Search the graph for a pre-verified answer. When only `problem_class` is provided, the best verified answer for that class is returned; `environment`, `language`, and `version` are exact-match filters when provided (a non-matching tuple returns `found:false` even if the class has answers under other tuples).

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `problem_class` | string | Yes | Problem class slug |
| `environment` | string | No | Runtime environment |
| `language` | string | No | Programming language |
| `version` | string | No | Toolchain version |
| `include_related` | boolean | No | Include related problem classes (default `true`) |

**Response `200 OK`**

```json
{
  "found": true,
  "answer": {
    "id": 42,
    "problem_class": "so-nil-pointer-deref",
    "env": "linux",
    "lang": "go",
    "version": "1.26.1",
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

**Status codes:** `200`, `400` (missing `problem_class` or invalid JSON), `404` (problem class not found), `500`.

**Example**

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/discover \
  -H "Content-Type: application/json" \
  -d '{
    "problem_class": "so-nil-pointer-deref",
    "environment": "linux",
    "language": "go",
    "version": "1.26"
  }'
```

---

### `GET /api/v1/problems/{class}/related`

Get related problem classes for a given class using graph edges.

**Path parameter**

| Parameter | Description |
|-----------|-------------|
| `class` | Problem class slug |

**Response `200 OK`**

```json
{
  "related": [
    {
      "problem_class": "so-nil-pointer-deref",
      "relationship": "similar",
      "weight": 0.85
    }
  ]
}
```

**Status codes:** `200`, `404` (problem class not found), `500`.

**Example**

```bash
curl -s http://localhost:8766/api/v1/problems/so-nil-pointer-deref/related
```

---

## Queue

### `GET /api/v1/queue`

List all queued submissions. Optionally filter by `status`.

**Query parameter**

| Parameter | Type | Description |
|-----------|------|-------------|
| `status` | string | `pending`, `in_progress`, `complete`, or `failed` |
| `limit` | integer | Page size (default 100, max 1000) |
| `offset` | integer | Pagination offset (default 0) |

**Response `200 OK`**

```json
{
  "entries": [
    {
      "submission_id": "sub_abc123",
      "problem_class": "so-nil-pointer-deref",
      "status": "pending",
      "stage": "queued",
      "position": 1,
      "estimated_time": "30s",
      "started_at": "",
      "completed_at": ""
    }
  ],
  "total": 1
}
```

`status` is one of `pending`, `in_progress`, `complete`, `failed`. `stage` may be `queued`, `sandbox_prepare`, `sandbox_solve`, `done`, or `failed`.

**`started_at` and `completed_at` semantics**

Both are RFC 3339 UTC timestamps (e.g. `2026-08-15T02:13:43Z`) and both keys are **always present**. `started_at` is set when the entry is dequeued and `completed_at` when the solve finishes or fails; a `pending` entry has not been claimed yet, so it returns `""` for both — the shape the example above shows.

**`position` and `estimated_time` semantics**

| Field | `GET /api/v1/queue` (list) | `GET /api/v1/queue/{submission_id}` |
|-------|----------------------------|-------------------------------------|
| `position` | The entry's 1-based place in the returned page — `offset + index + 1` | The entry's 1-based place in the **pending** queue, or `0` when the entry is not waiting (`in_progress`, `complete`, `failed`) |
| `estimated_time` | Wait left for this entry: `estimateTime(place in the pending queue, observed mean solve time)` while `pending`; one job while `in_progress`; empty for `complete` / `failed` | Same rule as the list |

`estimated_time` is the lab's observed mean solve time (`avg_solve_time` on `GET /api/v1/stats`, derived from completed solves) multiplied by the number of jobs ahead of the entry, rounded to the nearest second and capped at `30m`. A lab with no completed solves yet falls back to `30s` per job — which is why the sample above shows `"estimated_time": "30s"` for the first pending entry. A finished entry is no longer waiting, so `complete` and `failed` entries always return `"estimated_time": ""`.

**Status codes:** `200`, `500`.

**Example**

```bash
curl -s "http://localhost:8766/api/v1/queue?status=pending"
```

---

### `GET /api/v1/queue/{submission_id}`

Get the status of a single submission.

**Path parameter**

| Parameter | Description |
|-----------|-------------|
| `submission_id` | Submission ID returned by `POST /api/v1/problems/submit` |

**Response `200 OK`**

Same `QueueEntry` shape as the list endpoint, and the same `position` / `estimated_time` semantics documented there — `position` is the entry's 1-based place in the pending queue (`0` when the entry is not waiting) and `estimated_time` is empty once the entry is `complete` or `failed` — plus the same `started_at` / `completed_at` rule: RFC 3339, and `""` while the entry is `pending`.

**Status codes:** `200`, `404` (submission not found), `500`.

**Example**

```bash
curl -s http://localhost:8766/api/v1/queue/sub_abc123
```

---

## Export / Import

Both endpoints require the server to be started with `-export-dir` / `-import-dir` (or their environment equivalents). If not configured, the endpoint returns `501 Not Implemented`.

### `POST /api/v1/export`

Export verified answers to a git repository as a subtree commit.

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `target_repo` | string | Yes | Git URL of the target repo |
| `answer_ids` | integer[] | Yes | IDs of answers to export |
| `branch` | string | No | Target branch |
| `commit_message` | string | No | Commit message |

**Response `200 OK`**

```json
{
  "commit_sha": "abc123...",
  "pr_url": "...",
  "files_changed": 2
}
```

`pr_url` is omitted when the repo does not support pull requests.

**Status codes:** `200`, `400` (missing required fields), `501` (export not configured), `500` (export failed).

**Example**

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

---

### `POST /api/v1/import`

Import answers from a git repository.

**Request body**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source_repo` | string | Yes | Git URL of the source repo |
| `branch` | string | No | Branch to import |
| `conflict_strategy` | string | No | `skip`, `replace`, or `manual` |

**Response `200 OK`**

```json
{
  "added": 2,
  "updated": 0,
  "skipped": 1,
  "conflicted": 0
}
```

**Status codes:** `200`, `400` (missing required fields), `501` (import not configured), `500` (import failed).

**Example**

```bash
curl -s -X POST http://localhost:8766/api/v1/import \
  -H "Content-Type: application/json" \
  -d '{
    "source_repo": "https://github.com/example/pre-solve-answers",
    "branch": "main",
    "conflict_strategy": "skip"
  }'
```

---

## Taxonomy / Stats

### `GET /api/v1/taxonomy`

Return the full problem-class tree. Each node includes title, description, optional children, and any cached answers.

**Response `200 OK`**

```json
{
  "tree": [
    {
      "title": "so-nil-pointer-deref",
      "description": "Nil pointer dereference in Go code",
      "children": [],
      "answers": [
        {
          "id": 42,
          "problem_class": "so-nil-pointer-deref",
          ...
        }
      ]
    }
  ]
}
```

**Status codes:** `200`, `500`.

**Example**

```bash
curl -s http://localhost:8766/api/v1/taxonomy
```

---

### `GET /api/v1/stats`

Return system-level statistics.

**Response `200 OK`**

Values below were observed on the running lab at the time of writing — they drift as the corpus grows, so query your own instance for current numbers.

```json
{
  "total_problems": 1847,
  "total_answers": 2036,
  "verified_answers": 2008,
  "queue_depth": 0,
  "hit_rate": 0.9862475442043221,
  "coverage": 1.0871683811586357,
  "avg_solve_time": "2m58s",
  "readonly": false,
  "solver_available": true
}
```

`verified_answers` counts answer rows whose status is `verified` or `ci_passed` **and** whose signatures JSON does not record a failed solve — the predicate is `COALESCE(json_extract(signatures, '$.result'), '') != 'failed'`, so a row with absent, empty, or unparseable signature JSON still counts and only an explicit `result: "failed"` is excluded. A status-verified answer whose signature reports failure is therefore *not* counted: `verified_answers` can be lower than `total_answers` even when nearly every answer is verified, and `hit_rate` is a runtime value, not a constant. `coverage` = `verified_answers / total_problems` — it can exceed 1.0 because a single problem class may accumulate multiple verified answers; a value above 1 is normal, not corruption. `hit_rate` = `verified_answers / total_answers` (0..1). `readonly` and `solver_available` indicate whether the server is in public catalog mode and whether an active solver is wired up.

**Status codes:** `200`, `500`.

**Example**

```bash
curl -s http://localhost:8766/api/v1/stats
```

---

## System

### `GET /openapi.json`

Return the OpenAPI 3.0.3 specification. The server serves it as a JSON object; the `Content-Type` is `application/yaml; charset=utf-8` by current convention.

**Response `200 OK`**

The full OpenAPI 3.0.3 document.

**Status codes:** `200`, `501` (spec not loaded).

**Example**

```bash
curl -s http://localhost:8766/openapi.json | python3 -m json.tool | head -30
```

---

### `GET /health`

Health check. Returns the service status and uptime.

**Response `200 OK`**

```json
{
  "status": "ok",
  "uptime": "8h7m58s"
}
```

**Status codes:** `200`.

**Example**

```bash
curl -s http://localhost:8766/health
```

---

## Read-only catalog mode

When the server is started with `--readonly` (or `OFF_BY_ONE_READONLY=1`), all mutating endpoints (`POST /api/v1/*`, `POST /api/v1/export`, `POST /api/v1/import`) return `403 Forbidden`. The WebSocket chat endpoint (`/ws/chat`) is also disabled. `GET` endpoints for discovery, taxonomy, stats, and answers remain available.

---

## Solve timeouts

A solve is bounded by two independent timeouts, and both must be long enough before a long-running problem can complete:

| Environment variable | Default | Scope |
|----------------------|---------|-------|
| `OB1_BWRAP_TIMEOUT` | `300` (seconds) | Outer cap on the `bwrap` subprocess running the solve. When it fires, the submission fails with `signal: killed` at the cap. |
| `OFF_BY_ONE_SOLVE_TIMEOUT` | `30m` | Solver-level per-solve timeout (also exposed as `--solve-timeout`). |

`OB1_BWRAP_TIMEOUT` takes a **positive integer number of seconds**. If it is unset, non-numeric, or non-positive, the server logs a warning at startup and falls back to the **300-second default** — an invalid value never disables the cap. Raise it when legitimate solves routinely need more than five minutes:

```bash
# 15-minute sandbox cap for this run
OB1_BWRAP_TIMEOUT=900 ./off-by-one
```

Because the bwrap cap is the *outer* limit, raising only `OFF_BY_ONE_SOLVE_TIMEOUT` will not let a solve run past 300s by default — set `OB1_BWRAP_TIMEOUT` above the longest expected solve (and above `OFF_BY_ONE_SOLVE_TIMEOUT` if you intend that solver-level timeout to be the effective one). A repeated `signal: killed` at exactly the configured cap is the sandbox cap doing its job; verify the configured value before treating it as a solver failure.
