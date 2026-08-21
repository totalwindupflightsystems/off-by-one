.PHONY: build test test-short check-binary-fresh connect-muster clean

# Off-by-One Makefile
# Build, test, and Muster integration targets.

BINARY := off-by-one
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/off-by-one

# Guard for the deployed host artifact: ./off-by-one is gitignored but is what
# systemd runs. Fails (exit 1, "run make build") whenever code that affects the
# compiled binary has changed since the on-disk binary was built.
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
	built_rev=$$(echo "$$raw_version" | awk '{print $$NF}'); \
	built_rev=$$(echo "$$built_rev" | sed 's/-dirty$$//'); \
	case "$$built_rev" in \
		*-*-g?*|*-g?*) candidate=$$(echo "$$built_rev" | sed 's/.*-g//') ;; \
		*-*-?*)        candidate=$$(echo "$$built_rev" | awk -F- '{print $$NF}') ;; \
		*)             candidate="$$built_rev" ;; \
	esac; \
	if git rev-parse --verify --quiet "$$candidate^{commit}" >/dev/null 2>&1; then \
		rev=$$(git rev-parse --short "$$candidate"); \
		changed=$$(git diff --name-only "$$rev" -- cmd/ internal/ web/ sql/ pkg/ go.mod go.sum Makefile); \
		if [ -z "$$changed" ]; then \
			echo "./$(BINARY) is up to date with source (version stamp changed, but code paths are unchanged)"; \
			exit 0; \
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
