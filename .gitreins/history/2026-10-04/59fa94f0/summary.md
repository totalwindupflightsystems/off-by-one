# Verdict: DF-OFF-BY-ONE-31

**Task:** Solver produces unbounded simulations; near-cap runs must be rejected
**Evaluated:** 2026-10-04T13:23:01.723517
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Worker brief AC: (a) unbounded simulation fixture rejected by harness with measured peak recorded; (b) bounded version accepted; (c) prompt/contract change asserted by test: (a) internal/cron/loop.go:574-586 rejects a solve that returned success when `peakRSS > l.cfg.SolveRejectRSSBytes`: SetStage("solver_rejected"), MarkFailed with reason "rejected: peak RSS %s GiB exceeds reject threshold %s GiB (problem_class=%s)" built from gibStr(peakRSS) (the measured peak from tracker.Stop() at loop.go:520), recordRejection, Observer.RecordSolve(record(false,true)), returns ErrSolveRejected; Commit is never called. Test TestLoopRejectsNearCapSolve (internal/cron/memgate_test.go:470-580) drives peak 5GiB vs threshold 4GiB and asserts ErrSolveRejected, StatusFailed, no answer, reason contains class+gibStr(peak)+gibStr(threshold), commitCalls==0, SolveRejected==1, SolveFailed==1, SolveSuccess==0, LastSolve.Rejected==true/Success==false/PeakRSSBytes==peak. (b) TestLoopAcceptsBoundedSolveBelowRejectThreshold (memgate_test.go:585-620) with peak 1GiB < 4GiB threshold asserts Tick nil, StatusComplete, answer 42, Snapshot.Rejected==0; TestLoopRejectionDisabledWhenThresholdZero covers threshold 0. (c) scripts/pi-agent:268-276 adds the '## Resource bounds (required for simulation-style problems)' contract and internal/solver/contract_test.go TestPiAgentWrapperDeclaresResourceBounds reads the shipped wrapper asserting 5 marker phrases. Evidence: `go test ./internal/cron/... ./internal/solver/... ./internal/metrics/... -count=1 -run 'Reject|Bounded|ResourceBounds|SnapshotRejected|Threshold' -v` -> all PASS, exit 0; full `go test ./... -short -p 1 -count=1 -timeout 180s` -> every package ok, exit 0; `go build ./...` exit 0; `go vet` exit 0; `gofmt -l cmd/ internal/ pkg/ sql/` empty; LSP diagnostics 0. Wiring/docs present: cmd/off-by-one/main.go:364 flag -solve-reject-rss-mb (env OFF_BY_ONE_SOLVE_REJECT_RSS_MB, default -1 auto), resolveSolveRejectBytes main.go:407-418, main.go:187; docs/api-reference.md:682-699.
Harness-side near-cap solve rejection with measured peak recorded, bounded-solve acceptance, and the pi-agent prompt bounds contract are all implemented, wired, documented, and verified by passing tests (targeted + full short suite, build/vet/gofmt clean).

## Summary

Judge Result: DF-OFF-BY-ONE-31

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Worker brief AC: (a) unbounded simulation fixture rejected by harness with measured peak recorded; (b) bounded version accepted; (c) prompt/contract change asserted by test: (a) internal/cron/loop.go:574-586 rejects a solve that returned success when `peakRSS > l.cfg.SolveRejectRSSBytes`: SetStage("solver_rejected"), MarkFailed with reason "rejected: peak RSS %s GiB exceeds reject threshold %s GiB (problem_class=%s)" built from gibStr(peakRSS) (the measured peak from tracker.Stop() at loop.go:520), recordRejection, Observer.RecordSolve(record(false,true)), returns ErrSolveRejected; Commit is never called. Test TestLoopRejectsNearCapSolve (internal/cron/memgate_test.go:470-580) drives peak 5GiB vs threshold 4GiB and asserts ErrSolveRejected, StatusFailed, no answer, reason contains class+gibStr(peak)+gibStr(threshold), commitCalls==0, SolveRejected==1, SolveFailed==1, SolveSuccess==0, LastSolve.Rejected==true/Success==false/PeakRSSBytes==peak. (b) TestLoopAcceptsBoundedSolveBelowRejectThreshold (memgate_test.go:585-620) with peak 1GiB < 4GiB threshold asserts Tick nil, StatusComplete, answer 42, Snapshot.Rejected==0; TestLoopRejectionDisabledWhenThresholdZero covers threshold 0. (c) scripts/pi-agent:268-276 adds the '## Resource bounds (required for simulation-style problems)' contract and internal/solver/contract_test.go TestPiAgentWrapperDeclaresResourceBounds reads the shipped wrapper asserting 5 marker phrases. Evidence: `go test ./internal/cron/... ./internal/solver/... ./internal/metrics/... -count=1 -run 'Reject|Bounded|ResourceBounds|SnapshotRejected|Threshold' -v` -> all PASS, exit 0; full `go test ./... -short -p 1 -count=1 -timeout 180s` -> every package ok, exit 0; `go build ./...` exit 0; `go vet` exit 0; `gofmt -l cmd/ internal/ pkg/ sql/` empty; LSP diagnostics 0. Wiring/docs present: cmd/off-by-one/main.go:364 flag -solve-reject-rss-mb (env OFF_BY_ONE_SOLVE_REJECT_RSS_MB, default -1 auto), resolveSolveRejectBytes main.go:407-418, main.go:187; docs/api-reference.md:682-699.
Harness-side near-cap solve rejection with measured peak recorded, bounded-solve acceptance, and the pi-agent prompt bounds contract are all implemented, wired, documented, and verified by passing tests (targeted + full short suite, build/vet/gofmt clean).

Overall: PASS ✓
