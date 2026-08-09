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
  "submission_id": "sub-abc123",
  "problem_class": "go-nil-pointer-deref",
  "status": "queued",
  "position": 1,
  "estimated_time": "30s",
  "existing_solutions": 0,
  "related_problems": ["go-slice-index-out-of-bounds"]
}
```

`status` is one of `queued`, `deduplicated`, or `rejected`.

**Status codes:** `200` (queued/duplicate), `400` (invalid submission), `409` (duplicate — same tuple already queued or answered), `500` (internal error).

**Example**

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/submit \
  -H "Content-Type: application/json" \
  -d '{
    "problem_class": "go-nil-pointer-deref",
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
| `env` | string | Filter by environment |
| `lang` | string | Filter by language |
| `status` | string | Filter by status (`pending`, `verified`, `failed`, `ci_passed`) |
| `limit` | integer | Page size (default 20, max 100) |
| `offset` | integer | Pagination offset (default 0) |

**Response `200 OK`**

```json
{
  "problems": [
    {
      "id": 1,
      "title": "go-nil-pointer-deref",
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
| `class` | Problem class slug (e.g. `go-nil-pointer-deref`) |

**Response `200 OK`**

Same `ProblemClass` shape as the list endpoint.

**Status codes:** `200`, `404` (problem class not found), `500`.

**Example**

```bash
curl -s http://localhost:8766/api/v1/problems/go-nil-pointer-deref
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
      "problem_class": "go-nil-pointer-deref",
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
curl -s "http://localhost:8766/api/v1/problems/go-nil-pointer-deref/answers?limit=5"
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
curl -s http://localhost:8766/api/v1/problems/go-nil-pointer-deref/answers/42
```

---

## Discovery

### `POST /api/v1/problems/discover`

Search the graph for a pre-verified answer. Matches on `problem_class`, then optionally scores by environment, language, and version.

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
    "problem_class": "go-nil-pointer-deref",
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
      "problem_class": "go-slice-index-out-of-bounds",
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
    "problem_class": "go-nil-pointer-deref",
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
      "problem_class": "go-slice-index-out-of-bounds",
      "relationship": "similar",
      "weight": 0.85
    }
  ]
}
```

**Status codes:** `200`, `404` (problem class not found), `500`.

**Example**

```bash
curl -s http://localhost:8766/api/v1/problems/go-nil-pointer-deref/related
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
      "submission_id": "sub-abc123",
      "problem_class": "go-nil-pointer-deref",
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

Same `QueueEntry` shape as the list endpoint.

**Status codes:** `200`, `404` (submission not found), `500`.

**Example**

```bash
curl -s http://localhost:8766/api/v1/queue/sub-abc123
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
      "title": "go-nil-pointer-deref",
      "description": "Nil pointer dereference in Go code",
      "children": [],
      "answers": [
        {
          "id": 42,
          "problem_class": "go-nil-pointer-deref",
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

```json
{
  "total_problems": 779,
  "total_answers": 911,
  "verified_answers": 911,
  "queue_depth": 4,
  "hit_rate": 1,
  "coverage": 1.17,
  "avg_solve_time": "",
  "readonly": false,
  "solver_available": true
}
```

`readonly` and `solver_available` indicate whether the server is in public catalog mode and whether an active solver is wired up.

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
