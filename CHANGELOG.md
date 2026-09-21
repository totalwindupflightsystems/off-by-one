# Changelog

## [Unreleased]

### MVP snapshot — feature-complete pipeline, NOT yet tagged or shipped

**Release status:** the feature set below shipped to production hosts as a
running service, but no git tag, GitHub Release, or versioned artifact exists
yet (verified 2026-09-21: `git ls-remote --tags origin` and `gh release list`
are both empty). The first tagged cut is pending the release tooling work
tracked in the project board (RELEASE-OB-002). Nothing in this section should
be read as version 0.1.0 being published.

**Core Pipeline:**
- Submit problems via HTTP API (`POST /api/v1/problems/submit`)
- Auto-solve with Pi Agent inside Bubblewrap sandbox
- Discover solutions (`POST /api/v1/problems/discover`)
- SQLite graph database with FTS5 search
- OpenAPI 3.0.3 spec for Muster MCP auto-configuration

**Web UI:**
- Shell view (live stats, queue, taxonomy tree)
- Search view (full-text search across problems and answers)
- Submit view (human-facing problem submission)
- Explore view (graph traversal and answer history)
- Export/Import view (Git-based knowledge transfer)
- AI Agent Chat view (WebSocket-based agent interaction)

**Integrations:**
- Muster MCP bridge (bidirectional ingest and discovery)
- GitHub Actions CI (build, vet, test, govulncheck)
- GitReins guard (secrets, build, lint, tests)

**Language Support:**
- Shell/Bash problems
- Go problems
- Python problems
- JavaScript/Node.js problems

**Stats:** 24 problems solved, 29 verified answers, 11/11 test packages passing, 76.3% coverage.
