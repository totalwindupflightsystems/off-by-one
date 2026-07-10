# Off-by-One — Implementation Tasks

> Auto-generated from specs/system-spec.md
> Model: MiniMax M3 via ollama-cloud
> Foreman model: deepseek-v4-pro (planning only)

## Phase 1: Core Infrastructure

### [x] WI-001: OpenAPI spec generation
**Model:** ollama-cloud/minimax-m3
**Files:** pkg/api/openapi.yaml (new)
**Verify:** `yamllint pkg/api/openapi.yaml && curl -s localhost:8766/openapi.json | python3 -m json.tool`
**AC:**
1. Create OpenAPI 3.0.3 spec at pkg/api/openapi.yaml with all endpoints from system spec §10
2. Embed spec via go:embed in cmd/off-by-one/main.go
3. Serve at GET /openapi.json
4. Muster can auto-configure from this endpoint
**Status:** done (df71f0c)

### [x] WI-002: SQLite graph engine
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
**Status:** done (ce2bf6e)

### [x] WI-003: Ingest queue
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
**Status:** done (24f0d81)

### [x] WI-004: HTTP API server
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
**Status:** done (2e65698)

## Phase 2: Sandbox + Solver

### [x] WI-005: Bubblewrap sandbox
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
**Status:** done (d3f75de)

### [x] WI-006: Pi Agent solver integration
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
**Status:** done (28a54a9)

### [x] WI-007: Idle cron loop
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
**Status:** done (889dbeb)

## Phase 3: Web UI

### [x] WI-008: Web UI shell + static serving
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
**Status:** done (817de90) — Note: AC#4 (HTMX) and AC#6 (D3.js) deferred to per-view tasks (WI-009+)

### [x] WI-009: Web UI — Search view
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
**Status:** done (f756cca)

### [x] WI-010: Web UI — Submit view
**Model:** ollama-cloud/minimax-m3
**Files:** web/js/submit.js (new)
**Verify:** `go build ./...`
**AC:**
1. Form: problem class, environment, language, version, description, error message
2. Cadence selector: pre-phase, end-of-day, post-debug
3. Submit → POST /api/v1/problems/submit → show queue position + estimated time
4. Auto-suggest existing problem classes as user types (from GET /api/v1/taxonomy)
**Status:** done (6ab466b)

### [x] WI-011: Web UI — Explore view (taxonomy browser)
**Model:** ollama-cloud/minimax-m3
**Files:** web/js/explore.js (new)
**Verify:** `go build ./...`
**AC:**
1. Tree view of problem taxonomy from GET /api/v1/taxonomy
2. Expandable nodes: problem class → environment → language → version → answer
3. Click node to see details
4. D3.js force graph for related problems when viewing a problem class
**Status:** done

### [x] WI-012: Web UI — Export/Import views
**Model:** ollama-cloud/minimax-m3
**Files:** web/js/export.js (new), web/js/import.js (new)
**Verify:** `go build ./...`
**AC:**
1. Export: select answers via checkboxes → choose target repo → preview → push
2. Import: enter repo URL → preview incoming answers (diff against local) → select → import
3. Show conflict resolution UI (same class+version, different answer)
4. Progress indicators for git operations
**Status:** done (425f6b7)

### [x] WI-013: Web UI — AI Agent Chat
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
**Status:** done (b9c8ac1)

## Phase 4: Export/Import

### [x] WI-014: Git export engine
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
**Status:** done (08896ba)

### [x] WI-015: Git import engine
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
**Status:** done (0be7f2f)

## Phase 5: Integration + Polish

### [x] WI-016: Main binary wiring
**Model:** ollama-cloud/minimax-m3
**Files:** cmd/off-by-one/main.go (rewrite)
**Verify:** `go build ./... && go test -short -count=1 ./...`
**AC:**
1. Wire all components: API server + queue + sandbox + solver + cron + web UI
2. Parse config: port, db path, bwrap path, git repos, API keys from env
3. Graceful shutdown (SIGINT/SIGTERM → stop cron, close DB, kill sandboxes)
4. Health check endpoint: GET /health
5. `--version` flag
**Status:** done (c88ecc0)

