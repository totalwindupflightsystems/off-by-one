# Verdict: DF-OFF-BY-ONE-32-rework

**Task:** Fix dead-code metrics: memory gate, peak RSS, /metrics endpoint, wiring, tests
**Evaluated:** 2026-10-04T07:34:03.589499
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ All 6 judge failures resolved: memory gate in checkIdle, PeakRSSBytes populated from sandbox.Sampler, GET /metrics endpoint, Observer constructed in main.go, tests pass, over-budget alert fires: 4 of 6 sub-items verified present, but 2 are unresolved. (a) TESTS DO NOT PASS: `go test ./... -short -p 1 -count=1 -timeout 180s` => `FAIL github.com/totalwindupflightsystems/off-by-one/internal/cron [build failed]`; `go vet ./internal/cron/` => `internal/cron/memgate_test.go:7:2: "os" imported and not used`. The entire cron test package (all 17 tests incl. TestCheckIdleMemoryGateTrips, TestCheckIdleLoadAndMemoryComposite, TestRSSTrackerLiveProcSelf, TestSolveFailureLogCarriesPeakRSS) never executes. `go test ./internal/cron/ -run 'TestCheckIdleMemoryGateTrips|TestSolveFailureLogCarriesPeakRSS'` => build failed, 0 tests run. (b) OVER-BUDGET ALERT END-TO-END TEST MISSING: memgate_test.go:325-329 contains only the comment 'Over-budget alert end-to-end through the loop: a fake solver whose process tree reports a peak above the alert threshold must produce an over_budget record in the observer (judge failure #5/#6)' with no test function following — the test list jumps from TestChildPIDsDeadPid (line 318) to TestIsMemoryKill (line 334). Only the metrics-package unit test TestRecordSolveOverBudget (internal/metrics/metrics_test.go:43) covers the alert, not the loop path. Also `gofmt -l cmd/ internal/ pkg/ sql/` is NOT empty: cmd/off-by-one/main.go, internal/cron/memgate_test.go, internal/metrics/metrics.go, internal/metrics/metrics_test.go. VERIFIED PRESENT: memory gate in checkIdle (internal/cron/loop.go:389-437 with MemoryProbe/MemoryThreshold, memgate.go parseMeminfo/probeMeminfo, DefaultMemFraction=0.9); PeakRSSBytes populated from sandbox.Sampler (loop.go:168 RSSSampler field, loop.go:228-229 defaults to sandbox.ProcSampler{}, loop.go:461 startRSSTracker, loop.go:482 PeakRSSBytes: peakRSS, rsspoll.go:110 sampler.PeakRSSBytes); GET /metrics endpoint (internal/api/server.go:108 mux.HandleFunc("GET /metrics", s.handleMetrics), server.go:226-234 handleMetrics, and explicitly dispatched in cmd/off-by-one/main.go:238); Observer constructed in main.go (main.go:163 metrics.New(metrics.DefaultRSSAlertBytes, log.Default()) unconditionally, wired to loop at main.go:181 and apiServer.Observer at main.go:203).


## Summary

Judge Result: DF-OFF-BY-ONE-32-rework

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ All 6 judge failures resolved: memory gate in checkIdle, PeakRSSBytes populated from sandbox.Sampler, GET /metrics endpoint, Observer constructed in main.go, tests pass, over-budget alert fires: 4 of 6 sub-items verified present, but 2 are unresolved. (a) TESTS DO NOT PASS: `go test ./... -short -p 1 -count=1 -timeout 180s` => `FAIL github.com/totalwindupflightsystems/off-by-one/internal/cron [build failed]`; `go vet ./internal/cron/` => `internal/cron/memgate_test.go:7:2: "os" imported and not used`. The entire cron test package (all 17 tests incl. TestCheckIdleMemoryGateTrips, TestCheckIdleLoadAndMemoryComposite, TestRSSTrackerLiveProcSelf, TestSolveFailureLogCarriesPeakRSS) never executes. `go test ./internal/cron/ -run 'TestCheckIdleMemoryGateTrips|TestSolveFailureLogCarriesPeakRSS'` => build failed, 0 tests run. (b) OVER-BUDGET ALERT END-TO-END TEST MISSING: memgate_test.go:325-329 contains only the comment 'Over-budget alert end-to-end through the loop: a fake solver whose process tree reports a peak above the alert threshold must produce an over_budget record in the observer (judge failure #5/#6)' with no test function following — the test list jumps from TestChildPIDsDeadPid (line 318) to TestIsMemoryKill (line 334). Only the metrics-package unit test TestRecordSolveOverBudget (internal/metrics/metrics_test.go:43) covers the alert, not the loop path. Also `gofmt -l cmd/ internal/ pkg/ sql/` is NOT empty: cmd/off-by-one/main.go, internal/cron/memgate_test.go, internal/metrics/metrics.go, internal/metrics/metrics_test.go. VERIFIED PRESENT: memory gate in checkIdle (internal/cron/loop.go:389-437 with MemoryProbe/MemoryThreshold, memgate.go parseMeminfo/probeMeminfo, DefaultMemFraction=0.9); PeakRSSBytes populated from sandbox.Sampler (loop.go:168 RSSSampler field, loop.go:228-229 defaults to sandbox.ProcSampler{}, loop.go:461 startRSSTracker, loop.go:482 PeakRSSBytes: peakRSS, rsspoll.go:110 sampler.PeakRSSBytes); GET /metrics endpoint (internal/api/server.go:108 mux.HandleFunc("GET /metrics", s.handleMetrics), server.go:226-234 handleMetrics, and explicitly dispatched in cmd/off-by-one/main.go:238); Observer constructed in main.go (main.go:163 metrics.New(metrics.DefaultRSSAlertBytes, log.Default()) unconditionally, wired to loop at main.go:181 and apiServer.Observer at main.go:203).


Overall: FAIL ✗
