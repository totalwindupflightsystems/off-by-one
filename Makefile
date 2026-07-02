.PHONY: build test test-short connect-muster clean

# Off-by-One Makefile
# Build, test, and Muster integration targets.

BINARY := off-by-one
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "0.1.0-dev")
LDFLAGS := -X main.version=$(VERSION)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/off-by-one

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
