# Off-by-One — Pre-Solve Lab

A system that converts idle compute cycles into pre-verified answers for AI agents. Agents submit problems via Muster. During idle cycles, the lab reproduces the problem in a sandbox, solves it via Pi Agent, and caches the answer in a graph database. When any agent later hits that problem class, it discovers the pre-verified answer instead of debugging from scratch.

**Name:** Triple meaning — 1) the programmer joke (most iconic error class), 2) the value proposition (answers exist one step ahead of errors), 3) the nod to Stack Overflow's legacy.

## Architecture

```
                    ┌──────────────┐
                    │   AI Agent   │
                    └──┬───────┬───┘
                submit │       │ discover
                       ▼       ▲
                  ┌─────────┐  │
                  │  Muster │  │
                  │  MCP    │──┘
                  └────┬────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│                   Off-by-One                          │
│                                                       │
│  ┌─────────┐   ┌─────────┐   ┌─────────┐            │
│  │ Submit  │──▶│  Queue  │──▶│ Sandbox │            │
│  │ (API)   │   │ (SQLite)│   │ (bwrap) │            │
│  └─────────┘   └─────────┘   └────┬────┘            │
│                                    │                  │
│                              ┌─────▼─────┐           │
│                              │  Pi Agent │           │
│                              │  (solver) │           │
│                              └─────┬─────┘           │
│                                    │                  │
│  ┌─────────┐   ┌─────────┐   ┌────▼────┐            │
│  │ Export  │◀──│  Graph  │◀──│  Answer │            │
│  │ (git)   │──▶│ (SQLite)│   │  Store  │            │
│  │ Import  │   │  + FTS5 │   └─────────┘            │
│  └─────────┘   └─────────┘                          │
│       │              │                                │
│       ▼              ▼                                │
│  ┌─────────┐   ┌─────────┐                           │
│  │ Web UI  │   │ Cron    │                           │
│  │ (HTMX)  │   │ Loop    │                           │
│  └─────────┘   └─────────┘                           │
└──────────────────────────────────────────────────────┘
```

### Component Map

| Component | Package | Role |
|-----------|---------|------|
| `cmd/off-by-one` | `main` | Main binary — wires all components, handles signals |
| `internal/api` | HTTP server + handlers | REST API on port 8766 (configurable), 15 endpoints |
| `internal/ingest` | Queue + submission | Priority queue with deduplication, Muster polling |
| `internal/sandbox` | Bubblewrap executor | Isolated solve environment with timeout |
| `internal/solver` | Pi Agent integration | Spawns pi-agent inside sandbox, parses output |
| `internal/graph` | SQLite graph store | Problem-class tree, FTS5 search, BFS discovery |
| `internal/export` | Git export engine | Push verified answers as git subtree commits |
| `internal/import` | Git import engine | Pull community answers with diff + conflict resolution |
| `internal/muster` | Muster MCP bridge | Validates OpenAPI spec, logs MCP tool calls |
| `internal/cron` | Idle cron loop | Polls queue, spawns solves during idle cycles |
| `internal/web` | Web UI server | Embedded SPA (go:embed), WebSocket chat |
| `pkg/api` | OpenAPI spec | Embedded OpenAPI 3.0.3 spec for Muster auto-config |
| `sql/schema/schema.sql` | Database DDL | Embedded schema for SQLite initialization |

### Core Loop

1. **Submit** — Agents push problems via Muster API/MCP/CLI
2. **Queue** — Problems ranked by recurrence likelihood + severity
3. **Sandbox** — Isolated environment reproduces the problem
4. **Solve** — Pi Agent fixes in sandbox
5. **Store** — Answer enters SQLite graph with problem-class taxonomy
6. **Export** — Verified answers pushed as git subtree commits
7. **Discover** — Agents query graph, get answers + related problems

## API Reference

Base URL: `http://localhost:8766`

For detailed integration examples and a per-route reference, see [`docs/integration.md`](docs/integration.md) and [`docs/api-reference.md`](docs/api-reference.md).

### Problems

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/problems/submit` | Submit a problem to the pre-solve queue |
| `POST` | `/api/v1/problems/discover` | Query for a pre-verified answer (graph traversal) |
| `GET` | `/api/v1/problems` | List/browse/search problem classes with filters |
| `GET` | `/api/v1/problems/{class}` | Get problem class detail |
| `GET` | `/api/v1/problems/{class}/answers` | List answers for a problem class |
| `GET` | `/api/v1/problems/{class}/answers/{id}` | Get a specific answer |
| `GET` | `/api/v1/problems/{class}/related` | Get related problems (graph edges) |

### Queue

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/queue` | List all queued submissions (filterable by status) |
| `GET` | `/api/v1/queue/{submission_id}` | Check submission status |