### [x] WI-017: FTS5 full-text search
**Model:** ollama-cloud/minimax-m3
**Files:** internal/graph/search.go (new), internal/graph/search_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/graph/...`
**AC:**
1. SQLite FTS5 virtual table on problem_classes (title, description) + answer_nodes (solution)
2. Search across problem classes and answer content
3. Ranked results with snippets (highlight matching terms)
4. Filters: env, lang, status
5. Pagination: limit + offset
**Status:** done (08443d2)

## Phase 6: Muster Integration

### [x] WI-018: Muster MCP bridge — end-to-end wiring
**Model:** ollama-cloud/minimax-m3
**Files:** internal/muster/bridge.go (new), internal/muster/bridge_test.go (new), scripts/connect-muster.sh (new), muster-config.yaml (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/muster/... && bash scripts/connect-muster.sh --dry-run`
**AC:**
1. Create muster-config.yaml pointing Muster at Off-by-One's `/openapi.json` endpoint (default: `http://localhost:8766/openapi.json`)
2. Script `scripts/connect-muster.sh` that:
   a. Starts Off-by-One server if not running
   b. Verifies `/openapi.json` returns valid OpenAPI spec
   c. Starts Muster MCP server with `--host http://localhost:8766`
   d. Verifies MCP tools are generated (submit_problem, discover_solution, list_problems, get_queue_status)
3. Bridge module `internal/muster/bridge.go` that:
   a. Validates the OpenAPI spec is Muster-compatible (operationId on every path, valid requestBody schemas)
   b. Provides health check: is Muster connected? are MCP tools live?
   c. Logs MCP tool calls for debugging
4. Integration test: spawn Off-by-One → spawn Muster → submit problem via MCP tool → verify it appears in Off-by-One queue → discover solution → verify response
5. `make connect-muster` target that runs the connect script
6. Update AGENTS.md with Muster integration architecture
**Status:** done (6e20d10)

### [x] WI-019: Muster MCP — bidirectional verification
**Model:** ollama-cloud/minimax-m3
**Files:** internal/muster/e2e_test.go (new)
**Verify:** `go test -short -count=1 -run TestMusterE2E ./internal/muster/...`
**AC:**
1. Full E2E test: Muster client → submit_problem → Off-by-One queue → sandbox solve → graph store → discover_solution → verify answer returned
2. Test error paths: submit duplicate problem (expect dedup), discover nonexistent problem (expect not found), submit invalid cadence (expect validation error)
3. Test queue lifecycle: pending → in_progress → complete
4. Test Muster reconnection: kill Muster → restart → verify tools still work
5. All tests pass with `go test -short` (skip long-running solve in short mode)
**Status:** done (fad216a)

## Phase 7: Spec Gap Fixes

### [x] WI-020: Wire export/import API endpoints

### [x] WI-021: Solver execution hardening
**Model:** ollama-cloud/minimax-m3 (foreman direct-write)
**Files:** cmd/off-by-one/main.go, internal/sandbox/bwrap.go, internal/sandbox/bwrap_test.go, internal/solver/piagent.go, internal/solver/piagent_test.go, internal/cron/loop.go, internal/cron/loop_test.go
**Verify:** `go build ./... && go test -short -count=1 ./...`
**AC:**
1. --share-net in bwrap so pi-agent reaches DeepSeek API from sandbox
2. ExtraReadOnlyPaths on Executor + Config for pi-agent tool paths (/home/kara/.local/bin, /tmp/pi, /etc)
3. Commit(ctx, sub, sol) — solver uses submission's problem_class, falls back to extracted
4. Answer status: Pending → Verified (validator ring deferred to post-MVP)
5. All tests pass, gitreins guard PASS
**Status:** done (26cd445)
**Model:** ollama-cloud/minimax-m3 (foreman direct-write)
**Files:** internal/api/server.go (modify), internal/api/handlers.go (modify), internal/api/handlers_test.go (modify), cmd/off-by-one/main.go (modify)
**Verify:** `go build ./... && go test -short -count=1 ./internal/api/...`
**AC:**
1. Register `POST /api/v1/export` route in Server.Handler() — matches OpenAPI spec §10
2. Register `POST /api/v1/import` route in Server.Handler() — matches OpenAPI spec §10
3. Export handler: parse ExportRequest JSON (target_repo, answer_ids, branch, commit_message) → construct export.Engine → call Export() → return ExportResponse JSON (commit_sha, pr_url, files_changed)
4. Import handler: parse ImportRequest JSON (source_repo, branch, conflict_strategy) → construct import.Engine → call Import() → return ImportResponse JSON (added, updated, skipped, conflicted)
5. Error handling: 400 for bad request body, 500 for engine errors, 501 if export/import not configured
6. Server struct gains optional ExportLocalDir and ImportLocalDir fields for the working clone directories
7. Main binary wires the local dirs from env vars (OFF_BY_ONE_EXPORT_DIR, OFF_BY_ONE_IMPORT_DIR) or defaults to temp dirs
8. Integration tests: httptest POST /api/v1/export with mock items, POST /api/v1/import with mock repo
**Status:** done (cc5fde4)

