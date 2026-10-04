# Verdict: DF-OFF-BY-ONE-32-rework

**Task:** Fix dead-code metrics: memory gate, peak RSS, /metrics endpoint, wiring, tests
**Evaluated:** 2026-10-04T07:42:41.517045
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ All 6 judge failures resolved: memory gate in checkIdle, PeakRSSBytes populated from sandbox.Sampler, GET /metrics endpoint, Observer constructed in main.go, tests pass, over-budget alert fires: All 6 sub-failures verified with live evidence. (1) Memory gate in checkIdle: internal/cron/loop.go:408-424 checks MemoryThreshold/MemoryProbe, sets skipReason 'memory'/'load+memory', calls Observer.RecordSkip at :437; tests TestCheckIdleMemoryGateTrips + TestCheckIdleLoadAndMemoryComposite PASS. (2) PeakRSSBytes from sandbox.Sampler: loop.go:461 startRSSTracker(l.cfg.RSSSampler), :482 PeakRSSBytes: peakRSS, default sandbox.ProcSampler{} at :229; TestLoopRecordsPeakRSSFromSampler PASS asserting PeakRSSBytes=4<<20. (3) GET /metrics: internal/api/server.go:108 mux.HandleFunc("GET /metrics", s.handleMetrics), handler :227 returns Snapshot JSON (404 metrics_disabled when nil); TestHandleMetricsReturnsSnapshot/EmptyObserver/NilObserver/RouterDispatch all PASS. (4) Observer in main.go: cmd/off-by-one/main.go:163 observer := metrics.New(...) constructed unconditionally (moved outside the solverExec!=nil block), wired to cron loop :181 Observer: observer and API :203 apiServer.Observer = observer; /metrics explicitly routed at main.go:238 (resolves pre-screen 'wiring' concern). (5) Tests pass: `go test ./... -short -p 1 -count=1 -timeout 180s` exit_code=0, 15 packages ok, NO_FAILURES; go build exit 0; go vet exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty; LSP diagnostics 0. (6) Over-budget alert fires: live run output 'ERROR solve ALERT: peak RSS 4194304 (0.0 GiB) exceeds 1048576 (0.0 GiB) — problem_class=class-rss-1 submission=rss-1' from TestLoopRecordsPeakRSSFromSampler; metrics.RecordSolve logs ALERT when PeakRSSBytes > alertBytes; TestRecordSolveOverBudget PASS. [resolution 0.23; main.go]
All 6 judge failures are resolved and verified by passing tests, clean build/vet/gofmt, and a live over-budget ALERT log line.

## Summary

Judge Result: DF-OFF-BY-ONE-32-rework

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ All 6 judge failures resolved: memory gate in checkIdle, PeakRSSBytes populated from sandbox.Sampler, GET /metrics endpoint, Observer constructed in main.go, tests pass, over-budget alert fires: All 6 sub-failures verified with live evidence. (1) Memory gate in checkIdle: internal/cron/loop.go:408-424 checks MemoryThreshold/MemoryProbe, sets skipReason 'memory'/'load+memory', calls Observer.RecordSkip at :437; tests TestCheckIdleMemoryGateTrips + TestCheckIdleLoadAndMemoryComposite PASS. (2) PeakRSSBytes from sandbox.Sampler: loop.go:461 startRSSTracker(l.cfg.RSSSampler), :482 PeakRSSBytes: peakRSS, default sandbox.ProcSampler{} at :229; TestLoopRecordsPeakRSSFromSampler PASS asserting PeakRSSBytes=4<<20. (3) GET /metrics: internal/api/server.go:108 mux.HandleFunc("GET /metrics", s.handleMetrics), handler :227 returns Snapshot JSON (404 metrics_disabled when nil); TestHandleMetricsReturnsSnapshot/EmptyObserver/NilObserver/RouterDispatch all PASS. (4) Observer in main.go: cmd/off-by-one/main.go:163 observer := metrics.New(...) constructed unconditionally (moved outside the solverExec!=nil block), wired to cron loop :181 Observer: observer and API :203 apiServer.Observer = observer; /metrics explicitly routed at main.go:238 (resolves pre-screen 'wiring' concern). (5) Tests pass: `go test ./... -short -p 1 -count=1 -timeout 180s` exit_code=0, 15 packages ok, NO_FAILURES; go build exit 0; go vet exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty; LSP diagnostics 0. (6) Over-budget alert fires: live run output 'ERROR solve ALERT: peak RSS 4194304 (0.0 GiB) exceeds 1048576 (0.0 GiB) — problem_class=class-rss-1 submission=rss-1' from TestLoopRecordsPeakRSSFromSampler; metrics.RecordSolve logs ALERT when PeakRSSBytes > alertBytes; TestRecordSolveOverBudget PASS. [resolution 0.23; main.go]
All 6 judge failures are resolved and verified by passing tests, clean build/vet/gofmt, and a live over-budget ALERT log line.

Overall: PASS ✓
