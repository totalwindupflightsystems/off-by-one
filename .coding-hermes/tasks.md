# Off-by-One — Implementation Tasks

> Auto-generated from specs/system-spec.md
> Model: MiniMax M3 via ollama-cloud
> Foreman model: deepseek-v4-pro (planning only)

## Phase 1: Core Infrastructure

### [ ] WI-001: OpenAPI spec generation
**Model:** ollama-cloud/minimax-m3
**Files:** pkg/api/openapi.yaml (new)
**Verify:** `yamllint pkg/api/openapi.yaml && curl -s localhost:8766/openapi.json | python3 -m json.tool`
**AC:**
1. Create OpenAPI 3.0.3 spec at pkg/api/openapi.yaml with all endpoints from system spec §10
2. Embed spec via go:embed in cmd/off-by-one/main.go
3. Serve at GET /openapi.json
4. Muster can auto-configure from this endpoint
**Status:** ready

### [ ] WI-002: SQLite graph engine
**Model:** ollama-cloud/minimax-m3
**Files:** internal/graph/store.go (new), internal/graph/discovery.go (new), internal/graph/store_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/graph/...`
**AC:**
1. Initialize SQLite database from sql/schema.sql (embedded via go:embed)
2. Insert/query problem_classes
3. Insert/query answer_nodes with parent_id self-reference
4. Insert/query problem_edges
5. BFS discovery query (recursive CTE) — return exact match + parent versions + lateral edges
6. Full-text search via SQLite FTS5 on problem_classes.title + description
7. Unit tests for all CRUD + discovery queries
**Status:** ready

### [ ] WI-003: Ingest queue
**Model:** ollama-cloud/minimax-m3
**Files:** internal/ingest/queue.go (new), internal/ingest/submit.go (new), internal/ingest/queue_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/ingest/...`
**AC:**
1. In-memory priority queue with SQLite persistence (survive restart)
2. Validate submissions (required fields, valid cadence)
3. Deduplicate against existing queue + existing answers
4. Priority scoring: post-debug > end-of-day > pre-phase, weighted by recurrence
5. Dequeue method for idle cron
6. Queue status tracking (pending → in_progress → complete/failed)
7. Unit tests with concurrent submit + dequeue
**Status:** ready

### [ ] WI-004: HTTP API server
**Model:** ollama-cloud/minimax-m3
**Files:** internal/api/server.go (new), internal/api/handlers.go (new), internal/api/handlers_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/api/...`
**AC:**
1. net/http server on port 8766 (configurable)
2. Routes from OpenAPI spec §10
3. POST /api/v1/problems/submit — validate + enqueue
4. POST /api/v1/problems/discover — graph traversal query
5. GET /api/v1/problems — list with filters (env, lang, status, limit, offset)
6. GET /api/v1/queue/{id} — queue status
7. GET /api/v1/stats — hit rate, coverage, queue depth
8. httptest-based integration tests for all endpoints
**Status:** ready

## Phase 2: Sandbox + Solver

### [ ] WI-005: Bubblewrap sandbox
**Model:** ollama-cloud/minimax-m3
**Files:** internal/sandbox/bwrap.go (new), internal/sandbox/bwrap_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/sandbox/...`
**AC:**
1. bwrap executor: Create() spawns bwrap with bind mounts + tmpfs
2. Configurable workspace dir, timeout (default 5m)
3. Execute command inside sandbox
4. Copy files in/out of sandbox workspace
5. Destroy() kills bwrap process + cleans workspace
6. Handle bwrap not installed: skip tests gracefully
7. Unit tests with mock bwrap (exec.Command override)
**Status:** ready

### [ ] WI-006: Pi Agent solver integration
**Model:** ollama-cloud/minimax-m3
**Files:** internal/solver/piagent.go (new), internal/solver/piagent_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/solver/...`
**AC:**
1. Prepare problem.json in sandbox workspace
2. Spawn Pi Agent inside bwrap: `pi-agent solve --problem-file ... --model deepseek-v4-flash`
3. Pass DEEPSEEK_API_KEY from env
4. Monitor process, enforce timeout
5. Parse solution.md + evidence.md output
6. Store result in graph via internal/graph
7. Unit tests with mock bwrap executor
**Status:** ready

### [ ] WI-007: Idle cron loop
**Model:** ollama-cloud/minimax-m3
**Files:** internal/cron/loop.go (new), internal/cron/loop_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/cron/...`
**AC:**
1. Wake on configurable interval (default: check every 5m)
2. Check queue for pending problems
3. If queue non-empty: dequeue → sandbox → solve → store
4. Skip if system load > threshold (configurable) — true "idle" detection
5. Concurrency limit: 1 solve at a time (single-machine)
6. Metrics: track solve count, success rate, avg solve time
7. Unit tests with mock queue + mock sandbox
**Status:** ready

## Phase 3: Web UI

### [ ] WI-008: Web UI shell + static serving
**Model:** ollama-cloud/minimax-m3
**Files:** web/index.html (new), web/css/style.css (new), web/js/app.js (new), internal/web/serve.go (new)
**Verify:** `go build ./... && curl -s localhost:8766/ | grep -q "Off-by-One"`
**AC:**
1. Single-page HTML shell with nav tabs: Search, Submit, Explore, Export, Import
2. Embed HTML/CSS/JS via go:embed
3. Serve at GET / (root)
4. HTMX for dynamic content loading (no React/Vue — keep it light)
5. Responsive layout: main content area + chat sidebar
6. D3.js loaded for graph visualization (Explore view)
**Status:** ready

