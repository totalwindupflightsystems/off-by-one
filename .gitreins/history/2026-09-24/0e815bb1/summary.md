# Verdict: DF-OFF-BY-ONE-15

**Task:** Anchor placeholder probe regexes (unanchored dogfood/canary patterns 404 real classes)
**Evaluated:** 2026-09-24T12:15:36.053176
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	41.058s
- ✓ **tier2**
  - COMPLETE
  ✓ POST /api/v1/problems/discover returns found:true for a solved class whose title contains 'dogfood' and one with 'canary'; GET /api/v1/queue lists such entries with honest totals; every probe family in placeholder.go comments still excluded (regression test added); EXCLUDED_CLASS_PATTERNS in export-answers.py kept in sync: Fix commit 6046258 anchors all 20 patterns in internal/graph/placeholder.go (^prefix, ^...$, separator-delimited token). LIVE VERIFIED on a scratch server (:18799, DB copy of off-by-one.db, binary built from HEAD): POST /api/v1/problems/discover -> HTTP 200 found=True for dogfood-stale-premise-filing, canary-deliver-failure-recurrence, python-canary-staleness-probe (all have verified answers in DB). Probes still 404: docs-canary-readme-status-refresh, self-dogfood-tick23, test-self-dogfood, off-by-one-self-test, self-test, e2e-tick100, tick88-foreman-audit, ds-007, shell-script-e2e, test. GET /api/v1/queue total=4274, which EXACTLY matches an independent SQL count over queue_entries (4512 rows - 238 placeholders = 4274); 0 probe leakage; all 3 contains-word classes present in the kept set (queue.go:426/443/457 use graph.NotPlaceholderClassSQL). Regression test added: TestIsPlaceholderClass_ContainsWordNotProbe (internal/graph/placeholder_test.go:90) plus extended TestIsPlaceholderClass, TestPlaceholderClassSQLFunc_MatchesGoPredicate, TestPlaceholderSQL_GoOracleOverTableRows. Sync: scripts/export-answers.py:44-65 has 20 patterns identical to the 20 Go patterns (only diff is Go's inline (?i) vs Python's re.IGNORECASE at line 66); programmatic parity check over 35 titles returned MISMATCHES: [] PARITY OK. TESTS: `go test ./internal/graph/... -run Placeholder -count=1` PASS (6/6); `go test ./internal/ingest/... -count=1` ok 0.084s; `go test ./internal/api/... -run 'Discover|ListQueue' -count=1` ok 0.055s; `go test ./... -short -count=1 -p 1 -timeout 900s` -> 14 packages ok, 0 FAIL; `python3 scripts/tests/export_answers_exclusions_test.py` Ran 2 tests OK; `make export-exclusions-selftest` OK; gofmt -l cmd/ internal/ pkg/ sql/ empty; go vet exit 0; LSP diagnostics 0. [resolution 0.24; placeholder.go, export-answers.py]
All four sub-claims verified with live end-to-end evidence: discover returns found:true for real dogfood/canary classes, queue totals are honest (4274 = independent SQL count), all probe families still 404 with a new regression test, and the Go/Python pattern lists are in sync — full test suite green.

## Summary

Judge Result: DF-OFF-BY-ONE-15

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	41.058s

Stage tier2: PASS
  COMPLETE
  ✓ POST /api/v1/problems/discover returns found:true for a solved class whose title contains 'dogfood' and one with 'canary'; GET /api/v1/queue lists such entries with honest totals; every probe family in placeholder.go comments still excluded (regression test added); EXCLUDED_CLASS_PATTERNS in export-answers.py kept in sync: Fix commit 6046258 anchors all 20 patterns in internal/graph/placeholder.go (^prefix, ^...$, separator-delimited token). LIVE VERIFIED on a scratch server (:18799, DB copy of off-by-one.db, binary built from HEAD): POST /api/v1/problems/discover -> HTTP 200 found=True for dogfood-stale-premise-filing, canary-deliver-failure-recurrence, python-canary-staleness-probe (all have verified answers in DB). Probes still 404: docs-canary-readme-status-refresh, self-dogfood-tick23, test-self-dogfood, off-by-one-self-test, self-test, e2e-tick100, tick88-foreman-audit, ds-007, shell-script-e2e, test. GET /api/v1/queue total=4274, which EXACTLY matches an independent SQL count over queue_entries (4512 rows - 238 placeholders = 4274); 0 probe leakage; all 3 contains-word classes present in the kept set (queue.go:426/443/457 use graph.NotPlaceholderClassSQL). Regression test added: TestIsPlaceholderClass_ContainsWordNotProbe (internal/graph/placeholder_test.go:90) plus extended TestIsPlaceholderClass, TestPlaceholderClassSQLFunc_MatchesGoPredicate, TestPlaceholderSQL_GoOracleOverTableRows. Sync: scripts/export-answers.py:44-65 has 20 patterns identical to the 20 Go patterns (only diff is Go's inline (?i) vs Python's re.IGNORECASE at line 66); programmatic parity check over 35 titles returned MISMATCHES: [] PARITY OK. TESTS: `go test ./internal/graph/... -run Placeholder -count=1` PASS (6/6); `go test ./internal/ingest/... -count=1` ok 0.084s; `go test ./internal/api/... -run 'Discover|ListQueue' -count=1` ok 0.055s; `go test ./... -short -count=1 -p 1 -timeout 900s` -> 14 packages ok, 0 FAIL; `python3 scripts/tests/export_answers_exclusions_test.py` Ran 2 tests OK; `make export-exclusions-selftest` OK; gofmt -l cmd/ internal/ pkg/ sql/ empty; go vet exit 0; LSP diagnostics 0. [resolution 0.24; placeholder.go, export-answers.py]
All four sub-claims verified with live end-to-end evidence: discover returns found:true for real dogfood/canary classes, queue totals are honest (4274 = independent SQL count), all probe families still 404 with a new regression test, and the Go/Python pattern lists are in sync — full test suite green.

Overall: PASS ✓