---

## Phase 8: Discovery Sweep

> Tasks discovered during 2026-07-09 empty-board sweep. Build+test+endpoints all green.

### [x] DS-001: Add CI workflow (GitHub Actions)
**Priority:** high
**Files:** .github/workflows/ci.yml (new)
**Verify:** `gh run list --limit 1 --json status,conclusion` shows a run
**AC:**
1. Create `.github/workflows/ci.yml` with Go build + test + vet on push to main
2. Run `go build ./...`, `go vet ./...`, `go test -short -count=1 ./...`
3. Cache Go modules
4. Matrix: Go 1.25, 1.26
**Status:** done (9df77f3)

### [x] DS-002: Expand README with architecture + API reference
**Priority:** medium
**Files:** README.md (modify)
**Verify:** `wc -l README.md` shows >100 lines
**AC:**
1. Add architecture diagram (mermaid or ASCII)
2. Add API endpoint reference table (all 14 endpoints from OpenAPI spec)
3. Add configuration reference (env vars, flags, defaults)
4. Add development guide (build, test, lint, guard)
**Status:** done (d26322c)

### [x] DS-003: Semantic similarity scoring via OpenRouter embeddings
**Priority:** low
**Files:** internal/graph/embeddings.go (new), internal/graph/embeddings_test.go (new)
**Verify:** `go build ./... && go test -short -count=1 ./internal/graph/...`
**AC:**
1. Call OpenRouter embeddings API for problem class descriptions
2. Store embedding vectors in SQLite (new column or separate table)
3. Cosine similarity ranking during discovery (spec §7.3)
4. Unit tests with mock HTTP server
**Status:** done (1dd8642)

### [ ] DS-004: File attachment support for problem submissions
**Priority:** low
**Files:** internal/api/handlers.go (modify), web/js/submit.js (modify)
**Verify:** `go build ./... && go test -short -count=1 ./internal/api/...`
**AC:**
1. Accept multipart/form-data file uploads in POST /api/v1/problems/submit
2. Store attachments in sandbox workspace alongside problem.json
3. Pass file paths to Pi Agent via problem context
4. Unit tests with multipart writer

---

## Task Summary

| Phase | Tasks | Description |
|-------|-------|-------------|
| 1. Core | WI-001–004 | OpenAPI, graph, queue, API server |
| 2. Sandbox | WI-005–007 | bwrap, Pi Agent, idle cron |
| 3. Web UI | WI-008–013 | Shell, search, submit, explore, export/import, chat |
| 4. Git | WI-014–015 | Export, import |
| 5. Polish | WI-016–017 | Wiring, FTS5 |
| 6. Muster | WI-018–019 | Bridge, E2E verification |
| 7. Execution | WI-020–021 | API endpoints, solver hardening |
| 8. Discovery | DS-001–004 | CI, docs, embeddings, attachments |

**Total:** 25 tasks (21 done, 4 pending)
**Target model:** MiniMax M3 via ollama-cloud
**Verification:** `go build ./... && go test -short -count=1 ./...` on every task
**Quality gate:** GitReins guard must pass before every commit
