# Verdict: DF-OFF-BY-ONE-32-rework

**Task:** Fix dead-code metrics: memory gate, peak RSS, /metrics endpoint, wiring, tests
**Evaluated:** 2026-10-04T07:08:55.865683
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✗ **tier2**
  - INCOMPLETE
  ✗ All 6 judge failures resolved: memory gate in checkIdle, PeakRSSBytes populated from sandbox.Sampler, GET /metrics endpoint, Observer constructed in main.go, tests pass, over-budget alert fires: 2 of the 6 sub-failures are unresolved. (a) PeakRSSBytes is NOT populated from sandbox.Sampler: no Sampler type exists anywhere — `grep -rn Sampler internal/sandbox/ internal/solver/ internal/cron/ cmd/` returns 0 hits (GREP_EXIT=1); the only match repo-wide is a doc comment at internal/metrics/metrics.go:43 ('see sandbox.Sampler'). No RSS sampling code (no /proc/<pid>/status, VmHWM, statm) exists. Every cron/loop.go RecordSolve call (lines 437, 455, 469, 481) sets only SubmissionID/ProblemClass/WallTimeMS/Success — PeakRSSBytes, TestRuns, Killed and Model are always zero, so the field is dead in production. (b) Observer is constructed in main.go (cmd/off-by-one/main.go:170 `observer := metrics.New(metrics.DefaultRSSAlertBytes, log.Default())`, passed to cron.Config.Observer at :178) but is NEVER wired to the API server: `grep -rn '.Observer =' cmd/ internal/` returns 0 hits, and main.go's only apiServer assignments are ExportLocalDir/ImportLocalDir/ReadOnly/SolverAvailable (lines 194-197). internal/api/server.go:228 therefore always takes the nil branch and GET /metrics returns 404 metrics_disabled in the real binary. (c) Consequently the over-budget alert can never fire in production: internal/metrics/metrics.go:108-126 does log 'ERROR solve ALERT: peak RSS ...' and metrics_test.go TestRecordSolveOverBudget asserts it, but with PeakRSSBytes always 0 the branch is unreachable outside unit tests. (d) No test covers the /metrics endpoint at all — `grep -n 'metrics|Observer' internal/api/handlers.go internal/api/coverage_test.go internal/api/handlers_test.go internal/api/queue_page_test.go` returns nothing (GREP_EXIT=1). Verified PASS sub-items: memory gate present in checkIdle (internal/cron/loop.go:380-395, MemoryThreshold/MemoryProbe, skipReason load/memory/load+memory, Observer.RecordSkip at :398-404; probeMemory in internal/cron/memory.go:12); GET /metrics route registered (internal/api/server.go:108) with handler at :225-234; tests pass — `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./... -short -p 1 -count=1 -timeout 180s` exit 0 with 'ok internal/metrics 0.002s', 'ok internal/api 0.553s', 'ok internal/cron 0.081s'.
The memory gate, /metrics route and test suite are in place, but PeakRSSBytes is never populated (no sandbox.Sampler exists) and the Observer is never assigned to apiServer in main.go, so /metrics 404s and the over-budget alert cannot fire in production.

## Summary

Judge Result: DF-OFF-BY-ONE-32-rework

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: FAIL
  INCOMPLETE
  ✗ All 6 judge failures resolved: memory gate in checkIdle, PeakRSSBytes populated from sandbox.Sampler, GET /metrics endpoint, Observer constructed in main.go, tests pass, over-budget alert fires: 2 of the 6 sub-failures are unresolved. (a) PeakRSSBytes is NOT populated from sandbox.Sampler: no Sampler type exists anywhere — `grep -rn Sampler internal/sandbox/ internal/solver/ internal/cron/ cmd/` returns 0 hits (GREP_EXIT=1); the only match repo-wide is a doc comment at internal/metrics/metrics.go:43 ('see sandbox.Sampler'). No RSS sampling code (no /proc/<pid>/status, VmHWM, statm) exists. Every cron/loop.go RecordSolve call (lines 437, 455, 469, 481) sets only SubmissionID/ProblemClass/WallTimeMS/Success — PeakRSSBytes, TestRuns, Killed and Model are always zero, so the field is dead in production. (b) Observer is constructed in main.go (cmd/off-by-one/main.go:170 `observer := metrics.New(metrics.DefaultRSSAlertBytes, log.Default())`, passed to cron.Config.Observer at :178) but is NEVER wired to the API server: `grep -rn '.Observer =' cmd/ internal/` returns 0 hits, and main.go's only apiServer assignments are ExportLocalDir/ImportLocalDir/ReadOnly/SolverAvailable (lines 194-197). internal/api/server.go:228 therefore always takes the nil branch and GET /metrics returns 404 metrics_disabled in the real binary. (c) Consequently the over-budget alert can never fire in production: internal/metrics/metrics.go:108-126 does log 'ERROR solve ALERT: peak RSS ...' and metrics_test.go TestRecordSolveOverBudget asserts it, but with PeakRSSBytes always 0 the branch is unreachable outside unit tests. (d) No test covers the /metrics endpoint at all — `grep -n 'metrics|Observer' internal/api/handlers.go internal/api/coverage_test.go internal/api/handlers_test.go internal/api/queue_page_test.go` returns nothing (GREP_EXIT=1). Verified PASS sub-items: memory gate present in checkIdle (internal/cron/loop.go:380-395, MemoryThreshold/MemoryProbe, skipReason load/memory/load+memory, Observer.RecordSkip at :398-404; probeMemory in internal/cron/memory.go:12); GET /metrics route registered (internal/api/server.go:108) with handler at :225-234; tests pass — `go build ./...` exit 0, `go vet ./...` exit 0, `go test ./... -short -p 1 -count=1 -timeout 180s` exit 0 with 'ok internal/metrics 0.002s', 'ok internal/api 0.553s', 'ok internal/cron 0.081s'.
The memory gate, /metrics route and test suite are in place, but PeakRSSBytes is never populated (no sandbox.Sampler exists) and the Observer is never assigned to apiServer in main.go, so /metrics 404s and the over-budget alert cannot fire in production.

Overall: FAIL ✗
