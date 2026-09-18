.PHONY: build test test-short check-binary-fresh connect-muster transport-retry-selftest clean

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

test:
	go test -count=1 ./...

test-short:
	go test -short -count=1 ./...

# Regression self-test for the public-catalog publish transport
# (scripts/lib/transport-retry.sh + scripts/publish-catalog.sh). Deterministic:
# PATH shims + temp dirs, no network, no credentials, no real host.
transport-retry-selftest:
	bash scripts/tests/transport-retry-selftest.sh

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
