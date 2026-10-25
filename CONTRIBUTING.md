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
ensure `gitreins` is installed (via pipx: `pipx install gitreins`)
recipe names a directory that no longer exists, and a stale venv first on PATH
can shadow the repo interpreter and falsely FAIL the tests lane). The tests
lane is interpreter-pinned in `.gitreins/config.yaml`.

| Gate | Severity |
|------|----------|
| secrets | BLOCK |
| build | BLOCK |
| lint | WARN |
| tests | BLOCK |

## Git hooks

After cloning, run:

```bash
make install-hooks
```

This installs the tracked pre-commit hook (`scripts/hooks/pre-commit` —
GitReins guard plus fail-closed board-lint: duplicate board ids and dangling
`depends_on` reject the commit) into the hooks directory git actually
consults. If a global/system `core.hooksPath` would otherwise shadow the
repo-local hook, the installer sets a repo-local `core.hooksPath` override
(it only writes this repo's `.git/config`; global/user config is never
touched). Re-run any time; the installer is idempotent.

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
