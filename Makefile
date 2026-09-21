.PHONY: build test test-short check-binary-fresh check-deploy gate-deploy check-deploy-test connect-muster transport-retry-selftest pi-agent-watchdog-selftest clean

# Off-by-One Makefile
# Build, test, and Muster integration targets.

BINARY := off-by-one
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/off-by-one

# Guard for the deployed host artifact: ./off-by-one is gitignored but is what
# systemd runs. Fails (exit 1, "run make build") whenever the on-disk artifact
# was not produced by a clean `make build` at a revision whose code paths match
# HEAD.
#
# Three outcomes, in order (do NOT re-introduce the old unconditional
# `sed 's/-dirty$$//'` strip — it is what let a dirty-tree build pass):
#   1. DIRTY stamp (`<rev>-dirty`) — the artifact was compiled from an
#      uncommitted working tree, so it can contain code that exists nowhere in
#      the repo. Fail fast, naming the remedy ("built from a dirty tree"). The
#      marker is detected BEFORE any stripping, so a dirty build can never reach
#      the code-path shortcut and be reported "up to date".
#   2. NO VERSION STAMP — the stamp names no resolvable commit (e.g. the default
#      `0.1.0-dev` from a bare `go build` with no -ldflags). There is nothing to
#      diff against, so fail naming the missing stamp instead of misattributing
#      the failure to source drift.
#   3. RESOLVABLE stamp — the baked-in revision resolves to a real commit: PASS
#      when the code paths are unchanged since it (version-stamp-only changes
#      from data commits are expected), otherwise fall through to the
#      byte-for-byte rebuild comparison and fail with the "is stale — source
#      changed since it was built" message.
#
# Because LDFLAGS embeds `git describe`, data-only commits (e.g. corpus syncs in
# data/) change the binary's version stamp and make a byte-for-byte comparison
# falsely trip. We therefore parse the revision baked into the binary and first
# diff only the code paths against HEAD. Empty diff on code paths → PASS even
# though the bytes differ. If the baked-in revision is unparseable, fall back
# to the original deterministic byte comparison.
check-binary-fresh:
	@if [ ! -f $(BINARY) ]; then \
		echo "ERROR: ./$(BINARY) is missing — run 'make build'"; \
		exit 1; \
	fi
	@raw_version=$$(./$(BINARY) --version 2>/dev/null); \
	stamp=$$(echo "$$raw_version" | awk '{print $$NF}'); \
	case "$$stamp" in \
		*-dirty) \
			echo "ERROR: ./$(BINARY) was built from a dirty tree (stamp: $$stamp) — the working tree had uncommitted changes when it was compiled; commit or stash them, then run 'make build'"; \
			exit 1 ;; \
	esac; \
	case "$$stamp" in \
		*-*-g?*|*-g?*) candidate=$$(echo "$$stamp" | sed 's/.*-g//') ;; \
		*-*-?*)        candidate=$$(echo "$$stamp" | awk -F- '{print $$NF}') ;; \
		*)             candidate="$$stamp" ;; \
	esac; \
	resolved=no; \
	if git rev-parse --verify --quiet "$$candidate^{commit}" >/dev/null 2>&1; then \
		resolved=yes; \
		rev=$$(git rev-parse --short "$$candidate"); \
		changed=$$(git diff --name-only "$$rev" -- cmd/ internal/ web/ sql/ pkg/ go.mod go.sum Makefile); \
		if [ -z "$$changed" ]; then \
			echo "./$(BINARY) is up to date with source (version stamp changed, but code paths are unchanged)"; \
			exit 0; \
		fi; \
	fi; \
	if [ "$$resolved" = no ]; then \
		case "$$candidate" in \
			*[!0-9a-f]*) sha_shaped=no ;; \
			*) if [ "$${#candidate}" -ge 7 ]; then sha_shaped=yes; else sha_shaped=no; fi ;; \
		esac; \
		if [ "$$sha_shaped" = no ]; then \
			echo "ERROR: ./$(BINARY) carries no version stamp ($$stamp) — it was not built by 'make build'; run 'make build'"; \
			exit 1; \
		fi; \
	fi; \
	tmp=$$(mktemp); \
	trap 'rm -f "$$tmp"' EXIT; \
	go build -ldflags "$(LDFLAGS)" -o "$$tmp" ./cmd/off-by-one; \
	if ! cmp -s "$$tmp" $(BINARY); then \
		echo "ERROR: ./$(BINARY) is stale — source changed since it was built; run 'make build'"; \
		exit 1; \
	fi; \
	echo "./$(BINARY) is up to date with source"

# One-command deploy check (OB-GAP-077): FAILS with a named remedy whenever the
# RUNNING off-by-one.service does not serve the code at HEAD. Chains
# check-binary-fresh, verifies /proc/<MainPID>/exe points at the repo artifact,
# and resolves the running process's --version stamp against HEAD's code paths
# (data-only drift tolerated). Run after any code commit on master.
check-deploy:
	./scripts/check-deploy

# TICK CLOSE-OUT GATE (OB-GAP-085): the enforced entry point. Runs the
# check-deploy probe above and FAILS this make invocation on any leg failure, so
# a stale deployed artifact cannot survive a tick silently. It additionally
# refuses to report on a deployment it does not own: run from a git worktree (or
# a host without the unit) it SKIPs loudly instead of passing red herrings back
# to the live service. `make gate-deploy` is the documented step at tick
# close-out after ANY Go-source / go.mod / go.sum commit — see
# docs/dogfood/diagnostics.md ("Tick close-out enforcement") and README.
#
# Remedy when it fails: commit/stash board+gitreins state (a dirty tree stamps
# '<rev>-dirty', which check-binary-fresh refuses by design), `make build`, then
# `kill <MainPID>` — systemd Restart=always relaunches the service.
gate-deploy:
	./scripts/gate-deploy

