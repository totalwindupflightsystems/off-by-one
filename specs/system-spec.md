# Off-by-One — System Specification

> **Version:** 0.1.0-draft
> **Status:** Pre-implementation design
> **Repo:** totalwindupflightsystems/off-by-one

## Table of Contents

1. [Architecture Overview](#1-architecture-overview)
2. [Dual Interface Design](#2-dual-interface-design)
3. [Muster MCP Surface (AI Interface)](#3-muster-mcp-surface-ai-interface)
4. [Web UI (Human Interface)](#4-web-ui-human-interface)
5. [Output Templates](#5-output-templates)
6. [Bubblewrap Sandbox](#6-bubblewrap-sandbox)
7. [Graph Discovery Engine](#7-graph-discovery-engine)
8. [Git Export/Import](#8-git-exportimport)
9. [Data Flow Diagrams](#9-data-flow-diagrams)
10. [API Specification (OpenAPI)](#10-api-specification-openapi)

---

## 1. Architecture Overview

```
                          ┌──────────────────────────────┐
                          │        Muster MCP Server       │
                          │   (wojons/muster, Go 1.26)    │
                          │   Auto-configures from OAS    │
                          └──────┬──────────────┬────────┘
                                 │              │
                          submit │              │ discover
                                 ▼              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Off-by-One Server                        │
│                                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌────────────┐  │
│  │  Ingest  │  │ Sandbox  │  │  Solver  │  │   Graph    │  │
│  │  Queue   │──▶ (bwrap)  │──▶ Pi Agent │──▶  (SQLite)  │  │
│  └──────────┘  └──────────┘  └──────────┘  └─────┬──────┘  │
│                                                   │         │
│  ┌──────────┐  ┌──────────┐                      │         │
│  │  Export  │  │  Import  │                      │         │
│  │ (git)    │  │ (git)    │                      │         │
│  └────┬─────┘  └────▲─────┘                      │         │
│       │              │                           │         │
├───────┼──────────────┼───────────────────────────┼─────────┤
│       │              │                           │         │
│  ┌────┴──────────────┴───────────────────────────┴───────┐ │
│  │                    Web UI                              │ │
│  │  ┌────────┐ ┌────────┐ ┌────────┐ ┌────────────────┐  │ │
│  │  │ Search │ │ Submit │ │Explore │ │ Import/Export  │  │ │
│  │  └────────┘ └────────┘ └────────┘ └────────────────┘  │ │
│  │  ┌──────────────────────────────────────────────────┐  │ │
│  │  │              AI Agent Chat                        │  │ │
│  │  └──────────────────────────────────────────────────┘  │ │
│  └───────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

### Component Summary

| Component | Role | Technology |
|-----------|------|------------|
| **Muster MCP Server** | Bidirectional bridge: agents submit + discover | Go, wojons/muster |
| **Ingest Queue** | Validate, deduplicate, prioritize problems | Go, in-memory + SQLite |
| **Sandbox** | Isolate problem reproduction | Bubblewrap (`bwrap`) |
| **Solver** | Pi Agent fixes in sandbox | TypeScript, spawned via bwrap |
| **Graph Engine** | Store + traverse problem-class tree | SQLite + recursive CTE |
| **Export/Import** | Push/pull verified answers to git | Go + git2go or exec git |
| **Web UI** | Human control center | Go embed (HTML/JS/CSS) |
| **AI Chat** | Human talks to agent about problems | WebSocket → Pi Agent |

---

## 2. Dual Interface Design

Off-by-One serves two audiences through two interfaces, backed by the same engine:

### AI Interface (Muster MCP)
- **Audience:** AI coding agents (Coding Hermes, OpenCode, Claude Code, etc.)
- **Protocol:** MCP tools auto-generated from OpenAPI spec
- **Workflow:** Agent hits an error → submits problem → continues working → later discovers pre-verified answer
- **Tone:** Machine-optimized, structured JSON

### Human Interface (Web UI)
- **Audience:** Developers, operators, maintainers
- **Protocol:** HTTP/WebSocket, browser-rendered
- **Workflow:** Browse problem taxonomy → search for solutions → submit problems → export/import → chat with AI agent
- **Tone:** Human-readable, exploratory, visual

### Shared Engine
Both interfaces hit the same Go server. The server exposes:
- REST API (consumed by Web UI and Muster)
- WebSocket (AI chat in Web UI)
- OpenAPI spec (consumed by Muster for MCP tool generation)

```
AI Agent → Muster MCP → Off-by-One API ─┐
                                         ├── SQLite Graph
Human    → Web UI     → Off-by-One API ─┘
Human    → Web UI     → WebSocket → Pi Agent Chat
```

---

## 3. Muster MCP Surface (AI Interface)

Muster auto-discovers Off-by-One's API and generates MCP tools. The OpenAPI spec defines:

### 3.1 Tools (AI → Off-by-One)

#### `submit_problem`
Agent submits a problem it expects to hit, just hit, or just debugged.

**Input:**
```json
{
  "problem_class": "file-ownership-after-container-transfer",
  "environment": "docker",
  "language": "go",
  "version": "go-1.26",
  "description": "When transferring files from a build container to a runtime container via Docker volume, file ownership defaults to root:root instead of the target user. This causes 'permission denied' when the runtime container tries to execute or read the files.",
  "error_message": "fork/exec /app/binary: permission denied",
  "stack_trace": "...",
  "context": {
    "project": "muster",
    "phase": "build",
    "files_involved": ["Dockerfile", "docker-compose.yml"],
    "expected_behavior": "Files owned by appuser:appuser",
    "actual_behavior": "Files owned by root:root"
  },
  "cadence": "pre-phase"
}
```

**Output:**
```json
{
  "submission_id": "sub_a1b2c3d4",
  "problem_class": "file-ownership-after-container-transfer",
  "status": "queued",
  "position": 3,
  "estimated_time": "~2m",
  "existing_solutions": 0,
  "related_problems": ["docker-volume-permissions", "chown-in-dockerfile"]
}
```

**Cadence values:**
- `pre-phase` — Agent about to start build phase, expects this problem
- `end-of-day` — Problem occurred during the day
- `post-debug` — Agent just solved this; submit to help others

#### `discover_solution`
Agent queries for pre-verified answers.

**Input:**
```json
{
  "problem_class": "file-ownership-after-container-transfer",
  "environment": "docker",
  "language": "go",
  "version": "go-1.26",
  "include_related": true
}
```

**Output:**
```json
{
  "found": true,
  "answer": {
    "id": "ans_x1y2z3",
    "problem_class": "file-ownership-after-container-transfer",
    "solution": "## Solution\n\nUse `COPY --chown=appuser:appuser` in Dockerfile instead of relying on volume mount ownership propagation...",
    "evidence": "Verified: solves the issue in Docker 24.0+, confirmed on Ubuntu 22.04 and Alpine 3.19. Validator ring: 2/2 passed.",
    "status": "verified",
    "signatures": 2,
    "created_at": "2026-06-28T14:22:00Z"
  },
  "related": [
    {
      "problem_class": "docker-volume-permissions",
      "relationship": "same_root_cause",
      "relevance": 0.95
    },
    {
      "problem_class": "chown-in-dockerfile",
      "relationship": "prerequisite",
      "relevance": 0.85
    }
  ],
  "version_warnings": [
    "Docker 26.0+ changed default ownership behavior — this solution works for 24.x-25.x, untested on 26+"
  ]
}
```

#### `list_problems`
Browse the problem taxonomy.

**Input:**
```json
{
  "environment": "docker",
  "language": "go",
  "status": "verified",
  "limit": 20,
  "offset": 0
}
```

**Output:**
```json
{
  "problems": [
    {
      "problem_class": "file-ownership-after-container-transfer",
      "description_short": "Files owned by root after Docker volume transfer",
      "answer_count": 1,
      "status": "verified",
      "hit_count": 47,
      "last_hit": "2026-06-30T10:15:00Z"
    }
  ],
  "total": 1
}
```

#### `get_queue_status`
Check position of submitted problems.

**Input:**
```json
{
  "submission_id": "sub_a1b2c3d4"
}
```

**Output:**
```json
{
  "submission_id": "sub_a1b2c3d4",
  "status": "in_progress",
  "stage": "sandbox_solve",
  "started_at": "2026-06-30T15:22:00Z"
}
```

### 3.2 OpenAPI Spec Structure

Muster auto-generates MCP tools from this spec. The spec lives at `/openapi.json` on the Off-by-One server.

```yaml
openapi: 3.0.3
info:
  title: Off-by-One Pre-Solve Lab
  version: 0.1.0
servers:
  - url: http://localhost:8766
paths:
  /api/v1/problems/submit:
    post:
      operationId: submitProblem
      summary: Submit a problem for pre-solving
      # ...
  /api/v1/problems/discover:
    post:
      operationId: discoverSolution
      summary: Query for pre-verified answers
      # ...
  /api/v1/problems:
    get:
      operationId: listProblems
      summary: Browse problem taxonomy
      # ...
  /api/v1/queue/{submission_id}:
    get:
      operationId: getQueueStatus
      summary: Check submission status
      # ...
  /api/v1/export:
    post:
      operationId: exportToGit
      summary: Export verified answers to git repo
      # ...
  /api/v1/import:
    post:
      operationId: importFromGit
      summary: Import answers from git repo
      # ...
```

---

## 4. Web UI (Human Interface)

### 4.1 Layout

```
┌──────────────────────────────────────────────────────────┐
│  Off-by-One — Pre-Solve Lab                              │
│  [Search] [Submit] [Explore] [Export] [Import] [Settings]│
├──────────────────────────────────────────────────────────┤
│                                                          │
│  ┌─────────────────────────┐ ┌──────────────────────────┐│
│  │                         │ │                          ││
│  │    Main Content Area    │ │    AI Agent Chat         ││
│  │    (search results,     │ │                          ││
│  │     problem details,    │ │  "I'm hitting a Docker   ││
│  │     answer view,        │ │   ownership issue..."    ││
│  │     taxonomy browser)   │ │                          ││
│  │                         │ │  [Agent response...]     ││
│  │                         │ │                          ││
│  │                         │ │  [Type message...]  [Send]││
│  └─────────────────────────┘ └──────────────────────────┘│
│                                                          │
└──────────────────────────────────────────────────────────┘
```

### 4.2 Views

#### Search
- Full-text search across problem classes, solutions, error messages
- Filter by environment, language, version, status
- Results show: problem class name, short description, answer count, hit count
- Click to expand → full solution, evidence, related problems graph

#### Submit
- Form: problem class, environment, language, version
- Rich text editor for description + error message
- Submit button → queued, shows position + estimated time
- Option: attach files (Dockerfile, logs, etc.)

#### Explore
- Tree view of problem taxonomy
- Click node → expand children (environments, versions)
- Click leaf → full answer view
- Related problems shown as connected graph (simple D3.js force layout)

#### Export
- Select problems to export (checkboxes)
- Choose target git repo + branch
- Preview diff before push
- Push button → git subtree commit

#### Import
- Enter git repo URL
- Preview incoming answers (diff against local graph)
- Select which to import
- Import button → merge into local graph

#### AI Agent Chat
- WebSocket connection to Off-by-One server
- Server forwards to Pi Agent
- Human describes problem → agent diagnoses → suggests whether a pre-solved answer exists or submits new problem
- Chat context includes current graph state (agent can search while chatting)

### 4.3 Key Interactions

| Action | UI Path | API Call |
|--------|---------|----------|
| Search for answer | Search bar → type → results | `GET /api/v1/problems?q=...` |
| View answer | Click result → full view | `GET /api/v1/problems/{class}/answers/{id}` |
| Submit problem | Submit tab → fill form → submit | `POST /api/v1/problems/submit` |
| Browse taxonomy | Explore tab → click tree | `GET /api/v1/taxonomy` |
| Chat with agent | Chat panel → type → send | `WS /ws/chat` |
| Export to git | Export tab → select → push | `POST /api/v1/export` |
| Import from git | Import tab → URL → select → import | `POST /api/v1/import` |
| See related problems | Answer view → graph panel | `GET /api/v1/problems/{class}/related` |

### 4.4 Tech Stack

- **Backend:** Go server embeds HTML/JS/CSS via `embed.FS`
- **Frontend:** Vanilla JS + HTMX for interactivity, D3.js for graph visualization
- **Chat:** WebSocket, server relays to Pi Agent via bwrap sandbox
- **Auth:** None initially (localhost only). API key for remote access later.
- **Port:** 8766 (off-by-one on a phone keypad = 633-28-... ok that's forced. 8766 = "OFF-BY-ONE" adjacent to the pun.)

---

## 5. Output Templates

### 5.1 Answer Template (solution.md)

```markdown
# Problem: {problem_class}

**Environment:** {environment}
**Language:** {language} {version}
**Status:** {status} | **Signatures:** {count}/3 validators

---

## Problem Description

{description}

### Error Message
```
{error_message}
```

---

## Solution

{solution_markdown}

### Code Fix
```{language}
{code_fix}
```

### Explanation
{explanation}

---

## Evidence

### Validator Results

| Validator | Model | Result | Tests Written |
|-----------|-------|--------|---------------|
| {name} | {model} | ✅ Passed | {count} |
| {name} | {model} | ✅ Passed | {count} |

### Edge Cases Tested
- {edge_case_1}
- {edge_case_2}
- {edge_case_3}

### Reproducibility
{repro_steps}

---

## Metadata

- **Created:** {created_at}
- **Solved by:** {solver_model}
- **Solve time:** {solve_duration}
- **Hit count:** {hit_count}
- **Related:** {related_links}
```

### 5.2 Discovery Result Template

When an agent queries and finds a solution, the response includes:

```
📦 PRE-SOLVED ANSWER FOUND
Problem: file-ownership-after-container-transfer
Environment: docker | Language: go | Version: go-1.26
Status: ✅ Verified (2/2 validators)

Solution: Use COPY --chown=appuser:appuser in Dockerfile instead
of relying on volume mount ownership propagation...

[View full solution] [Related: 2 problems] [Version warnings: 1]

⚠️ Version Warning: Docker 26.0+ changed default ownership behavior
   — this solution works for 24.x-25.x, untested on 26+
```

### 5.3 Queue Status Template

```
📋 SUBMISSION STATUS
ID: sub_a1b2c3d4
Problem: file-ownership-after-container-transfer
Status: 🔄 In Progress — Sandbox Solve
Position: #3 → #1 → In Progress
Started: 2026-06-30T15:22:00Z (~1m ago)
Estimated: ~2m remaining
```

---

## 6. Bubblewrap Sandbox

### 6.1 Why Bubblewrap

Bubblewrap (`bwrap`) provides unprivileged containerization without Docker overhead:

| Aspect | Docker | Bubblewrap |
|--------|--------|------------|
| Daemon required | dockerd | None |
| Root required | Yes (daemon) | No (user namespaces) |
| Image overhead | Layers + registry | None (bind mounts) |
| Startup time | ~1-5s | ~10ms |
| Filesystem isolation | Full overlayfs | Bind mounts + tmpfs |
| Network isolation | Full namespace | None by default |
| Resource limits | cgroups | rlimits (ulimit) |

For a single-machine pre-solve lab, Docker is overkill. Bubblewrap gives us:
- Isolated filesystem so Pi Agent can't touch the host
- Temp directory for sandbox work
- Read-only access to problem files
- Fast spin-up/down (no image pull, no layer extraction)

### 6.2 Sandbox Profile

```bash
bwrap \
  --ro-bind /usr /usr \
  --ro-bind /lib /lib \
  --ro-bind /lib64 /lib64 \
  --ro-bind /bin /bin \
  --bind /tmp/off-by-one/sandbox-{id} /workspace \
  --tmpfs /tmp \
  --tmpfs /var \
  --tmpfs /run \
  --proc /proc \
  --dev /dev \
  --unshare-all \
  --die-with-parent \
  --new-session \
  /workspace/entrypoint.sh
```

### 6.3 Sandbox Lifecycle

```
1. CREATE: bwrap container with bind-mounted workspace
2. SETUP: Copy problem files + Pi Agent into workspace
3. SOLVE: Pi Agent runs inside bwrap, produces solution
4. EXTRACT: Copy solution + evidence out of workspace
5. DESTROY: Kill bwrap process, rm workspace dir
6. TIMEOUT: 5 minute wall clock. Agent stuck = kill.
```

### 6.4 Pi Agent Integration

The solver spawns Pi Agent inside the sandbox:

```
Off-by-One Server
  └─ bwrap exec
       └─ pi-agent solve \
            --problem-file /workspace/problem.json \
            --output /workspace/solution.md \
            --model deepseek-v4-flash \
            --api-key $DEEPSEEK_API_KEY
```

Pi Agent receives the problem as structured JSON, investigates, attempts fixes, and writes the solution. The server monitors the sandbox process and enforces the 5-minute timeout.

### 6.5 Tool Provisioning

Problems may declare tools they need inside the sandbox via the `required_tools` field (a JSON array of tool names like `["jq", "parallel", "python3-venv"]`). At sandbox creation time, each declared tool is resolved on the host:

- **Standard binaries** (`jq`, `parallel`, etc.) — resolved via `exec.LookPath` and mounted read-only at their host path.
- **git** — already in `DefaultReadOnlyPaths` (`/usr/bin/git` + `/usr/lib/git-core`); deduplicated so no duplicate `--ro-bind` entry is emitted.
- **python3-venv** — the `python3` binary plus `/usr/lib/python3*/venv` support directories.

**Degrade-gracefully contract:** a tool that cannot be resolved on the host (e.g. `jq` not installed) NEVER fails the solve. The resolver logs a warning via `slog` and the solve proceeds without that tool — Pi Agent may still produce a useful answer with whatever tools are available. Paths already covered by `DefaultReadOnlyPaths` (e.g. `/usr/bin/git` is under `/usr`) are skipped to avoid redundant bind mounts.

---

## 7. Graph Discovery Engine

### 7.1 Schema (Implemented in `sql/schema.sql`)

Already created. Three tables:
- `problem_classes` — taxonomy root nodes
- `answer_nodes` — version-branched solution leaves, self-referential `parent_id`
- `problem_edges` — lateral connections (same_root_cause, prerequisite, superseded_by, generalizes)

### 7.2 Discovery Query (BFS)

```sql
WITH RECURSIVE tree AS (
    SELECT *, 0 as depth
    FROM answer_nodes
    WHERE class_id = ? AND env = ? AND version = ?
    
    UNION ALL
    
    SELECT a.*, t.depth + 1
    FROM answer_nodes a
    JOIN tree t ON a.parent_id = t.id
)
SELECT DISTINCT a.*, e.relationship, e.weight
FROM tree t
LEFT JOIN problem_edges e ON e.source_id = t.class_id
LEFT JOIN answer_nodes a ON a.class_id = e.target_id
ORDER BY t.depth, e.weight DESC;
```

### 7.3 Ranking

Results ranked by:
1. Exact match (same version) — score 1.0
2. Parent version (e.g., go-1.25 for go-1.26 query) — score 0.8
3. Same problem class, different env — score 0.6
4. Related via lateral edge — score = edge weight
5. Semantic similarity (OpenRouter embeddings) — score 0.3–0.7

---

## 8. Git Export/Import

### 8.1 Export Format

```
pre-solve-answers/
  file-ownership-after-container-transfer/
    docker/
      go-1.26/
        solution.md
        evidence.md
        signatures.json
    k8s/
      go-1.26/
        solution.md
        evidence.md
        signatures.json
  docker-volume-permissions/
    docker/
      python-3.11/
        solution.md
        evidence.md
        signatures.json
```

### 8.2 Export Flow

1. User (human or agent) selects verified answers to export
2. Server clones target repo (or uses existing clone)
3. Generates markdown + evidence files in S3-key depth path
4. Commits as subtree: `git subtree add --prefix=pre-solve-answers/{class}`
5. Pushes to remote
6. Returns: commit SHA, PR URL (if applicable)

### 8.3 Import Flow

1. User provides git repo URL
2. Server clones/pulls repo
3. Parses `pre-solve-answers/` directory tree
4. Diffs against local graph:
   - **New:** problem class not in local graph → show as import candidate
   - **Updated:** same class, newer answer → show with diff
   - **Conflict:** same class+version, different answer → show side-by-side
5. User selects which to import
6. Server inserts into local SQLite graph
7. Returns: import summary (added, updated, skipped)

### 8.4 Community Model

```
                    ┌──────────────────────┐
                    │   Community Repo      │
                    │   (public, read-only) │
                    │   off-by-one-answers  │
                    └──────────┬───────────┘
                               │
                    PR merge (maintainers)
                               │
         ┌─────────────────────┼─────────────────────┐
         │                     │                     │
   Private Instance      Private Instance      Private Instance
   (imports from         (imports from         (imports from
    community repo)       community repo)       community repo)
```

---

## 9. Data Flow Diagrams

### 9.1 Agent Submission → Discovery

```
Agent hits error
     │
     ▼
Muster MCP: submit_problem()
     │
     ▼
Off-by-One: validate → deduplicate → queue
     │
     ▼
Idle cycle: dequeue → sandbox → Pi Agent solve
     │
     ▼
Solution stored in SQLite graph
     │
     ▼
Agent hits same error class later
     │
     ▼
Muster MCP: discover_solution()
     │
     ▼
Off-by-One: BFS graph traversal → return answer + related + warnings
     │
     ▼
Agent applies pre-verified solution. Skip debugging.
```

### 9.2 Human Search → Export

```
Human opens Web UI
     │
     ▼
Searches: "docker permission denied"
     │
     ▼
Server: full-text SQLite FTS5 + graph traversal
     │
     ▼
UI: shows matching problems with answer count
     │
     ▼
Human clicks → full solution with related problems graph
     │
     ▼
Human clicks Export → selects answers → targets repo
     │
     ▼
Server: generates subtree files → commits → pushes
```

### 9.3 Human Chat → Submission

```
Human opens chat panel
     │
     ▼
Types: "I'm getting 'permission denied' after docker cp"
     │
     ▼
Server: forwards to Pi Agent via WebSocket
     │
     ▼
Pi Agent: searches local graph → finds no answer
     │
     ▼
Pi Agent: "I couldn't find a pre-solved answer for this.
           Want me to submit it to the queue?"
     │
     ▼
Human: "Yes"
     │
     ▼
Server: submit_problem() → queue
     │
     ▼
Pi Agent: "Submitted as 'docker-cp-permission-denied'.
           Position #2. ~3m estimated."
```

---

## 10. API Specification (OpenAPI)

The full OpenAPI 3.0.3 spec lives at `pkg/api/openapi.yaml`. Key endpoints:

| Method | Path | Operation | Description |
|--------|------|-----------|-------------|
| POST | `/api/v1/problems/submit` | submitProblem | Submit problem to queue |
| POST | `/api/v1/problems/discover` | discoverSolution | Query for pre-verified answer |
| GET | `/api/v1/problems` | listProblems | Browse/search problem taxonomy |
| GET | `/api/v1/problems/{class}` | getProblemClass | Get problem class detail |
| GET | `/api/v1/problems/{class}/answers` | listAnswers | List answers for problem class |
| GET | `/api/v1/problems/{class}/answers/{id}` | getAnswer | Get specific answer |
| GET | `/api/v1/problems/{class}/related` | getRelated | Get related problems (graph edges) |
| GET | `/api/v1/queue` | listQueue | List all queued problems |
| GET | `/api/v1/queue/{submission_id}` | getQueueStatus | Check submission status |
| POST | `/api/v1/export` | exportToGit | Export answers to git |
| POST | `/api/v1/import` | importFromGit | Import answers from git |
| GET | `/api/v1/taxonomy` | getTaxonomy | Full problem-class tree |
| GET | `/api/v1/stats` | getStats | Hit rate, coverage, queue depth |
| WS | `/ws/chat` | chatWithAgent | WebSocket chat with Pi Agent |
