# Verdict: OB-GAP-020

**Task:** Allow POST /api/v1/problems/discover in read-only catalog mode (pure read); submit/export/import stay blocked; integration.md corrected
**Evaluated:** 2026-08-10T13:29:40.607729
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ lint: 
  ✓ secrets: [90m8:28AM[0m [32mINF[0m [1mscanned ~8219413 bytes (8.22 MB) in 2.42s[0m
[90m8:28AM[0m [33m
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
- ✓ **tier2**
  - COMPLETE
  ✓ Readonly instance: POST /api/v1/problems/discover returns 200 found:true (not 403): internal/api/server.go:112 exempts discover via readOnlyAllowedPost (line 129 returns true only for /api/v1/problems/discover); handleDiscover (handlers.go:342) is pure read (Store.Discovery does only SELECTs). TestReadOnly_DiscoverAllowed (handlers_test.go:346) passes: POST discover returns 200 found:true.
  ✓ Readonly instance: POST /api/v1/problems/submit and /api/v1/export still return 403 read_only: internal/api/server.go:112-113 blocks all other POSTs with 403 read_only. TestReadOnly_MutatingEndpointsBlocked (handlers_test.go:371) verifies POST submit/export/import/ws-chat all return 403 with error=read_only; passes.
  ✓ Tests cover readonly discover allowed + readonly submit blocked: internal/api/handlers_test.go:346 TestReadOnly_DiscoverAllowed and :371 TestReadOnly_MutatingEndpointsBlocked both present and pass (go test -short -run TestReadOnly ./internal/api/ -v: both PASS).
  ✓ go build/vet/gofmt clean, go test -short ./... passes, gitreins guard 4/4: go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty; go test -short ./... exit 0 (11 packages ok, 0 FAIL). LSP diagnostics empty, dead-code 0, skylos A+. All 4 criteria verified by guard.
Readonly catalog mode now allows POST /api/v1/problems/discover (pure read, 200 found:true) while submit/export/import stay blocked with 403 read_only; tests cover both, integration.md corrected, and build/vet/gofmt/tests all clean.

## Summary

Judge Result: OB-GAP-020

Stage tier1: PASS
    ✓ lint: 
  ✓ secrets: [90m8:28AM[0m [32mINF[0m [1mscanned ~8219413 bytes (8.22 MB) in 2.42s[0m
[90m8:28AM[0m [33m
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t

Stage tier2: PASS
  COMPLETE
  ✓ Readonly instance: POST /api/v1/problems/discover returns 200 found:true (not 403): internal/api/server.go:112 exempts discover via readOnlyAllowedPost (line 129 returns true only for /api/v1/problems/discover); handleDiscover (handlers.go:342) is pure read (Store.Discovery does only SELECTs). TestReadOnly_DiscoverAllowed (handlers_test.go:346) passes: POST discover returns 200 found:true.
  ✓ Readonly instance: POST /api/v1/problems/submit and /api/v1/export still return 403 read_only: internal/api/server.go:112-113 blocks all other POSTs with 403 read_only. TestReadOnly_MutatingEndpointsBlocked (handlers_test.go:371) verifies POST submit/export/import/ws-chat all return 403 with error=read_only; passes.
  ✓ Tests cover readonly discover allowed + readonly submit blocked: internal/api/handlers_test.go:346 TestReadOnly_DiscoverAllowed and :371 TestReadOnly_MutatingEndpointsBlocked both present and pass (go test -short -run TestReadOnly ./internal/api/ -v: both PASS).
  ✓ go build/vet/gofmt clean, go test -short ./... passes, gitreins guard 4/4: go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty; go test -short ./... exit 0 (11 packages ok, 0 FAIL). LSP diagnostics empty, dead-code 0, skylos A+. All 4 criteria verified by guard.
Readonly catalog mode now allows POST /api/v1/problems/discover (pure read, 200 found:true) while submit/export/import stay blocked with 403 read_only; tests cover both, integration.md corrected, and build/vet/gofmt/tests all clean.

Overall: PASS ✓
