# Verdict: DF-OFF-BY-ONE-11

**Task:** Queue endpoints must carry position and estimated_time (spec-declared fields were never populated)
**Evaluated:** 2026-09-17T00:21:45.542478
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m7:20PM[0m [32mINF[0m [1mscanned ~5781736 bytes (5.78 MB) in 2.45s[0m
[90m7:20PM[0m [32m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
- ✓ **tier2**
  - COMPLETE
  ✓ AC1: internal/api never returns an empty estimated_time for a queue entry that is waiting or solving: GET /api/v1/queue/{submission_id} and GET /api/v1/queue populate estimated_time from the same estimator the submit response uses (estimateTime over Queue.AvgSolveTime), for status pending and in_progress; terminal entries (complete/failed) return estimated_time "" as documented. AC2: GET /api/v1/queue/{submission_id} populates position as the 1-based place in the pending queue for a pending entry and 0 for a non-pending entry; the list endpoint keeps its existing position = offset+i+1 behaviour. AC3: unit tests in internal/api/handlers_test.go assert both fields on the wire (raw JSON key checks) for a pending entry, a complete entry, and the list response. AC4: pkg/api/openapi.yaml QueueEntry.position and QueueEntry.estimated_time carry descriptions of those semantics and docs/api-reference.md matches. AC5: gofmt clean, go build ./... , go vet ./... and go test ./... -short -count=1 pass; gitreins guard full passes.: AC1: handlers.go:655-656 (detail) and :629-634 (list) read s.Queue.AvgSolveTime once then call queueWireEntry (handlers.go:773-787): pending -> estimateTime(pendingIndex, perJob), in_progress -> estimateTime(1, perJob), complete/failed -> EstimatedTime left "" by entryToWire. Submit path (handlers.go:310-316) uses the identical estimateTime(depth, perJob) over AvgSolveTime (estimateTime defined handlers.go:1024). AC2: queueWireEntry sets Position=pendingIndex (1-based pending place) for pending, 0 otherwise; pendingPositions (handlers.go:794-804) maps ID->i+1; handleListQueue (handlers.go:634) overwrites Position=offset+i+1 preserving list behaviour. AC3: handlers_test.go TestGetQueueStatus_PositionAndEstimatedTime (~1166) and TestListQueue_PositionAndEstimatedTime (~1303) do raw-JSON key checks (raw["position"], raw["estimated_time"], raw.Entries[i]["estimated_time"]) for pending/complete/failed/in_progress and the list; both PASS with the fix and were confirmed to FAIL against pre-fix handlers (position=0, estimated_time=""). AC4: pkg/api/openapi.yaml:602-618 both fields carry multi-line semantic descriptions; docs/api-reference.md Queue section adds a matching semantics table. AC5: gofmt -l cmd/ internal/ pkg/ sql/ empty (exit 0); go build ./... exit 0; go vet ./... exit 0; go test ./... -short -count=1 EXIT=0 with 13 pkgs 'ok' and no FAIL; gitreins guard --full -> 'Tier 1 Guards: PASS (test mode: full)' (secrets clean, go_build ok, go_lint ok, go_tests ok), exit 0.
All five acceptance criteria are met: queue detail/list endpoints populate position and estimated_time via the shared estimateTime/AvgSolveTime estimator, raw-JSON wire tests cover pending/complete/failed/in_progress and the list, OpenAPI + docs describe the semantics, and gofmt/build/vet/test plus gitreins guard full all pass.

## Summary

Judge Result: DF-OFF-BY-ONE-11

Stage tier1: PASS
    ✓ secrets: [90m7:20PM[0m [32mINF[0m [1mscanned ~5781736 bytes (5.78 MB) in 2.45s[0m
[90m7:20PM[0m [32m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin

Stage tier2: PASS
  COMPLETE
  ✓ AC1: internal/api never returns an empty estimated_time for a queue entry that is waiting or solving: GET /api/v1/queue/{submission_id} and GET /api/v1/queue populate estimated_time from the same estimator the submit response uses (estimateTime over Queue.AvgSolveTime), for status pending and in_progress; terminal entries (complete/failed) return estimated_time "" as documented. AC2: GET /api/v1/queue/{submission_id} populates position as the 1-based place in the pending queue for a pending entry and 0 for a non-pending entry; the list endpoint keeps its existing position = offset+i+1 behaviour. AC3: unit tests in internal/api/handlers_test.go assert both fields on the wire (raw JSON key checks) for a pending entry, a complete entry, and the list response. AC4: pkg/api/openapi.yaml QueueEntry.position and QueueEntry.estimated_time carry descriptions of those semantics and docs/api-reference.md matches. AC5: gofmt clean, go build ./... , go vet ./... and go test ./... -short -count=1 pass; gitreins guard full passes.: AC1: handlers.go:655-656 (detail) and :629-634 (list) read s.Queue.AvgSolveTime once then call queueWireEntry (handlers.go:773-787): pending -> estimateTime(pendingIndex, perJob), in_progress -> estimateTime(1, perJob), complete/failed -> EstimatedTime left "" by entryToWire. Submit path (handlers.go:310-316) uses the identical estimateTime(depth, perJob) over AvgSolveTime (estimateTime defined handlers.go:1024). AC2: queueWireEntry sets Position=pendingIndex (1-based pending place) for pending, 0 otherwise; pendingPositions (handlers.go:794-804) maps ID->i+1; handleListQueue (handlers.go:634) overwrites Position=offset+i+1 preserving list behaviour. AC3: handlers_test.go TestGetQueueStatus_PositionAndEstimatedTime (~1166) and TestListQueue_PositionAndEstimatedTime (~1303) do raw-JSON key checks (raw["position"], raw["estimated_time"], raw.Entries[i]["estimated_time"]) for pending/complete/failed/in_progress and the list; both PASS with the fix and were confirmed to FAIL against pre-fix handlers (position=0, estimated_time=""). AC4: pkg/api/openapi.yaml:602-618 both fields carry multi-line semantic descriptions; docs/api-reference.md Queue section adds a matching semantics table. AC5: gofmt -l cmd/ internal/ pkg/ sql/ empty (exit 0); go build ./... exit 0; go vet ./... exit 0; go test ./... -short -count=1 EXIT=0 with 13 pkgs 'ok' and no FAIL; gitreins guard --full -> 'Tier 1 Guards: PASS (test mode: full)' (secrets clean, go_build ok, go_lint ok, go_tests ok), exit 0.
All five acceptance criteria are met: queue detail/list endpoints populate position and estimated_time via the shared estimateTime/AvgSolveTime estimator, raw-JSON wire tests cover pending/complete/failed/in_progress and the list, OpenAPI + docs describe the semantics, and gofmt/build/vet/test plus gitreins guard full all pass.

Overall: PASS ✓
