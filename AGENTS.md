# Off-by-One — AGENTS.md

An AI-powered pre-solve lab that converts idle compute cycles into pre-verified answers. Named as a nod to Stack Overflow — answers exist one step ahead of errors.

## Architecture

Go monorepo with SQLite graph database.

| Component | Role | Package |
|-----------|------|---------|
| `cmd/off-by-one` | Main binary | `main` |
| `internal/ingest` | Muster polling + problem queue | `ingest` |
| `internal/sandbox` | Bubblewrap (bwrap) namespace isolation | `sandbox` |
| `internal/solver` | Pi Agent integration for solving | `solver` |
| `internal/graph` | SQLite graph store (problem-class tree) | `graph` |
| `internal/export` | Git subtree export/import | `export` |
| `pkg/api` | OpenAPI spec for Muster auto-config | `api` |

Key files:
- `sql/schema/schema.sql` — Database schema
- `.env` — DEEPSEEK_API_KEY, OPENROUTER_API_KEY
- `go.mod` — Go module definition

## GitReins Quality Harness (MANDATORY)

Every commit runs static guards. If guards fail, the commit is BLOCKED.

```bash
PATH="$HOME/gitreins-poc/.venv/bin:$PATH" gitreins guard
```

What's checked:
- **secrets** — API keys, tokens (BLOCKS on fail)
- **build** — `go build ./...` (BLOCKS on fail)
- **lint** — `go vet ./...` (WARNS on fail)
- **tests** — `go test ./...` (BLOCKS on fail)

### Never:
- Commit API keys or tokens
- Skip guards with `--no-verify` for code changes
- Push if guards failed
