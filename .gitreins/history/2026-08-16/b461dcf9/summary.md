# Verdict: OB-GAP-039

**Task:** seed subcommand must honor OFF_BY_ONE_DB env var
**Evaluated:** 2026-08-16T17:32:54.165588
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ lint: 
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.163s
ok  	github.com/totalwindu
  ✗ secrets: [90m12:30PM[0m [32mINF[0m [1mscanned ~9384538 bytes (9.38 MB) in 2.56s[0m
[90m12:30PM[0m [3
- ✓ **tier2**
  - COMPLETE
  ✓ OFF_BY_ONE_DB=/tmp/x.db ./off-by-one seed creates /tmp/x.db with corpus loaded; OFF_BY_ONE_DB=/tmp/x.db ./off-by-one serves GET /api/v1/stats with total_answers > 0 and POST /api/v1/problems/discover for 0078-so-nil-pointer-deref returns found:true: Verified end-to-end with fresh build. (1) OFF_BY_ONE_DB=/tmp/x.db ./off-by-one seed created /tmp/x.db (7901184 bytes) with corpus loaded (classes=1038, answers=1120) — cmd/off-by-one/seed.go:27 uses envString("OFF_BY_ONE_DB", "./off-by-one.db") as -db default. (2) OFF_BY_ONE_DB=/tmp/x.db ./off-by-one served GET /api/v1/stats with total_answers=1120 (>0). (3) POST /api/v1/problems/discover for the 0078 class returned found:true — the class file is 0078-so-nil-pointer-deref.json with title 'so-nil-pointer-deref' (0078 is the class_id prefix in the filename); discover matches by exact title (internal/graph/store.go:245 GetProblemClassByTitle WHERE title=?), so querying 'so-nil-pointer-deref' returns found:true. Regression test TestRunSeedHonorsEnvVar passes (cmd/off-by-one/seed_test.go:44). go test ./... -short all ok (14 pkgs), go vet clean, gofmt clean, no LSP diagnostics.
The seed subcommand correctly honors OFF_BY_ONE_DB: seeding creates /tmp/x.db with the corpus, the server serves stats with total_answers>0, and discover for the 0078 class returns found:true.

## Summary

Judge Result: OB-GAP-039

Stage tier1: FAIL
    ✓ lint: 
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.163s
ok  	github.com/totalwindu
  ✗ secrets: [90m12:30PM[0m [32mINF[0m [1mscanned ~9384538 bytes (9.38 MB) in 2.56s[0m
[90m12:30PM[0m [3

Stage tier2: PASS
  COMPLETE
  ✓ OFF_BY_ONE_DB=/tmp/x.db ./off-by-one seed creates /tmp/x.db with corpus loaded; OFF_BY_ONE_DB=/tmp/x.db ./off-by-one serves GET /api/v1/stats with total_answers > 0 and POST /api/v1/problems/discover for 0078-so-nil-pointer-deref returns found:true: Verified end-to-end with fresh build. (1) OFF_BY_ONE_DB=/tmp/x.db ./off-by-one seed created /tmp/x.db (7901184 bytes) with corpus loaded (classes=1038, answers=1120) — cmd/off-by-one/seed.go:27 uses envString("OFF_BY_ONE_DB", "./off-by-one.db") as -db default. (2) OFF_BY_ONE_DB=/tmp/x.db ./off-by-one served GET /api/v1/stats with total_answers=1120 (>0). (3) POST /api/v1/problems/discover for the 0078 class returned found:true — the class file is 0078-so-nil-pointer-deref.json with title 'so-nil-pointer-deref' (0078 is the class_id prefix in the filename); discover matches by exact title (internal/graph/store.go:245 GetProblemClassByTitle WHERE title=?), so querying 'so-nil-pointer-deref' returns found:true. Regression test TestRunSeedHonorsEnvVar passes (cmd/off-by-one/seed_test.go:44). go test ./... -short all ok (14 pkgs), go vet clean, gofmt clean, no LSP diagnostics.
The seed subcommand correctly honors OFF_BY_ONE_DB: seeding creates /tmp/x.db with the corpus, the server serves stats with total_answers>0, and discover for the 0078 class returns found:true.

Overall: FAIL ✗
