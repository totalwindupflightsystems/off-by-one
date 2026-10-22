# Verdict: REVIEW-OB-021

**Task:** Fix SPA version chip: /health returns no version field, UI ships v0.0.0
**Evaluated:** 2026-10-22T00:19:58.319049
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✗ **tier2**
  - INCOMPLETE
  ✗ Tier1: build + vet + tests. Tier2: (1) GET /health returns JSON with a 'version' field that matches main.go's version variable, (2) web/js/app.js reads version from /health or X-Off-by-One-Version header, (3) web/index.html footer renders the version (not hardcoded v0.0.0), (4) a test exercises the /health response includes 'version': Tier1 PASS: `go build ./...` exit 0 (no output); `go vet ./...` exit 0 (no output); `go test ./internal/api/... ./cmd/... -short -count=1 -p 1` -> `ok internal/api 0.630s`, `ok cmd/off-by-one 2.581s`; `go test ./internal/web/... ./internal/sandbox/... ./internal/solver/...` -> all ok; `go test ./internal/... -short -count=1 -p 1` -> zero failures; `go test ./pkg/...` -> `ok pkg/api 0.025s`. Tier2(1) PASS: internal/api/server.go handleHealth emits `"version": s.Version`, wired at cmd/off-by-one/main.go:234 `apiServer.Version = version` (var at main.go:53); live proof with `-ldflags "-X main.version=0.2.1-review021"` -> `{"status":"ok","uptime":"7s","version":"0.2.1-review021"}`. Tier2(2) PASS: web/js/app.js:173-176 `fetch('/health')` then `elVersion.textContent = 'v' + j.version`. Tier2(4) PASS: internal/api/server_test.go TestHealthReportsVersion asserts `body["version"] == "9.9.9-test"`; `go test -run TestHealth -v` -> `--- PASS: TestHealthReportsVersion (0.00s)`. Tier2(3) FAIL: web/index.html:88 still hardcodes `<span class="status-version" id="status-version">v0.0.0</span>`; the served page from the freshly built binary returns that exact hardcoded line, and `grep -rn status-version --include=*.go` finds no server-side injection. Fix commit d2ff8d77 touched only cmd/off-by-one/main.go, internal/api/server.go, internal/api/server_test.go — `git show --name-only` confirms web/index.html and web/js/app.js were never modified, so the criterion's explicit 'not hardcoded v0.0.0' requirement is unmet (the JS only overwrites it at runtime, leaving the literal fallback in the shipped markup).


## Summary

Judge Result: REVIEW-OB-021

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: FAIL
  INCOMPLETE
  ✗ Tier1: build + vet + tests. Tier2: (1) GET /health returns JSON with a 'version' field that matches main.go's version variable, (2) web/js/app.js reads version from /health or X-Off-by-One-Version header, (3) web/index.html footer renders the version (not hardcoded v0.0.0), (4) a test exercises the /health response includes 'version': Tier1 PASS: `go build ./...` exit 0 (no output); `go vet ./...` exit 0 (no output); `go test ./internal/api/... ./cmd/... -short -count=1 -p 1` -> `ok internal/api 0.630s`, `ok cmd/off-by-one 2.581s`; `go test ./internal/web/... ./internal/sandbox/... ./internal/solver/...` -> all ok; `go test ./internal/... -short -count=1 -p 1` -> zero failures; `go test ./pkg/...` -> `ok pkg/api 0.025s`. Tier2(1) PASS: internal/api/server.go handleHealth emits `"version": s.Version`, wired at cmd/off-by-one/main.go:234 `apiServer.Version = version` (var at main.go:53); live proof with `-ldflags "-X main.version=0.2.1-review021"` -> `{"status":"ok","uptime":"7s","version":"0.2.1-review021"}`. Tier2(2) PASS: web/js/app.js:173-176 `fetch('/health')` then `elVersion.textContent = 'v' + j.version`. Tier2(4) PASS: internal/api/server_test.go TestHealthReportsVersion asserts `body["version"] == "9.9.9-test"`; `go test -run TestHealth -v` -> `--- PASS: TestHealthReportsVersion (0.00s)`. Tier2(3) FAIL: web/index.html:88 still hardcodes `<span class="status-version" id="status-version">v0.0.0</span>`; the served page from the freshly built binary returns that exact hardcoded line, and `grep -rn status-version --include=*.go` finds no server-side injection. Fix commit d2ff8d77 touched only cmd/off-by-one/main.go, internal/api/server.go, internal/api/server_test.go — `git show --name-only` confirms web/index.html and web/js/app.js were never modified, so the criterion's explicit 'not hardcoded v0.0.0' requirement is unmet (the JS only overwrites it at runtime, leaving the literal fallback in the shipped markup).


Overall: FAIL ✗
