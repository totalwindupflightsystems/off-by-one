.PHONY: build test test-short check-binary-fresh connect-muster clean

# Off-by-One Makefile
# Build, test, and Muster integration targets.

BINARY := off-by-one
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/off-by-one

# Guard for the deployed host artifact: ./off-by-one is gitignored but is what
# systemd runs. Fails (exit 1, "run make build") whenever the on-disk binary
# does not byte-match a fresh build of the current source. Go builds are
# deterministic for identical source + toolchain + flags, so cmp is meaningful;
# the fresh build uses the same LDFLAGS as `make build`.
check-binary-fresh:
	@if [ ! -f $(BINARY) ]; then \
		echo "ERROR: ./$(BINARY) is missing — run 'make build'"; \
		exit 1; \
	fi
	@tmp=$$(mktemp); \
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
