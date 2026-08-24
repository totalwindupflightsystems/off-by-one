# Verdict: OB-GAP-055

**Task:** Stale in_progress queue entries never reclaimed (9 stuck, queue_depth inflated)
**Evaluated:** 2026-08-24T18:34:10.679747
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m1:32PM[0m [32mINF[0m [1mscanned ~6739486 bytes (6.74 MB) in 1.2s[0m
[90m1:32PM[0m [32mI
- ✓ **tier2**
  - COMPLETE
  ✓ reaper run reduces queue_depth to the true pending count; no in_progress entry older than 2x solve-timeout exists 1h after deploy: ReapStale (internal/ingest/queue.go:489) UPDATEs in_progress entries older than cutoff to status='failed'; Depth (queue.go:350) counts only pending+in_progress, so reaped rows drop out of queue_depth. ReapAfter defaults to 2*solver.DefaultSolveTimeout=60m (loop.go:186-187; DefaultSolveTimeout=30m in piagent.go:34). Tick calls ReapStale before the idle check (loop.go:313) every 5-min interval, so within 1h of deploy any in_progress entry older than 60m is swept. Tests: TestQueue_ReapStale (queue_test.go:366) proves stale reaped, fresh/NULL-started/pending survive, Depth 4->3, idempotent; TestTickReapsBeforeIdle/TestTickReapErrorNonFatal/TestResolveConfigReapAfter (loop_test.go). Actual run: go test ./... -short -count=1 -p 1 -timeout 120s -> all 13 pkgs ok; go build/vet exit 0; gofmt -l empty.
The stale in_progress reaper is correctly implemented, wired into the cron loop before the idle check, defaults to 2x solve-timeout (60m), reduces queue_depth, and all tests/build/vet/gofmt pass.

## Summary

Judge Result: OB-GAP-055

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m1:32PM[0m [32mINF[0m [1mscanned ~6739486 bytes (6.74 MB) in 1.2s[0m
[90m1:32PM[0m [32mI

Stage tier2: PASS
  COMPLETE
  ✓ reaper run reduces queue_depth to the true pending count; no in_progress entry older than 2x solve-timeout exists 1h after deploy: ReapStale (internal/ingest/queue.go:489) UPDATEs in_progress entries older than cutoff to status='failed'; Depth (queue.go:350) counts only pending+in_progress, so reaped rows drop out of queue_depth. ReapAfter defaults to 2*solver.DefaultSolveTimeout=60m (loop.go:186-187; DefaultSolveTimeout=30m in piagent.go:34). Tick calls ReapStale before the idle check (loop.go:313) every 5-min interval, so within 1h of deploy any in_progress entry older than 60m is swept. Tests: TestQueue_ReapStale (queue_test.go:366) proves stale reaped, fresh/NULL-started/pending survive, Depth 4->3, idempotent; TestTickReapsBeforeIdle/TestTickReapErrorNonFatal/TestResolveConfigReapAfter (loop_test.go). Actual run: go test ./... -short -count=1 -p 1 -timeout 120s -> all 13 pkgs ok; go build/vet exit 0; gofmt -l empty.
The stale in_progress reaper is correctly implemented, wired into the cron loop before the idle check, defaults to 2x solve-timeout (60m), reduces queue_depth, and all tests/build/vet/gofmt pass.

Overall: PASS ✓