# Regression self-test for the deploy gate + probe: hermetic scratch clones,
# stub systemd unit and stub systemctl — it never touches the live service.
# Covers gate-red (stale artifact) → gate-green (rebuilt), the unstamped-artifact
# failure, the unit-bound-elsewhere refusal, the not-a-checkout refusal, and the
# live-checkout SKIP verdict.
check-deploy-test:
	bash scripts/check-deploy-test.sh

# RELEASE-OB-002 — cut a tagged release. Minimal make+git tooling: no
# goreleaser, by convention.
#
# Usage: make release TAG=v0.1.0
#
# Gates, in order:
#   1. TAG must be set and match v<semver> (vMAJOR.MINOR.PATCH, digits only).
#   2. The tag must not already exist.
#   3. The tree must be clean (`git status --porcelain` empty — files under
#      .gitignore do not count).
#   4. CHANGELOG.md must have a `## [<TAG>]` section (or `## [<TAG>] - date`);
#      its body becomes the annotated tag message.
#   5. Build + FULL test suite must pass BEFORE anything is tagged; any failure
#      aborts with no tag created.
#
# DRY_RUN=1 runs every gate, prints each step (including the exact tag command
# and the push command for the human), and stops short of creating the tag —
# the verification mode; a green DRY_RUN run must print the push command.
#
# NOTE: TAG is intentionally read from the recipe environment (`$$TAG`), never
# via `$(TAG)` interpolation — an interpolated TAG is unquoted make text that
# a malicious/typo'd value could turn into arbitrary shell execution.
release:
	@if [ -z "$${TAG:-}" ]; then \
		echo "ERROR: TAG is required — usage: make release TAG=v0.1.0"; \
		exit 1; \
	fi
	@case "$$TAG" in \
		v[0-9]*.[0-9]*.[0-9]*) \
			ok=1 ;; \
		*) \
			echo "ERROR: TAG '$$TAG' is not a valid release tag — expected v<semver> (e.g. v0.1.0)"; \
			exit 1 ;; \
	esac
	@if git rev-parse -q --verify "refs/tags/$$TAG" >/dev/null 2>&1; then \
		echo "ERROR: tag $$TAG already exists — delete it or pick a new version"; \
		exit 1; \
	fi
	@if [ -n "$$(git status --porcelain)" ]; then \
		echo "ERROR: working tree is dirty — commit or clean it before releasing (git status --porcelain must be empty)"; \
		exit 1; \
	fi
	@version_body=$$(echo "$$TAG" | sed 's/^v//'); \
	section_start=$$(grep -n "^## \[$${TAG}\]" CHANGELOG.md | head -1 | cut -d: -f1); \
	if [ -z "$$section_start" ]; then \
		echo "ERROR: no CHANGELOG.md section '## [$$TAG]' found — add one before tagging"; \
		exit 1; \
	fi; \
	next_header=$$(tail -n +$$((section_start + 1)) CHANGELOG.md | grep -n '^## ' | head -1 | cut -d: -f1); \
	if [ -n "$$next_header" ]; then \
		section_end=$$((section_start + next_header - 1)); \
	else \
		section_end=$$(wc -l < CHANGELOG.md); \
	fi; \
	tag_msg=$$(sed -n "$$((section_start + 1)),$$section_end p" CHANGELOG.md | sed -e '/^[[:space:]]*$$/d' | head -40); \
	if [ -z "$$(echo "$$tag_msg" | tr -d '[:space:]')" ]; then \
		echo "ERROR: CHANGELOG section '## [$$TAG]' has no body to use as the tag message"; \
		exit 1; \
	fi; \
	echo "== release gates passed for $$TAG =="; \
	echo "tag message (from CHANGELOG):"; \
	echo "$$tag_msg"; \
	echo ""; \
	if [ "$${DRY_RUN:-0}" = "1" ]; then \
		echo "DRY_RUN=1 — stopping before build/test and tag creation."; \
		echo "Would run: go build && go test ./... (full)"; \
		echo "Would run: git tag -a $$TAG -m <CHANGELOG section for $$TAG>"; \
		echo "Next: git push origin $$TAG"; \
		exit 0; \
	fi; \
	echo "== running go build + full test suite =="; \
	go build ./... || exit 1; \
	go test -count=1 ./... || exit 1; \
	git tag -a "$$TAG" -m "$$tag_msg" || exit 1; \
	echo "Created annotated tag $$TAG."; \
	echo "Next: git push origin $$TAG"

test:
	go test -count=1 ./...

test-short:
	go test -short -count=1 ./...

# Regression self-test for the public-catalog publish transport
# (scripts/lib/transport-retry.sh + scripts/publish-catalog.sh). Deterministic:
# PATH shims + temp dirs, no network, no credentials, no real host.
transport-retry-selftest:
	bash scripts/tests/transport-retry-selftest.sh

# Regression self-test for the pi-agent health watchdog
# (scripts/pi-agent-watchdog.sh, OB-GAP-078). Deterministic: temp fixture
# workspaces + PI_DIR/stamp/wrapper env overrides, no network, no credentials,
# no contact with the real /tmp/pi install.
pi-agent-watchdog-selftest:
	bash scripts/tests/pi-agent-watchdog-selftest.sh

lint:
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./... ; \
	else \
		echo "golangci-lint not installed, skipping" ; \
	fi

connect-muster:
	bash scripts/connect-muster.sh

connect-muster-dry:
	bash scripts/connect-muster.sh --dry-run

clean:
	rm -f $(BINARY)
