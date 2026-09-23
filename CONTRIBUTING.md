# Contributing to Off-by-One

## Quick Start

```bash
go build ./...
go test -short -count=1 ./...
go vet ./...
```

## Quality Gates

All commits must pass GitReins static guards:

```bash
gitreins guard
```

`gitreins` resolves through the pipx shim at `~/.local/bin/gitreins` — do NOT
prefix PATH with a repo venv (the old `PATH="$HOME/gitreins-poc/.venv/bin:$PATH"`
recipe names a directory that no longer exists, and a stale venv first on PATH
can shadow the repo interpreter and falsely FAIL the tests lane). The tests
lane is interpreter-pinned in `.gitreins/config.yaml`.

| Gate | Severity |
|------|----------|
| secrets | BLOCK |
| build | BLOCK |
| lint | WARN |
| tests | BLOCK |

## Architecture

See [AGENTS.md](AGENTS.md) for the full architecture overview.

## Testing

- Unit tests: `go test -short -count=1 ./...`
- Integration: E2E self-dogfood loop runs every foreman tick

## Pull Requests

1. Create a feature branch
2. Write tests for new functionality
3. Ensure all guards pass
4. Request review via the foreman pipeline
