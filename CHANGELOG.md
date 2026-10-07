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

**v0.2.0 version-prep complete (2026-10-07):** the `## [v0.2.0]` section below
is written and `cmd/off-by-one/main.go` carries the 0.2.0-dev version string.
Remaining gate before `make release TAG=v0.2.0`: owner authorization only —
no open P0/P1 board blockers (DF-OFF-BY-ONE-29/30/31/32 all closed with live
deploy evidence).

## [v0.2.0] - 2026-10-07

### Resource safety: the solver can no longer take the host down

- Per-solve memory cap: `oby-memcap` exec wrapper pins RLIMIT_AS inside the
  sandbox chain; the daemon grows a `-solve-mem-mb` flag and the shipped
  systemd unit carries durable caps (DF-OFF-BY-ONE-30) — a generated
  solution previously reached 47 GB and filled swap
- Near-cap solve rejection + a solver bounds contract so sim-style
  solutions cannot allocate unbounded per-event slices (DF-OFF-BY-ONE-31)
- Idle gate restored to the shipped unit: `-load-threshold 4` replaces the
  always-solve `-1`, making the lab opportunistic again (DF-OFF-BY-ONE-29)
- Host-aware problem feeder with per-solve observability: the 4x/day pump
  skips under host pressure and every solve is measured (DF-OFF-BY-ONE-32)

### Web chat: the conversation finally talks back

- WebSocket chat no longer dies on client-close races and answers past the
  old 30s read deadline: status/progress frames eliminate the silent 74s
  gap (DF-OFF-BY-ONE-16 + judge finding fix)

### Privacy and corpus hygiene

- Common PII (emails, phone numbers, hostnames) is redacted from shared
  answers by default (feat(privacy))
- Internal project/tool names and operator host paths scrubbed from the
  published answer corpus; env/version tokens normalized
- Static distribution site regenerated after months of freeze
  (DF-OFF-BY-ONE-12)

### API and behavior fixes

- Export/import round-trip protects community edits with conflict strategy
  honored in the import path and import details exposed (DF-OFF-BY-ONE-22,
  DF-OFF-BY-ONE-24)
- Embedded horizontal rules in corpus answers no longer truncate solution
  sections (DF-OFF-BY-ONE-23)
- Empty collections serialize as `[]`, never `null` (queue list, related
  problems; OB-GAP-093, REVIEW-OB-008)
- HTTP server binds to loopback by default (REVIEW-OB-007)
- Placeholder probe regexes anchored — substring matches no longer 404 real
  problem classes (DF-OFF-BY-ONE-15)
- MusterFlow named as the supported consumer; connect-muster exits non-zero
  with a remedy when Muster is missing (DF-OFF-BY-ONE-17)
- Empty-queue endpoint returns an honest error shape (DF-OFF-BY-ONE-27)
- 32 golangci-lint issues resolved (errcheck + staticcheck)
- Memcap ENOMEM test made environment-independent (INT-CI-003)
- Script hygiene: unique tmp paths; connect-muster honors SERVER_URL port
  (DF-OFF-BY-ONE-13, DF-OFF-BY-ONE-18)

### Docs

- WebSocket chat frame protocol and ping cadence documented; q= documented
  as a single FTS5 phrase (DF-OFF-BY-ONE-20)
- README refreshed against live state (release install, CI, watchdog,
  static mirror); community-node bootstrap documented as first-class flow
  (DF-OFF-BY-ONE-25)

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
