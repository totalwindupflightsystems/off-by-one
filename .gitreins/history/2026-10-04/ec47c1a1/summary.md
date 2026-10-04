# Verdict: DF-OFF-BY-ONE-32-rework

**Task:** Fix dead-code metrics: memory gate, peak RSS, /metrics endpoint, wiring, tests
**Evaluated:** 2026-10-04T06:57:47.540227
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✗ **tier2**
  - INCOMPLETE
  ✗ All 6 judge failures resolved: memory gate in checkIdle, PeakRSSBytes populated from sandbox.Sampler, GET /metrics endpoint, Observer constructed in main.go, tests pass, over-budget alert fires: The repository does not compile, so none of the 6 failures are resolved. `go build ./...` exits 1 with: internal/cron/loop.go:197-201,371-378 'cfg.MemoryProbe undefined (type Config has no field or method MemoryProbe)' and 'cfg.MemoryThreshold undefined' — the Config struct (loop.go:130-160) declares only Interval/LoadThreshold/ReapAfter/Solver/Queue/IdleProbe/Logger/Now/Sleep/Observer, so the memory gate in checkIdle is dead code that cannot build. internal/api/server.go:103 registers `mux.HandleFunc("GET /metrics", s.handleMetrics)` but `grep -rn 'func.*handleMetrics' internal/` returns ZERO definitions, so the /metrics endpoint does not exist. `grep -rn 'Sampler|PeakRSS' *.go` matches only internal/metrics/metrics_test.go — no sandbox.Sampler exists and all four RecordSolve call sites in loop.go set only SubmissionID/ProblemClass/WallTimeMS/Success, so PeakRSSBytes is never populated. main.go:172 does call `metrics.New(...)` and pass `Observer: observer`, but it also passes `MemoryThreshold: 0.85` to a Config with no such field, so cmd/off-by-one does not build either. `go test ./internal/metrics/... ./internal/cron/... -count=1` => 'FAIL github.com/.../internal/metrics [build failed]' (metrics_test.go:32,33,35,36,83,84,86,87,103,104 reference snap.Recent/snap.Host which do not exist on the Snapshot type, which only has LastSolve/LastHost) and 'FAIL github.com/.../internal/cron [build failed]'. `go vet ./cmd/... ./internal/api/... ./internal/cron/... ./internal/metrics/...` reports the same errors. The over-budget alert logic exists in metrics.go RecordSolve (compares rec.PeakRSSBytes > o.alertBytes) but can never fire in production because PeakRSSBytes is always 0 and the package does not compile.
The rework is incomplete: the code does not compile (go build ./... exit 1), the memory gate references non-existent Config fields, handleMetrics is undefined, no sandbox.Sampler exists so PeakRSSBytes is never populated, and both metrics and cron test packages fail to build.

## Summary

Judge Result: DF-OFF-BY-ONE-32-rework

Stage tier1: FAIL
    ✗ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: FAIL
  INCOMPLETE
  ✗ All 6 judge failures resolved: memory gate in checkIdle, PeakRSSBytes populated from sandbox.Sampler, GET /metrics endpoint, Observer constructed in main.go, tests pass, over-budget alert fires: The repository does not compile, so none of the 6 failures are resolved. `go build ./...` exits 1 with: internal/cron/loop.go:197-201,371-378 'cfg.MemoryProbe undefined (type Config has no field or method MemoryProbe)' and 'cfg.MemoryThreshold undefined' — the Config struct (loop.go:130-160) declares only Interval/LoadThreshold/ReapAfter/Solver/Queue/IdleProbe/Logger/Now/Sleep/Observer, so the memory gate in checkIdle is dead code that cannot build. internal/api/server.go:103 registers `mux.HandleFunc("GET /metrics", s.handleMetrics)` but `grep -rn 'func.*handleMetrics' internal/` returns ZERO definitions, so the /metrics endpoint does not exist. `grep -rn 'Sampler|PeakRSS' *.go` matches only internal/metrics/metrics_test.go — no sandbox.Sampler exists and all four RecordSolve call sites in loop.go set only SubmissionID/ProblemClass/WallTimeMS/Success, so PeakRSSBytes is never populated. main.go:172 does call `metrics.New(...)` and pass `Observer: observer`, but it also passes `MemoryThreshold: 0.85` to a Config with no such field, so cmd/off-by-one does not build either. `go test ./internal/metrics/... ./internal/cron/... -count=1` => 'FAIL github.com/.../internal/metrics [build failed]' (metrics_test.go:32,33,35,36,83,84,86,87,103,104 reference snap.Recent/snap.Host which do not exist on the Snapshot type, which only has LastSolve/LastHost) and 'FAIL github.com/.../internal/cron [build failed]'. `go vet ./cmd/... ./internal/api/... ./internal/cron/... ./internal/metrics/...` reports the same errors. The over-budget alert logic exists in metrics.go RecordSolve (compares rec.PeakRSSBytes > o.alertBytes) but can never fire in production because PeakRSSBytes is always 0 and the package does not compile.
The rework is incomplete: the code does not compile (go build ./... exit 1), the memory gate references non-existent Config fields, handleMetrics is undefined, no sandbox.Sampler exists so PeakRSSBytes is never populated, and both metrics and cron test packages fail to build.

Overall: FAIL ✗
