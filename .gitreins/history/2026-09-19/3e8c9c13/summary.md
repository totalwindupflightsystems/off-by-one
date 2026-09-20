# Verdict: OB-GAP-081

**Task:** Clamp or reject over-max problems limit
**Evaluated:** 2026-09-19T18:17:21.294811
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	1.822s
- ✓ **tier2**
  - COMPLETE
  ✓ GET /api/v1/problems handles limit boundaries according to documented contract; tests and api-reference.md agree; guard and full short suite pass: Code: internal/api/handlers.go:432 uses clampIntDefault(q.Get("limit"), 20, 1, 100); helper at handlers.go:1011-1024 clamps n>max to max and falls back to def for empty/non-numeric/n<min (answers:532 and queue:627 correctly keep parseIntDefault). Docs: docs/api-reference.md:129 states 'default 20, max 100. Values above 100 are clamped to 100; omitted, non-numeric, and values below 1 fall back to the default' — exact match to impl. Tests: internal/api/handlers_test.go:817 TestListProblems_LimitBoundaries pins 100/101/200/0/-3/abc/omitted + offset pagination across the clamp boundary. RED-PROOF: reverting handler to parseIntDefault gave 'FAIL handlers_test.go:853: over max clamps (limit=101): got 20 problems, want 100' and 'far over max clamps (limit=200): got 20, want 100'; restored -> PASS. Commands: 'go test ./internal/api/ -run TestListProblems -count=1 -v' -> PASS (6/6 incl. LimitBoundaries); 'go test ./... -short -p 1 -count=1 -timeout 300s' -> all 13 pkgs ok, exit 0; 'gitreins guard' -> 'Tier 1 Guards: PASS (test mode: full)' 4/4 (secrets, go_build, go_lint, go_tests); go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty. LIVE E2E (fresh binary from HEAD 7817065, port 18779, 1651 problems): limit=100->100, limit=101->100 (clamped), limit=200->100 (clamped), limit=0/-3/abc/omitted->20, total invariant 1651, limit=200&offset=100->100 items. Note: the long-running server on :8766 (PID 1665810, binary built 12:39) predates the 13:09 fix commit and still returns 20 for limit=101/200 — a stale-binary artifact, not a code defect; a fresh build from HEAD behaves per contract.
GET /api/v1/problems now clamps over-max limit to the documented max of 100, with api-reference.md, a RED-proven boundary test, a passing guard 4/4 and full short suite, and live end-to-end behavior all in agreement.

## Summary

Judge Result: OB-GAP-081

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	1.822s

Stage tier2: PASS
  COMPLETE
  ✓ GET /api/v1/problems handles limit boundaries according to documented contract; tests and api-reference.md agree; guard and full short suite pass: Code: internal/api/handlers.go:432 uses clampIntDefault(q.Get("limit"), 20, 1, 100); helper at handlers.go:1011-1024 clamps n>max to max and falls back to def for empty/non-numeric/n<min (answers:532 and queue:627 correctly keep parseIntDefault). Docs: docs/api-reference.md:129 states 'default 20, max 100. Values above 100 are clamped to 100; omitted, non-numeric, and values below 1 fall back to the default' — exact match to impl. Tests: internal/api/handlers_test.go:817 TestListProblems_LimitBoundaries pins 100/101/200/0/-3/abc/omitted + offset pagination across the clamp boundary. RED-PROOF: reverting handler to parseIntDefault gave 'FAIL handlers_test.go:853: over max clamps (limit=101): got 20 problems, want 100' and 'far over max clamps (limit=200): got 20, want 100'; restored -> PASS. Commands: 'go test ./internal/api/ -run TestListProblems -count=1 -v' -> PASS (6/6 incl. LimitBoundaries); 'go test ./... -short -p 1 -count=1 -timeout 300s' -> all 13 pkgs ok, exit 0; 'gitreins guard' -> 'Tier 1 Guards: PASS (test mode: full)' 4/4 (secrets, go_build, go_lint, go_tests); go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty. LIVE E2E (fresh binary from HEAD 7817065, port 18779, 1651 problems): limit=100->100, limit=101->100 (clamped), limit=200->100 (clamped), limit=0/-3/abc/omitted->20, total invariant 1651, limit=200&offset=100->100 items. Note: the long-running server on :8766 (PID 1665810, binary built 12:39) predates the 13:09 fix commit and still returns 20 for limit=101/200 — a stale-binary artifact, not a code defect; a fresh build from HEAD behaves per contract.
GET /api/v1/problems now clamps over-max limit to the documented max of 100, with api-reference.md, a RED-proven boundary test, a passing guard 4/4 and full short suite, and live end-to-end behavior all in agreement.

Overall: PASS ✓
