# Verdict: DF-OFF-BY-ONE-31

**Task:** Solver produces unbounded simulations; near-cap runs must be rejected
**Evaluated:** 2026-10-04T13:21:38.474753
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ Worker brief AC: (a) unbounded simulation fixture rejected by harness with measured peak recorded; (b) bounded version accepted; (c) prompt/contract change asserted by test: The entire DF-OFF-BY-ONE-31 implementation is absent from the delivered working tree (HEAD=master @ c18bdba8). It exists only on the unmerged branch wt/DF-OFF-BY-ONE-31 (commit 6c354615), which is NOT an ancestor of HEAD (git merge-base --is-ancestor 6c354615 HEAD => 'NO not ancestor'; git branch --contains => only wt/DF-OFF-BY-ONE-31). The only working-tree change is the 9-line .gitreins/tasks.yaml entry marking the task complete (git status: ' M .gitreins/tasks.yaml'). (a) internal/cron/loop.go has no SolveRejectRSSBytes/ErrSolveRejected rejection block (grep exit 1), cmd/off-by-one/main.go has no -solve-reject-rss-mb flag (grep exit 1), internal/metrics has no Rejected field; `go test ./internal/cron/ -run 'TestLoopRejectsNearCapSolve|TestLoopAcceptsBoundedSolveBelowRejectThreshold|TestSolveRejectThreshold' -count=1` => 'ok ... [no tests to run]'. (b) TestLoopAcceptsBoundedSolveBelowRejectThreshold does not exist at HEAD (same '[no tests to run]' output). (c) internal/solver/contract_test.go does not exist ('ls: cannot access ... No such file or directory'), scripts/pi-agent contains 0 occurrences of 'Resource bounds' (grep -c => 0), and `go test ./internal/solver/ -run 'TestPiAgentWrapperDeclaresResourceBounds' -count=1` => 'ok ... [no tests to run]'. docs/api-reference.md has no 'Solve memory bounds' section. The branch-only work (loop.go rejection, main.go flag, metrics Rejected field, contract_test.go, pi-agent prompt contract, docs) was never merged, so no criterion is satisfied in the deliverable.
The task is marked complete but its implementation lives only on the unmerged branch wt/DF-OFF-BY-ONE-31; HEAD contains none of the rejection logic, tests, or prompt contract, and the required tests report '[no tests to run]'.

## Summary

Judge Result: DF-OFF-BY-ONE-31

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ Worker brief AC: (a) unbounded simulation fixture rejected by harness with measured peak recorded; (b) bounded version accepted; (c) prompt/contract change asserted by test: The entire DF-OFF-BY-ONE-31 implementation is absent from the delivered working tree (HEAD=master @ c18bdba8). It exists only on the unmerged branch wt/DF-OFF-BY-ONE-31 (commit 6c354615), which is NOT an ancestor of HEAD (git merge-base --is-ancestor 6c354615 HEAD => 'NO not ancestor'; git branch --contains => only wt/DF-OFF-BY-ONE-31). The only working-tree change is the 9-line .gitreins/tasks.yaml entry marking the task complete (git status: ' M .gitreins/tasks.yaml'). (a) internal/cron/loop.go has no SolveRejectRSSBytes/ErrSolveRejected rejection block (grep exit 1), cmd/off-by-one/main.go has no -solve-reject-rss-mb flag (grep exit 1), internal/metrics has no Rejected field; `go test ./internal/cron/ -run 'TestLoopRejectsNearCapSolve|TestLoopAcceptsBoundedSolveBelowRejectThreshold|TestSolveRejectThreshold' -count=1` => 'ok ... [no tests to run]'. (b) TestLoopAcceptsBoundedSolveBelowRejectThreshold does not exist at HEAD (same '[no tests to run]' output). (c) internal/solver/contract_test.go does not exist ('ls: cannot access ... No such file or directory'), scripts/pi-agent contains 0 occurrences of 'Resource bounds' (grep -c => 0), and `go test ./internal/solver/ -run 'TestPiAgentWrapperDeclaresResourceBounds' -count=1` => 'ok ... [no tests to run]'. docs/api-reference.md has no 'Solve memory bounds' section. The branch-only work (loop.go rejection, main.go flag, metrics Rejected field, contract_test.go, pi-agent prompt contract, docs) was never merged, so no criterion is satisfied in the deliverable.
The task is marked complete but its implementation lives only on the unmerged branch wt/DF-OFF-BY-ONE-31; HEAD contains none of the rejection logic, tests, or prompt contract, and the required tests report '[no tests to run]'.

Overall: FAIL ✗