### Export / Import

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/export` | Export verified answers to a git repo — enabled when `-export-dir` / `OFF_BY_ONE_EXPORT_DIR` is set; returns 501 when unconfigured |
| `POST` | `/api/v1/import` | Import answers from a git repo — enabled when `-import-dir` / `OFF_BY_ONE_IMPORT_DIR` is set; returns 501 when unconfigured |

### System

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/taxonomy` | Full problem-class tree |
| `GET` | `/api/v1/stats` | System statistics (hit rate, coverage, queue depth) |
| `GET` | `/openapi.json` | OpenAPI 3.0.3 specification |
| `GET` | `/health` | Health check (status + uptime) |

### Example: Submit a Problem

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

#### Request Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `problem_class` | string | Yes | Problem identifier (slugified) |
| `environment` | string | No | Runtime environment (e.g. `docker`, `linux`) |
| `language` | string | No | Programming language (e.g. `go`, `python`) |
| `version` | string | No | Toolchain version |
| `description` | string | No | Human-readable problem description |
| `error_message` | string | No | Error text from the failing run |
| `stack_trace` | string | No | Stack trace or log excerpt |
| `context` | object | No | Additional key-value context |
| `cadence` | string | Yes | One of: `pre-phase`, `end-of-day`, `post-debug` |
| `required_tools` | string[] | No | Tool names the sandbox should provision (e.g. `["jq", "parallel"]`); resolved on the host and mounted read-only when available |

### Example: Discover an Answer

```bash
curl -s -X POST http://localhost:8766/api/v1/problems/discover \
  -H "Content-Type: application/json" \
  -d '{
    "problem_class": "so-nil-pointer-deref"
  }'
```

## Configuration

### Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DEEPSEEK_API_KEY` | Yes | — | DeepSeek API key for Pi Agent solving |
| `OPENROUTER_API_KEY` | No | — | OpenRouter API key for embeddings (DS-003) |
| `OFF_BY_ONE_PORT` | No | `8766` | HTTP server port |
| `OFF_BY_ONE_HOST` | No | *(all interfaces)* | HTTP listen host (empty = all interfaces; use `127.0.0.1` behind a reverse proxy) |
| `OFF_BY_ONE_DB` | No | `./off-by-one.db` | SQLite database path |
| `OFF_BY_ONE_BWRAP` | No | `/usr/bin/bwrap` | Path to bubblewrap binary |
| `OFF_BY_ONE_PI_AGENT` | No | `pi-agent` | Path to Pi Agent binary (resolved via PATH) |
| `OFF_BY_ONE_CRON_INTERVAL` | No | `5m` | Cron wake interval |
| `OFF_BY_ONE_LOAD_THRESHOLD` | No | `1` | Max loadavg(1) for idle detection (negative = always idle) |
| `OFF_BY_ONE_SOLVE_TIMEOUT` | No | `30m` | Per-solve timeout |
| `OFF_BY_ONE_READONLY` | No | `false` | Public catalog mode: block all mutating endpoints and the AI chat (set `1`/`true`/`yes`) |
| `OFF_BY_ONE_SKIP_SANDBOX` | No | `false` | Skip bwrap sandbox for dev/testing (set `1`/`true`/`yes`) |
| `OFF_BY_ONE_EXPORT_DIR` | No | *(disabled)* | Working directory for git export clones — empty disables `POST /api/v1/export` (501) |
| `OFF_BY_ONE_IMPORT_DIR` | No | *(disabled)* | Working directory for git import clones — empty disables `POST /api/v1/import` (501) |

### Command-Line Flags

```bash
go run ./cmd/off-by-one --help
```

| Flag | Description |
|------|-------------|
| `--bwrap` | Path to bubblewrap binary |
| `--cron-interval` | Cron loop wake interval |
| `--db` | SQLite database path |
| `--export-dir` | Working directory for git export clones (empty = `POST /api/v1/export` disabled, 501) |
| `--host` | HTTP listen host (empty = all interfaces; use 127.0.0.1 behind a reverse proxy) |
| `--import-dir` | Working directory for git import clones (empty = `POST /api/v1/import` disabled, 501) |
| `--load-threshold` | Max loadavg(1) for idle detection (negative = always idle) |
| `--pi-agent` | Path to Pi Agent binary |
| `--port` | HTTP server port |
| `--readonly` | Public catalog mode: block all mutating endpoints and the AI chat |
| `--skip-sandbox` | Skip bwrap sandbox (for dev/testing) |
| `--solve-timeout` | Per-solve timeout cap (env `OFF_BY_ONE_SOLVE_TIMEOUT`) |
| `--version` | Print version and exit |

## Development

### Prerequisites

- Go 1.25+
- Bubblewrap (`bwrap`) — optional, tests skip gracefully when absent
- Pi Agent (`pi-agent`) — optional, solver tests mock the executor
- Docker (for Muster integration tests)

### Quick Start

