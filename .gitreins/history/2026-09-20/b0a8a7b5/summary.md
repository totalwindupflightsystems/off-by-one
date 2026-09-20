# Verdict: DF-OFF-BY-ONE-10

**Task:** Fix DF-OFF-BY-ONE-10: GET /api/v1/queue list hides pending submissions
**Evaluated:** 2026-09-20T09:24:01.976767
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	2.226s
- ✓ **tier2**
  - COMPLETE
  ✓ Live on a scratch instance: POST /api/v1/problems/submit twice with fresh problem_class values, then GET /api/v1/queue -> entries non-null, total 2, both submission ids present, and GET /api/v1/queue?status=pending returns both; GET /api/v1/queue/{id} agrees. go test ./internal/api/ -run Queue -count=1 passes, including a new regression test that fails on the pre-fix code.: LIVE (binary built from HEAD e1ba282, OFF_BY_ONE_DB=/tmp/scratch10.db, port 8791): POST /api/v1/problems/submit x2 fresh classes -> sub_47cc2d (df10-fresh-alpha), sub_c0f3f2 (df10-fresh-beta). GET /api/v1/queue -> entries non-null (2 entries), total 2, both ids present. GET /api/v1/queue?status=pending -> both entries, total 2. GET /api/v1/queue/sub_47cc2d -> status pending, position 1, estimated_time 30s; GET /api/v1/queue/sub_c0f3f2 -> status pending, position 2, estimated_time 1m0s — both agree with the list. TESTS: `go test ./internal/api/ -run Queue -count=1` -> exit 0, 'ok github.com/totalwindupflightsystems/off-by-one/internal/api 0.032s', all 10 Queue tests PASS (incl. TestListQueue_NewestFirstHonestTotalAndPendingPosition). Full suite `go test ./... -short -p 1 -count=1 -timeout 240s` -> 13 pkgs ok; go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty. REGRESSION RED-PROOF: TestListQueue_NewestFirstHonestTotalAndPendingPosition is a NEW test added in commit e1ba282 (diff line '+func TestListQueue_NewestFirstHonestTotalAndPendingPosition'). Reverting the fix (internal/ingest/queue.go ORDER BY back to `priority DESC, created_at ASC`; internal/api/handlers.go Total back to len(entries)) makes it FAIL: 'handlers_test.go:1200: entries[0] = "sub_hist_hot", want the newest submission "sub_7dcea0"', 'handlers_test.go:1226: total with limit=1 = 1, want 5 (match count, not the page size)', 'handlers_test.go:1239: total at offset=3 = 2, want 5'. Restoring the fix (internal/ingest/queue.go:465 `ORDER BY created_at DESC, id ASC`; internal/api/handlers.go:636 ListPage total; :646 queueListResponse{Total: total}) makes it pass. Working tree verified clean after restore.
The queue list fix is verified end-to-end: live scratch instance returns both fresh submissions with total 2 and agreeing detail responses, all Queue tests pass, and the new regression test is proven RED on pre-fix code.

## Summary

Judge Result: DF-OFF-BY-ONE-10

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	2.226s

Stage tier2: PASS
  COMPLETE
  ✓ Live on a scratch instance: POST /api/v1/problems/submit twice with fresh problem_class values, then GET /api/v1/queue -> entries non-null, total 2, both submission ids present, and GET /api/v1/queue?status=pending returns both; GET /api/v1/queue/{id} agrees. go test ./internal/api/ -run Queue -count=1 passes, including a new regression test that fails on the pre-fix code.: LIVE (binary built from HEAD e1ba282, OFF_BY_ONE_DB=/tmp/scratch10.db, port 8791): POST /api/v1/problems/submit x2 fresh classes -> sub_47cc2d (df10-fresh-alpha), sub_c0f3f2 (df10-fresh-beta). GET /api/v1/queue -> entries non-null (2 entries), total 2, both ids present. GET /api/v1/queue?status=pending -> both entries, total 2. GET /api/v1/queue/sub_47cc2d -> status pending, position 1, estimated_time 30s; GET /api/v1/queue/sub_c0f3f2 -> status pending, position 2, estimated_time 1m0s — both agree with the list. TESTS: `go test ./internal/api/ -run Queue -count=1` -> exit 0, 'ok github.com/totalwindupflightsystems/off-by-one/internal/api 0.032s', all 10 Queue tests PASS (incl. TestListQueue_NewestFirstHonestTotalAndPendingPosition). Full suite `go test ./... -short -p 1 -count=1 -timeout 240s` -> 13 pkgs ok; go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty. REGRESSION RED-PROOF: TestListQueue_NewestFirstHonestTotalAndPendingPosition is a NEW test added in commit e1ba282 (diff line '+func TestListQueue_NewestFirstHonestTotalAndPendingPosition'). Reverting the fix (internal/ingest/queue.go ORDER BY back to `priority DESC, created_at ASC`; internal/api/handlers.go Total back to len(entries)) makes it FAIL: 'handlers_test.go:1200: entries[0] = "sub_hist_hot", want the newest submission "sub_7dcea0"', 'handlers_test.go:1226: total with limit=1 = 1, want 5 (match count, not the page size)', 'handlers_test.go:1239: total at offset=3 = 2, want 5'. Restoring the fix (internal/ingest/queue.go:465 `ORDER BY created_at DESC, id ASC`; internal/api/handlers.go:636 ListPage total; :646 queueListResponse{Total: total}) makes it pass. Working tree verified clean after restore.
The queue list fix is verified end-to-end: live scratch instance returns both fresh submissions with total 2 and agreeing detail responses, all Queue tests pass, and the new regression test is proven RED on pre-fix code.

Overall: PASS ✓
