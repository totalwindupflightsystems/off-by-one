# Verdict: OB-GAP-064

**Task:** Exclude failed-signature answers from discovery version history and best-status aggregates
**Evaluated:** 2026-09-17T01:15:34.245591
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m8:14PM[0m [32mINF[0m [1mscanned ~6352152 bytes (6.35 MB) in 1.23s[0m
[90m8:14PM[0m [32m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.434s
ok  	github.com/totalwindu
- ✓ **tier2**
  - COMPLETE
  ✓ AC1: Store.versionHistory (internal/graph/discovery.go) omits any answer_nodes row whose signatures JSON has result=failed from DiscoveryResult.Versions, while still returning good ancestors deeper in the parent_id chain; covered by a test in package internal/graph. AC2: the derived best_status in internal/graph/store.go (ListProblemClassesWithCountsFiltered and CountProblemClasses) and internal/graph/search.go (Search) never reports verified or ci_passed for a row whose signatures JSON has result=failed; covered by tests in internal/graph/store_test.go and internal/graph/search_test.go. AC3: gofmt -l cmd internal pkg sql prints nothing; go build ./... , go vet ./... , go test ./... -short -count=1 -p 1 and gitreins guard (full Tier 1) all pass. AC4: non-vacuity proven - the new tests fail when the predicate usage is disabled (versions still contain the failed-signature ancestor; derived status reads verified) and pass with it.: AC1: discovery.go versionHistory adds `if signatureFailed(a.Signatures) { continue }` before append (continue, not break, so good ancestors deeper in chain survive); signatureFailed defined in new internal/graph/signature.go. Test TestStore_Discovery_VersionHistory_ExcludesFailedSignature (store_test.go, 3-link chain with failed middle) PASSES. AC2: store.go ListProblemClassesWithCountsFiltered (~L598-599) and CountProblemClasses (~L645-646) add `AND signatureNotFailedSQL` to ci_passed/verified branches; search.go Search (~L112-113) same; GetProblemClassStatus also fixed. Tests TestStore_ListProblemClassesWithCounts_ExcludesFailedSignatureStatus, TestStore_CountProblemClasses_ExcludesFailedSignatureStatus, TestSearch_StatusExcludesFailedSignature all PASS. AC3: `gofmt -l cmd internal pkg sql` empty (exit 0); `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./... -short -count=1 -p 1` -> all 13 pkgs 'ok', exit 0; `gitreins guard` -> 'Tier 1 Guards: PASS (test mode: full)' secrets clean/go_build ok/go_lint ok/go_tests ok, exit 0. AC4: with predicate disabled (signatureNotFailedSQL="1 = 1", signatureFailed always false) all 5 new tests FAIL with expected messages ('Versions contains failed-signature row: [oldest good root failed middle link newest good tip] (id 2, want it omitted)', 'Versions = [...] (3 rows), want 2', 'status = "verified", want "failed"', 'CountProblemClasses("solved") = 2, want 1', 'Search hit status = "verified", want "failed"'); files restored, git status clean, tests green again.


## Summary

Judge Result: OB-GAP-064

Stage tier1: PASS
    ✓ secrets: [90m8:14PM[0m [32mINF[0m [1mscanned ~6352152 bytes (6.35 MB) in 1.23s[0m
[90m8:14PM[0m [32m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.434s
ok  	github.com/totalwindu

Stage tier2: PASS
  COMPLETE
  ✓ AC1: Store.versionHistory (internal/graph/discovery.go) omits any answer_nodes row whose signatures JSON has result=failed from DiscoveryResult.Versions, while still returning good ancestors deeper in the parent_id chain; covered by a test in package internal/graph. AC2: the derived best_status in internal/graph/store.go (ListProblemClassesWithCountsFiltered and CountProblemClasses) and internal/graph/search.go (Search) never reports verified or ci_passed for a row whose signatures JSON has result=failed; covered by tests in internal/graph/store_test.go and internal/graph/search_test.go. AC3: gofmt -l cmd internal pkg sql prints nothing; go build ./... , go vet ./... , go test ./... -short -count=1 -p 1 and gitreins guard (full Tier 1) all pass. AC4: non-vacuity proven - the new tests fail when the predicate usage is disabled (versions still contain the failed-signature ancestor; derived status reads verified) and pass with it.: AC1: discovery.go versionHistory adds `if signatureFailed(a.Signatures) { continue }` before append (continue, not break, so good ancestors deeper in chain survive); signatureFailed defined in new internal/graph/signature.go. Test TestStore_Discovery_VersionHistory_ExcludesFailedSignature (store_test.go, 3-link chain with failed middle) PASSES. AC2: store.go ListProblemClassesWithCountsFiltered (~L598-599) and CountProblemClasses (~L645-646) add `AND signatureNotFailedSQL` to ci_passed/verified branches; search.go Search (~L112-113) same; GetProblemClassStatus also fixed. Tests TestStore_ListProblemClassesWithCounts_ExcludesFailedSignatureStatus, TestStore_CountProblemClasses_ExcludesFailedSignatureStatus, TestSearch_StatusExcludesFailedSignature all PASS. AC3: `gofmt -l cmd internal pkg sql` empty (exit 0); `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./... -short -count=1 -p 1` -> all 13 pkgs 'ok', exit 0; `gitreins guard` -> 'Tier 1 Guards: PASS (test mode: full)' secrets clean/go_build ok/go_lint ok/go_tests ok, exit 0. AC4: with predicate disabled (signatureNotFailedSQL="1 = 1", signatureFailed always false) all 5 new tests FAIL with expected messages ('Versions contains failed-signature row: [oldest good root failed middle link newest good tip] (id 2, want it omitted)', 'Versions = [...] (3 rows), want 2', 'status = "verified", want "failed"', 'CountProblemClasses("solved") = 2, want 1', 'Search hit status = "verified", want "failed"'); files restored, git status clean, tests green again.


Overall: PASS ✓
