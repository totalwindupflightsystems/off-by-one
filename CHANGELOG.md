# Changelog

## [Unreleased]

**Release status:** release tooling exists — `make release TAG=v<semver>`
(added in RELEASE-OB-002). It refuses a missing or non-semver TAG, an already
existing tag, a dirty tree, or a missing `## [<TAG>]` section in this file,
and runs the full build+test suite before creating the annotated tag with the
message taken from that section (`DRY_RUN=1` previews all gates without
tagging). v0.1.0 was the first cut with that tooling; v0.1.1 is the first cut
to ship with a GitHub Release object and artifacts (the release gate: a tag
without a Release object is not a release).

## [v0.1.1] - 2026-09-22

### Operational hardening (no API or behavior changes)

- CI honesty bundle: bwrap installed for sandbox tests, honest Go 1.26 matrix,
  grouped sqlite3 guard (OB-GAP-087/088/090), AppArmor unprivileged-userns
  restriction lifted (OB-GAP-091)
- Deploy: off-by-one systemd unit vendored in-repo with doc pointers and a
  drift guard (OB-GAP-089)
- Docs: gitreins guard recipe corrected in README/CONTRIBUTING (the deleted
  gitreins-poc venv path replaced by the pipx shim); AGENTS.md replacement
  text parked in docs/agents-harness-guard.md pending approval
- Hygiene: churning classes untracked (.gitreins/logs/, board *.pre-*
  snapshots) so `git status` stays clean
- Release surface: GitHub Release objects published for v0.1.0 (backfilled)
  and v0.1.1; answer-corpus data syncs continued through the cycle

## [v0.1.0] - 2026-09-21

### MVP snapshot — feature-complete pipeline

The feature set below shipped to production hosts as a running service;
v0.1.0 is its first tagged cut (verified 2026-09-21, before this cut: `git
ls-remote --tags origin` and `gh release list` were both empty).

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
