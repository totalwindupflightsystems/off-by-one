# Verdict: DF-OFF-BY-ONE-32

**Task:** Make feeder host-aware and add per-solve observability
**Evaluated:** 2026-10-04T06:46:06.798956
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ Feeder skips runs under host pressure; per-solve metrics (peak RSS, wall time, skip reason) are queryable; alert fires on over-budget solve: Multiple independent failures. (1) 'host pressure' is load-only: internal/cron/loop.go checkIdle() (lines 346-372) checks only the loadavg IdleProbe and records Reason:"load"; the memory fields HostSnapshot.MemUsedFraction/MemTotalBytes/MemAvailableBytes (metrics.go:67-69) are declared but never populated, so the documented 'memory'/'load+memory' skip reasons are never produced. (2) Peak RSS is never captured: all four RecordSolve call sites in loop.go (lines 395, 413, 427, 439) set only SubmissionID/ProblemClass/WallTimeMS/Success — PeakRSSBytes stays 0; no sandbox.Sampler exists. (3) Metrics are NOT queryable: internal/api/server.go routes (lines 88-102) contain no /metrics endpoint, contradicting the commit message's 'Add GET /metrics endpoint to API server'; the commit stat touches only 3 files (tasks.yaml, cron/loop.go, metrics/metrics.go). (4) No production wiring: metrics.New is never called anywhere (grep found zero callers) and cmd/off-by-one/main.go:167-172 builds cron.Config with only Interval/LoadThreshold/Solver/Queue — Observer is always nil, so nothing is ever recorded. (5) The over-budget alert can never fire because PeakRSSBytes is always 0 (Observer.RecordSolve at metrics.go:110-125 compares PeakRSSBytes > alertBytes). (6) Tests claimed in the commit message do not exist: `go test ./internal/metrics/... ./internal/cron/... -count=1` => 'internal/metrics [no test files]' and 'ok internal/cron 0.100s'; find for *metrics*test* is empty and grep for RecordSolve/RecordSkip/OverBudget/AlertThreshold in *_test.go returns 0 matches. go build ./... exits 0, but the feature is dead code.
The metrics package and cron Observer hooks exist, but the feature is unwired (Observer never constructed in main.go), peak RSS is never populated, there is no memory-pressure gate, no /metrics endpoint, and no tests — so the criterion fails.

## Summary

Judge Result: DF-OFF-BY-ONE-32

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ Feeder skips runs under host pressure; per-solve metrics (peak RSS, wall time, skip reason) are queryable; alert fires on over-budget solve: Multiple independent failures. (1) 'host pressure' is load-only: internal/cron/loop.go checkIdle() (lines 346-372) checks only the loadavg IdleProbe and records Reason:"load"; the memory fields HostSnapshot.MemUsedFraction/MemTotalBytes/MemAvailableBytes (metrics.go:67-69) are declared but never populated, so the documented 'memory'/'load+memory' skip reasons are never produced. (2) Peak RSS is never captured: all four RecordSolve call sites in loop.go (lines 395, 413, 427, 439) set only SubmissionID/ProblemClass/WallTimeMS/Success — PeakRSSBytes stays 0; no sandbox.Sampler exists. (3) Metrics are NOT queryable: internal/api/server.go routes (lines 88-102) contain no /metrics endpoint, contradicting the commit message's 'Add GET /metrics endpoint to API server'; the commit stat touches only 3 files (tasks.yaml, cron/loop.go, metrics/metrics.go). (4) No production wiring: metrics.New is never called anywhere (grep found zero callers) and cmd/off-by-one/main.go:167-172 builds cron.Config with only Interval/LoadThreshold/Solver/Queue — Observer is always nil, so nothing is ever recorded. (5) The over-budget alert can never fire because PeakRSSBytes is always 0 (Observer.RecordSolve at metrics.go:110-125 compares PeakRSSBytes > alertBytes). (6) Tests claimed in the commit message do not exist: `go test ./internal/metrics/... ./internal/cron/... -count=1` => 'internal/metrics [no test files]' and 'ok internal/cron 0.100s'; find for *metrics*test* is empty and grep for RecordSolve/RecordSkip/OverBudget/AlertThreshold in *_test.go returns 0 matches. go build ./... exits 0, but the feature is dead code.
The metrics package and cron Observer hooks exist, but the feature is unwired (Observer never constructed in main.go), peak RSS is never populated, there is no memory-pressure gate, no /metrics endpoint, and no tests — so the criterion fails.

Overall: FAIL ✗
