# Verdict: OB-GAP-050

**Task:** Populate status/description/created_at in FTS search handler (q=)
**Evaluated:** 2026-08-21T04:19:21.185267
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m11:17PM[0m [32mINF[0m [1mscanned ~6030626 bytes (6.03 MB) in 1.46s[0m
[90m11:17PM[0m [3
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.015s
ok  	github.com/totalwindu
- ✓ **tier2**
  - COMPLETE
  ✗ curl 'http://localhost:8766/api/v1/problems?q=so-nil-pointer-deref' returns status:verified and a non-empty created_at for the class; go build ./... && go test ./... -short pass; FTS search tests updated: Code change is correct (fresh binary against fresh DB returns {"status":"verified","created_at":"2026-08-21T04:18:45Z"}), and go build ./... (exit 0) + go test ./... -short (exit 0, 13 pkgs ok) pass, and FTS tests updated (TestSearch_HitCarriesClassMetadata in internal/graph/search_test.go, TestListProblems_SearchQuery in internal/api/handlers_test.go both PASS). BUT the live curl against localhost:8766 FAILS: the running server (PID 801637, binary built 11:34, source changed 23:11 = stale) returns {"problems":[{"id":78,"title":"so-nil-pointer-deref","status":"pending","created_at":""}]} — status is 'pending' and created_at is empty, not 'verified'/non-empty. The deployed server has not been rebuilt/restarted with the OB-GAP-050 fix, so the literal curl criterion is not met.
The code fix is correct and all tests pass, but the live curl criterion fails because the running server on :8766 is a stale binary that returns status:pending and empty created_at instead of status:verified and a non-empty created_at.

## Summary

Judge Result: OB-GAP-050

Stage tier1: PASS
    ✓ secrets: [90m11:17PM[0m [32mINF[0m [1mscanned ~6030626 bytes (6.03 MB) in 1.46s[0m
[90m11:17PM[0m [3
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.015s
ok  	github.com/totalwindu

Stage tier2: PASS
  COMPLETE
  ✗ curl 'http://localhost:8766/api/v1/problems?q=so-nil-pointer-deref' returns status:verified and a non-empty created_at for the class; go build ./... && go test ./... -short pass; FTS search tests updated: Code change is correct (fresh binary against fresh DB returns {"status":"verified","created_at":"2026-08-21T04:18:45Z"}), and go build ./... (exit 0) + go test ./... -short (exit 0, 13 pkgs ok) pass, and FTS tests updated (TestSearch_HitCarriesClassMetadata in internal/graph/search_test.go, TestListProblems_SearchQuery in internal/api/handlers_test.go both PASS). BUT the live curl against localhost:8766 FAILS: the running server (PID 801637, binary built 11:34, source changed 23:11 = stale) returns {"problems":[{"id":78,"title":"so-nil-pointer-deref","status":"pending","created_at":""}]} — status is 'pending' and created_at is empty, not 'verified'/non-empty. The deployed server has not been rebuilt/restarted with the OB-GAP-050 fix, so the literal curl criterion is not met.
The code fix is correct and all tests pass, but the live curl criterion fails because the running server on :8766 is a stale binary that returns status:pending and empty created_at instead of status:verified and a non-empty created_at.

Overall: PASS ✓