### [ ] WI-009: Web UI — Search view
**Model:** ollama-cloud/minimax-m3
**Files:** web/js/search.js (new), web/css/style.css (modify)
**Verify:** `curl -s localhost:8766/ | grep "search"`
**AC:**
1. Search bar with debounced input → API call GET /api/v1/problems?q=...
2. Results list: problem class name, description, answer count, hit count, status badge
3. Filter chips: environment, language, status
4. Click result → expand full solution inline (GET /api/v1/problems/{class}/answers/{id})
5. Related problems shown as D3.js force graph
6. Version warnings highlighted in yellow
**Status:** ready

### [ ] WI-010: Web UI — Submit view
**Model:** ollama-cloud/minimax-m3
**Files:** web/js/submit.js (new)
**Verify:** `go build ./...`
**AC:**
1. Form: problem class, environment, language, version, description, error message
2. Cadence selector: pre-phase, end-of-day, post-debug
3. Submit → POST /api/v1/problems/submit → show queue position + estimated time
4. Auto-suggest existing problem classes as user types (from GET /api/v1/taxonomy)
**Status:** ready

### [ ] WI-011: Web UI — Explore view (taxonomy browser)
**Model:** ollama-cloud/minimax-m3
**Files:** web/js/explore.js (new)
**Verify:** `go build ./...`
**AC:**
1. Tree view of problem taxonomy from GET /api/v1/taxonomy
2. Expandable nodes: problem class → environment → language → version → answer
3. Click node to see details
4. D3.js force graph for related problems when viewing a problem class
**Status:** ready

### [ ] WI-012: Web UI — Export/Import views
**Model:** ollama-cloud/minimax-m3
**Files:** web/js/export.js (new), web/js/import.js (new)
**Verify:** `go build ./...`
**AC:**
1. Export: select answers via checkboxes → choose target repo → preview → push
2. Import: enter repo URL → preview incoming answers (diff against local) → select → import
3. Show conflict resolution UI (same class+version, different answer)
4. Progress indicators for git operations
**Status:** ready

### [ ] WI-013: Web UI — AI Agent Chat
**Model:** ollama-cloud/minimax-m3
**Files:** web/js/chat.js (new), internal/web/chat.go (new), internal/web/chat_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/web/...`
**AC:**
1. Chat panel in sidebar (collapsible)
2. WebSocket connection to /ws/chat
3. Server relays messages to Pi Agent (spawned via bwrap)
4. Pi Agent has access to graph.search() — can look up answers while chatting
5. Pi Agent can suggest submit_problem() — server handles the submission
6. Chat history scrollable, markdown rendering
7. Unit tests for WebSocket handler with mock Pi Agent
**Status:** ready

## Phase 4: Export/Import

### [ ] WI-014: Git export engine
**Model:** ollama-cloud/minimax-m3
**Files:** internal/export/git.go (new), internal/export/git_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/export/...`
**AC:**
1. Clone/pull target git repo
2. Generate subtree files in pre-solve-answers/{class}/{env}/{version}/
3. Format as solution.md + evidence.md + signatures.json per template §5.1
4. Commit and push with descriptive message
5. Return commit SHA + PR URL (if branch)
6. Handle auth: SSH key or token from config
7. Unit tests with in-memory git repo (go-git or git init --bare)
**Status:** ready

### [ ] WI-015: Git import engine
**Model:** ollama-cloud/minimax-m3
**Files:** internal/import/git.go (new), internal/import/git_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/import/...`
**AC:**
1. Clone/pull source git repo
2. Parse pre-solve-answers/ directory tree
3. Diff against local graph: new, updated, conflict
4. Insert selected answers into local SQLite
5. Handle merge conflicts: same class+version with different answer
6. Return import summary (added, updated, skipped, conflicted)
7. Unit tests with mock repos
**Status:** ready

## Phase 5: Integration + Polish

### [ ] WI-016: Main binary wiring
**Model:** ollama-cloud/minimax-m3
**Files:** cmd/off-by-one/main.go (rewrite)
**Verify:** `go build ./... && go test -short -count=1 ./...`
**AC:**
1. Wire all components: API server + queue + sandbox + solver + cron + web UI
2. Parse config: port, db path, bwrap path, git repos, API keys from env
3. Graceful shutdown (SIGINT/SIGTERM → stop cron, close DB, kill sandboxes)
4. Health check endpoint: GET /health
5. `--version` flag
**Status:** ready

### [ ] WI-017: FTS5 full-text search
**Model:** ollama-cloud/minimax-m3
**Files:** internal/graph/search.go (new), internal/graph/search_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/graph/...`
**AC:**
1. SQLite FTS5 virtual table on problem_classes (title, description) + answer_nodes (solution)
2. Search across problem classes and answer content
3. Ranked results with snippets (highlight matching terms)
4. Filters: env, lang, status
5. Pagination: limit + offset
**Status:** ready

---

## Task Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| 1. Core | WI-001–004 | OpenAPI, graph, queue, API server |
| 2. Sandbox | WI-005–007 | bwrap, Pi Agent, idle cron |
| 3. Web UI | WI-008–013 | Shell, search, submit, explore, export/import, chat |
| 4. Git | WI-014–015 | Export, import |
| 5. Polish | WI-016–017 | Wiring, FTS5 |

**Total:** 17 tasks
**Target model:** MiniMax M3 via ollama-cloud
**Verification:** `go build ./... && go test -short -count=1 ./...` on every task
**Quality gate:** GitReins guard must pass before every commit