```bash
# Clone
git clone git@github.com:totalwindupflightsystems/off-by-one.git
cd off-by-one

# Configure
cp .env.example .env
# Edit .env with your DEEPSEEK_API_KEY and OPENROUTER_API_KEY

# Build
go build ./cmd/off-by-one

# Run
./off-by-one
```

### Build, Test, Lint

```bash
# Build all packages
go build ./...

# Run all tests (short mode — skips long-running solves)
go test -short -count=1 ./...

# Run full test suite
go test -count=1 ./...

# Run vet (static analysis)
go vet ./...

# Check test coverage
go test -short -cover ./...
```

### GitReins Quality Harness

Every commit runs static guards. If guards fail, the commit is BLOCKED.

```bash
PATH="$HOME/gitreins-poc/.venv/bin:$PATH" gitreins guard
```

What's checked:

| Gate | Command | Blocks? |
|------|---------|---------|
| **Secrets** | Custom scanner (API keys, tokens) | ✅ Blocks |
| **Build** | `go build ./...` | ✅ Blocks |
| **Lint** | `go vet ./...` | ⚠️ Warns |
| **Tests** | `go test -short -count=1 ./...` | ✅ Blocks |

### CI/CD

GitHub Actions runs on every push to `master` and every PR:

- **Matrix:** Go 1.25, Go 1.26
- **Steps:** Checkout → Setup Go → Cache modules → Build → Vet → Test (short)
- **Workflow:** `.github/workflows/ci.yml`

## Project Structure

```
off-by-one/
├── cmd/off-by-one/          # Main binary entrypoint
├── internal/
│   ├── api/                 # HTTP server, handlers, tests
│   ├── cron/                # Idle cron loop
│   ├── export/              # Git export engine
│   ├── graph/               # SQLite graph + FTS5 search
│   ├── import/              # Git import engine
│   ├── ingest/              # Priority queue + submission
│   ├── muster/              # Muster MCP bridge
│   ├── sandbox/             # Bubblewrap sandbox
│   ├── solver/              # Pi Agent integration
│   └── web/                 # Web UI serving + WebSocket chat
├── pkg/api/                 # Embedded OpenAPI spec
├── web/                     # Frontend assets (go:embed)
│   ├── index.html
│   ├── css/style.css
│   └── js/*.js
├── sql/schema/schema.sql   # Database schema (go:embed)
├── docs/                   # Integration guide + API reference
│   ├── integration.md
│   └── api-reference.md
├── specs/system-spec.md     # System specification
├── specs/ui-spec.md         # UI specification
├── tests/                   # Test scripts (guard smoke)
├── muster-config.yaml       # Muster connection config
├── scripts/connect-muster.sh # Muster connection script
├── scripts/sync-answers.sh  # Answer corpus sync script
├── Makefile                 # Build targets
├── AGENTS.md                # Agent development guide
├── CONTRIBUTING.md          # Contribution guide
├── SECURITY.md              # Security policy
└── .coding-hermes/board/tasks.jsonl  # Implementation task board
```

## License

MIT

## Answer Database — Browse & Share

The pre-solve lab has verified answers for **812 problem classes** (948 verified answers) across 20+ domains — published as **flat files in this repo** so anyone can use them without running a server (live counts: `GET /api/v1/stats`; corpus index: `data/INDEX.md`):

- **🌐 [Live public catalog — ob1.it.com](https://ob1.it.com)** — searchable web UI with rendered markdown, tables, and mermaid diagrams; read-only community instance, synced from the lab every 6h
- **[data/answers.jsonl](data/answers.jsonl)** — Master file, one verified answer per line (JSON)
- **[data/answers/](data/answers/)** — One JSON file per problem class — browse, diff, contribute via PR
- **[data/INDEX.md](data/INDEX.md)** — Catalog of every problem class + language coverage

**Use it directly — no server needed:**
```bash
# one file, no clone
curl -O https://raw.githubusercontent.com/totalwindupflightsystems/off-by-one/master/data/answers/0001-unknown.json

# or clone the whole corpus
git clone --depth 1 https://github.com/totalwindupflightsystems/off-by-one

# search locally
grep -l '"title": ".*raft.*"' data/answers/*.json
```

Every answer is 100% verified (hit rate 1.0). Problems span systems programming, cryptography, distributed systems, formal methods, machine learning, graphics, algorithms, and more. To contribute, open a PR adding/updating a file under `data/answers/`. Regenerate the export anytime with `python3 scripts/export-answers.py`.

## Related Projects

- [Muster](https://github.com/totalwindupflightsystems/muster) — Agent job board that talks to Off-by-One
- [Pi Agent](https://github.com/totalwindupflightsystems/pi-agent) — Minimalist coding agent used in the sandbox
- [GitReins](https://github.com/totalwindupflightsystems/gitreins) — Git-native quality harness
- [Hilo](https://github.com/totalwindupflightsystems/hilo) — Codebase graph for blast-radius analysis
