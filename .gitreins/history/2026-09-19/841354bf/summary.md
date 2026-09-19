# Verdict: OB-GAP-080

**Task:** env/lang filters ignored when q is empty on GET /api/v1/problems
**Evaluated:** 2026-09-19T14:26:30.025153
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	2.838s
- ✓ **tier2**
  - COMPLETE
  ✓ GET /api/v1/problems?env=<nonsense> with empty q returns total=0; filters apply in both q and non-q branches; documented behavior matches; tests cover it: Live server (seeded DB, 1889 classes): curl 'http://localhost:8799/api/v1/problems?env=zzz-nonsense-env' -> {"problems":[],"total":0}; no filters -> total 1889; env=go1.26 -> 103; lang=nonsense -> 0. Both branches filter: internal/api/handlers.go:433-468 passes env/lang to Search/SearchCount (q branch) and ListProblemClassesWithCountsFiltered/CountProblemClasses (non-q branch); internal/graph/store.go:611,672 and internal/graph/search.go:30,147 apply `(? = '' OR EXISTS(... a.env = ?))` / lang equivalents. q branch live: q=raft&env=nonsense -> 0, q=raft&env=go1.26 -> 11. Tests: internal/api/handlers_test.go:765 TestListProblems_EnvLangFiltersWithoutQuery PASS and internal/graph/store_test.go:744 TestStore_ListProblemClassesWithCountsFiltered_EnvLang PASS (go test ./internal/api/... ./internal/graph/... -run 'EnvLang|ListProblems' -count=1 -v -> ok). Docs: docs/api-reference.md:118-132 documents env/lang as exact-match filters on GET /api/v1/problems including the 0-row outcome. Full suite: go test ./... -short -count=1 -p 1 -timeout 180s -> all 13 pkgs ok; go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty.
env/lang filters now apply in both the q and non-q branches of GET /api/v1/problems, verified live (nonsense env -> total=0), by code inspection, by passing unit tests, and documented in docs/api-reference.md.

## Summary

Judge Result: OB-GAP-080

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	2.838s

Stage tier2: PASS
  COMPLETE
  ✓ GET /api/v1/problems?env=<nonsense> with empty q returns total=0; filters apply in both q and non-q branches; documented behavior matches; tests cover it: Live server (seeded DB, 1889 classes): curl 'http://localhost:8799/api/v1/problems?env=zzz-nonsense-env' -> {"problems":[],"total":0}; no filters -> total 1889; env=go1.26 -> 103; lang=nonsense -> 0. Both branches filter: internal/api/handlers.go:433-468 passes env/lang to Search/SearchCount (q branch) and ListProblemClassesWithCountsFiltered/CountProblemClasses (non-q branch); internal/graph/store.go:611,672 and internal/graph/search.go:30,147 apply `(? = '' OR EXISTS(... a.env = ?))` / lang equivalents. q branch live: q=raft&env=nonsense -> 0, q=raft&env=go1.26 -> 11. Tests: internal/api/handlers_test.go:765 TestListProblems_EnvLangFiltersWithoutQuery PASS and internal/graph/store_test.go:744 TestStore_ListProblemClassesWithCountsFiltered_EnvLang PASS (go test ./internal/api/... ./internal/graph/... -run 'EnvLang|ListProblems' -count=1 -v -> ok). Docs: docs/api-reference.md:118-132 documents env/lang as exact-match filters on GET /api/v1/problems including the 0-row outcome. Full suite: go test ./... -short -count=1 -p 1 -timeout 180s -> all 13 pkgs ok; go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty.
env/lang filters now apply in both the q and non-q branches of GET /api/v1/problems, verified live (nonsense env -> total=0), by code inspection, by passing unit tests, and documented in docs/api-reference.md.

Overall: PASS ✓
