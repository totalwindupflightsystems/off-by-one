# Verdict: DF-OFF-BY-ONE-6

**Task:** Fix POST /api/v1/export 500 — handler drops ClassID
**Evaluated:** 2026-09-11T16:52:52.835323
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m11:52AM[0m [32mINF[0m [1mscanned ~5307082 bytes (5.31 MB) in 953ms[0m
[90m11:52AM[0m [3
- ✓ **tier2**
  - COMPLETE
  ✓ POST /api/v1/export with a valid local target_repo and existing answer_id returns HTTP 200 with commit_sha and files_changed >= 1 (not 500 export_failed); handler-level success-path test exists and passes: Fix in commit 7c30912: internal/api/handlers.go:786-800 now resolves each answer via s.Store.GetAnswerNode and builds export.ExportItem{AnswerID: id, ClassID: answer.ClassID}; previously ClassID was always 0, so export/git.go:256 GetProblemClass(ctx, 0) failed -> 500 export_failed. Missing answers now return 404 answer_not_found. Handler-level success-path test exists: internal/api/handlers_test.go TestExportSuccess (seeds class + verified answer, real bare git repo as target_repo, asserts 200 + non-empty CommitSHA + FilesChanged>=1). Fresh run: `go test ./internal/api/ -run 'TestExport' -count=1 -v` -> `--- PASS: TestExportSuccess (0.11s)`, `ok github.com/totalwindupflightsystems/off-by-one/internal/api 0.120s` (exit 0), not skipped. Regression proof: reverting handlers.go to 7c30912~1 yields `--- FAIL: TestExportSuccess ... status = 500, want 200; body: {"error":"export_failed","message":"export item class=0 answer=1: get problem class: graph: not found"}`. Full suite `go test ./... -short -p 1 -count=1 -timeout 180s` -> all 13 packages ok (exit 0); go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty.


## Summary

Judge Result: DF-OFF-BY-ONE-6

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m11:52AM[0m [32mINF[0m [1mscanned ~5307082 bytes (5.31 MB) in 953ms[0m
[90m11:52AM[0m [3

Stage tier2: PASS
  COMPLETE
  ✓ POST /api/v1/export with a valid local target_repo and existing answer_id returns HTTP 200 with commit_sha and files_changed >= 1 (not 500 export_failed); handler-level success-path test exists and passes: Fix in commit 7c30912: internal/api/handlers.go:786-800 now resolves each answer via s.Store.GetAnswerNode and builds export.ExportItem{AnswerID: id, ClassID: answer.ClassID}; previously ClassID was always 0, so export/git.go:256 GetProblemClass(ctx, 0) failed -> 500 export_failed. Missing answers now return 404 answer_not_found. Handler-level success-path test exists: internal/api/handlers_test.go TestExportSuccess (seeds class + verified answer, real bare git repo as target_repo, asserts 200 + non-empty CommitSHA + FilesChanged>=1). Fresh run: `go test ./internal/api/ -run 'TestExport' -count=1 -v` -> `--- PASS: TestExportSuccess (0.11s)`, `ok github.com/totalwindupflightsystems/off-by-one/internal/api 0.120s` (exit 0), not skipped. Regression proof: reverting handlers.go to 7c30912~1 yields `--- FAIL: TestExportSuccess ... status = 500, want 200; body: {"error":"export_failed","message":"export item class=0 answer=1: get problem class: graph: not found"}`. Full suite `go test ./... -short -p 1 -count=1 -timeout 180s` -> all 13 packages ok (exit 0); go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty.


Overall: PASS ✓
