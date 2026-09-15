# Verdict: DF-OFF-BY-ONE-5

**Task:** Submit estimated_time derived from observed solve throughput, not a fixed per-position 30s
**Evaluated:** 2026-09-15T23:27:31.545669
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m6:26PM[0m [32mINF[0m [1mscanned ~6213069 bytes (6.21 MB) in 1.09s[0m
[90m6:26PM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ estimateTime in internal/api/handlers.go derives its answer from Queue.AvgSolveTime and the queued depth: with a completed solve averaging 2m13s and depth 3 the submit response estimated_time scales from 2m13s (never the old fixed 30s-per-position string), and with no completed solves it falls back to 30s per queued job: internal/api/handlers.go:310-316 submit handler calls s.Queue.AvgSolveTime(ctx) and s.Queue.Depth(ctx) then EstimatedTime: estimateTime(depth, perJob). estimateTime (handlers.go:964-987): depth<=0 -> "0s"; perJob<=0 -> defaultPerJobEstimate = 30*time.Second (handlers.go:948); total = depth*perJob capped at maxEstimateTotal = 30*time.Minute (handlers.go:952). No fixed 30s-per-position string remains. Queue.AvgSolveTime (internal/ingest/queue.go:424-437) returns 0 when no completed solves (NULL AVG). TestSubmit_EstimatedTimeFromObservedAverage (handlers_test.go:1311-1368) inserts a 2m13s completed solve and asserts depth 1 -> "2m13s" and depth 4 -> "8m52s" (4 x observed mean), proving scaling from the observed average rather than 30s/position.
  ✓ internal/api/handlers_test.go carries table-driven tests for the estimator (depth 0, no-history fallback, observed-average scaling, cap ceiling) and they pass under go test ./internal/api/ -run Estimate -count=1: TestEstimateTime (handlers_test.go:1256-1300) is table-driven with cases: "nothing queued" depth 0 -> "0s", depth 0 with no history -> "0s", negative depth -> "0s", no-history fallback depth 1 perJob 0 -> "30s", negative average fallback -> "1m0s", observed-average scaling depth 3 x 2m13s -> "6m39s", rounding, just-under-cap -> "29m30s", cap ceiling depth 20 -> "30m0s"; plus explicit cap assertion estimateTime(1000, observed) == maxEstimateTotal.String(). Ran `go test ./internal/api/ -run Estimate -count=1 -v`: exit_code 0, all 9 subtests PASS, TestSubmit_EstimatedTimeFromObservedAverage PASS, output line `ok github.com/totalwindupflightsystems/off-by-one/internal/api 0.010s`.
  ✓ go test ./... -short -p 1 -count=1 still passes 13 packages plus 2 no-test, and pkg/api/openapi.yaml describes estimated_time as a derived best-effort estimate rather than a guarantee: Ran `go test ./... -short -p 1 -count=1`: exit_code 0 with 13 `ok` lines (cmd/off-by-one, internal/api, internal/cron, internal/export, internal/graph, internal/import, internal/ingest, internal/muster, internal/sandbox, internal/seed, internal/solver, internal/web, pkg/api) and 2 `[no test files]` lines (sql/schema, web). pkg/api/openapi.yaml:439-448 describes estimated_time as "Best-effort estimate of how long the caller can expect to wait ... Derived from the lab's observed mean solve time and the number of queued jobs (capped), so it moves as solver throughput changes. It is not a guarantee: an idle lab with no completed solves yet falls back to a fixed per-job default...".
All three criteria verified: estimateTime derives ETA from Queue.AvgSolveTime x depth with a 30s no-history fallback and 30m cap, table-driven estimator tests pass under -run Estimate, the full -short suite passes 13 packages + 2 no-test, and openapi.yaml documents estimated_time as a non-guaranteed best-effort estimate.

## Summary

Judge Result: DF-OFF-BY-ONE-5

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m6:26PM[0m [32mINF[0m [1mscanned ~6213069 bytes (6.21 MB) in 1.09s[0m
[90m6:26PM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ estimateTime in internal/api/handlers.go derives its answer from Queue.AvgSolveTime and the queued depth: with a completed solve averaging 2m13s and depth 3 the submit response estimated_time scales from 2m13s (never the old fixed 30s-per-position string), and with no completed solves it falls back to 30s per queued job: internal/api/handlers.go:310-316 submit handler calls s.Queue.AvgSolveTime(ctx) and s.Queue.Depth(ctx) then EstimatedTime: estimateTime(depth, perJob). estimateTime (handlers.go:964-987): depth<=0 -> "0s"; perJob<=0 -> defaultPerJobEstimate = 30*time.Second (handlers.go:948); total = depth*perJob capped at maxEstimateTotal = 30*time.Minute (handlers.go:952). No fixed 30s-per-position string remains. Queue.AvgSolveTime (internal/ingest/queue.go:424-437) returns 0 when no completed solves (NULL AVG). TestSubmit_EstimatedTimeFromObservedAverage (handlers_test.go:1311-1368) inserts a 2m13s completed solve and asserts depth 1 -> "2m13s" and depth 4 -> "8m52s" (4 x observed mean), proving scaling from the observed average rather than 30s/position.
  ✓ internal/api/handlers_test.go carries table-driven tests for the estimator (depth 0, no-history fallback, observed-average scaling, cap ceiling) and they pass under go test ./internal/api/ -run Estimate -count=1: TestEstimateTime (handlers_test.go:1256-1300) is table-driven with cases: "nothing queued" depth 0 -> "0s", depth 0 with no history -> "0s", negative depth -> "0s", no-history fallback depth 1 perJob 0 -> "30s", negative average fallback -> "1m0s", observed-average scaling depth 3 x 2m13s -> "6m39s", rounding, just-under-cap -> "29m30s", cap ceiling depth 20 -> "30m0s"; plus explicit cap assertion estimateTime(1000, observed) == maxEstimateTotal.String(). Ran `go test ./internal/api/ -run Estimate -count=1 -v`: exit_code 0, all 9 subtests PASS, TestSubmit_EstimatedTimeFromObservedAverage PASS, output line `ok github.com/totalwindupflightsystems/off-by-one/internal/api 0.010s`.
  ✓ go test ./... -short -p 1 -count=1 still passes 13 packages plus 2 no-test, and pkg/api/openapi.yaml describes estimated_time as a derived best-effort estimate rather than a guarantee: Ran `go test ./... -short -p 1 -count=1`: exit_code 0 with 13 `ok` lines (cmd/off-by-one, internal/api, internal/cron, internal/export, internal/graph, internal/import, internal/ingest, internal/muster, internal/sandbox, internal/seed, internal/solver, internal/web, pkg/api) and 2 `[no test files]` lines (sql/schema, web). pkg/api/openapi.yaml:439-448 describes estimated_time as "Best-effort estimate of how long the caller can expect to wait ... Derived from the lab's observed mean solve time and the number of queued jobs (capped), so it moves as solver throughput changes. It is not a guarantee: an idle lab with no completed solves yet falls back to a fixed per-job default...".
All three criteria verified: estimateTime derives ETA from Queue.AvgSolveTime x depth with a 30s no-history fallback and 30m cap, table-driven estimator tests pass under -run Estimate, the full -short suite passes 13 packages + 2 no-test, and openapi.yaml documents estimated_time as a non-guaranteed best-effort estimate.

Overall: PASS ✓
