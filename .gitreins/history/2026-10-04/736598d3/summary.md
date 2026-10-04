# Verdict: DF-OFF-BY-ONE-32-rework

**Task:** Fix dead-code metrics: memory gate, peak RSS, /metrics endpoint, wiring, tests
**Evaluated:** 2026-10-04T07:26:12.354665
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ All 6 judge failures resolved: memory gate in checkIdle, PeakRSSBytes populated from sandbox.Sampler, GET /metrics endpoint, Observer constructed in main.go, tests pass, over-budget alert fires: 2 of 6 sub-failures remain unresolved. (b) Observer is NOT wired to the API server: cmd/off-by-one/main.go:170 constructs observer and passes it only to cron.Config.Observer (main.go:178); grep '.Observer' over cmd/ and internal/ shows NO apiServer.Observer assignment and no setter exists. internal/api/server.go:65 declares the field but it is never set in production, so handleMetrics (server.go:227-233) always takes the nil branch. Empirically reproduced with a probe test using main.go's exact wiring: GET /metrics returned status=404 body={"error":"metrics_disabled","message":"metrics observer not configured"}. (d) No test covers the /metrics endpoint: grep -rn 'metrics|Observer' internal/api/handlers_test.go internal/api/coverage_test.go internal/api/queue_page_test.go returns nothing (EXIT=1). RESOLVED sub-items: memory gate in checkIdle (internal/cron/loop.go:380-443 with MemoryThreshold/MemoryProbe, memgate.go probeMeminfo); PeakRSSBytes populated from sandbox.Sampler (internal/sandbox/sampler.go Sampler+ProcSampler, cron.Config.RSSSampler loop.go:168/228, startRSSTracker rsspoll.go:67, peakRSS->SolveRecord.PeakRSSBytes loop.go:465/482); tests pass (go build ./... exit 0; go test ./... -short -count=1 -p 1 -timeout 180s exit 0, all packages 'ok' incl. internal/metrics, internal/cron, internal/api, internal/sandbox); over-budget alert unit test TestRecordSolveOverBudget passes (metrics_test.go:43) but production reachability is undermined by the dead /metrics endpoint.
Memory gate, PeakRSS sampling, and the test suite are fixed, but the Observer is never assigned to the API server (GET /metrics returns 404 metrics_disabled in the real binary) and no test covers the /metrics endpoint, so 2 of the 6 judge failures remain unresolved.

## Summary

Judge Result: DF-OFF-BY-ONE-32-rework

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ All 6 judge failures resolved: memory gate in checkIdle, PeakRSSBytes populated from sandbox.Sampler, GET /metrics endpoint, Observer constructed in main.go, tests pass, over-budget alert fires: 2 of 6 sub-failures remain unresolved. (b) Observer is NOT wired to the API server: cmd/off-by-one/main.go:170 constructs observer and passes it only to cron.Config.Observer (main.go:178); grep '.Observer' over cmd/ and internal/ shows NO apiServer.Observer assignment and no setter exists. internal/api/server.go:65 declares the field but it is never set in production, so handleMetrics (server.go:227-233) always takes the nil branch. Empirically reproduced with a probe test using main.go's exact wiring: GET /metrics returned status=404 body={"error":"metrics_disabled","message":"metrics observer not configured"}. (d) No test covers the /metrics endpoint: grep -rn 'metrics|Observer' internal/api/handlers_test.go internal/api/coverage_test.go internal/api/queue_page_test.go returns nothing (EXIT=1). RESOLVED sub-items: memory gate in checkIdle (internal/cron/loop.go:380-443 with MemoryThreshold/MemoryProbe, memgate.go probeMeminfo); PeakRSSBytes populated from sandbox.Sampler (internal/sandbox/sampler.go Sampler+ProcSampler, cron.Config.RSSSampler loop.go:168/228, startRSSTracker rsspoll.go:67, peakRSS->SolveRecord.PeakRSSBytes loop.go:465/482); tests pass (go build ./... exit 0; go test ./... -short -count=1 -p 1 -timeout 180s exit 0, all packages 'ok' incl. internal/metrics, internal/cron, internal/api, internal/sandbox); over-budget alert unit test TestRecordSolveOverBudget passes (metrics_test.go:43) but production reachability is undermined by the dead /metrics endpoint.
Memory gate, PeakRSS sampling, and the test suite are fixed, but the Observer is never assigned to the API server (GET /metrics returns 404 metrics_disabled in the real binary) and no test covers the /metrics endpoint, so 2 of the 6 judge failures remain unresolved.

Overall: FAIL ✗
