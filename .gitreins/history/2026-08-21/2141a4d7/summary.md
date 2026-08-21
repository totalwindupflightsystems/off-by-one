# Verdict: OB-GAP-047

**Task:** P3 stats avg_solve_time always empty
**Evaluated:** 2026-08-21T23:22:47.204568
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.017s
ok  	github.com/totalwindu
  ✓ secrets: [90m6:21PM[0m [32mINF[0m [1mscanned ~6400003 bytes (6.40 MB) in 1.28s[0m
[90m6:21PM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ internal/api stats handler computes avg_solve_time from completed queue_entries solves (started_at/completed_at) via Queue.AvgSolveTime; unit test asserts non-empty avg_solve_time when completed solves exist; go build ./... && go test ./... -short pass: internal/api/handlers.go:662-663 calls s.Queue.AvgSolveTime(r.Context()) and sets st.AvgSolveTime = avg.Round(time.Second).String() when avg>0. internal/ingest/queue.go:362-378 AvgSolveTime computes AVG(julianday(completed_at)-julianday(started_at))*86400.0 FROM queue_entries WHERE status='complete' AND started_at/completed_at NOT NULL, scanning into sql.NullFloat64 (0 when empty). Stats.AvgSolveTime field with json:"avg_solve_time" at internal/graph/store.go:471. Unit test TestStats_AvgSolveTime (internal/api/handlers_test.go:857) inserts a completed solve (10:00:00->10:02:13) plus a pending row, asserts avg_solve_time non-empty and == "2m13s"; ran `go test ./internal/api/ -short -run TestStats_AvgSolveTime -v` -> PASS. go build ./... exit 0; go test ./... -short -count=1 exit 0 (all 13 pkgs ok).
OB-GAP-047 fully implemented: stats handler computes avg_solve_time from completed queue_entries via Queue.AvgSolveTime, unit test asserts non-empty value and passes, and go build + go test -short both pass.

## Summary

Judge Result: OB-GAP-047

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.017s
ok  	github.com/totalwindu
  ✓ secrets: [90m6:21PM[0m [32mINF[0m [1mscanned ~6400003 bytes (6.40 MB) in 1.28s[0m
[90m6:21PM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ internal/api stats handler computes avg_solve_time from completed queue_entries solves (started_at/completed_at) via Queue.AvgSolveTime; unit test asserts non-empty avg_solve_time when completed solves exist; go build ./... && go test ./... -short pass: internal/api/handlers.go:662-663 calls s.Queue.AvgSolveTime(r.Context()) and sets st.AvgSolveTime = avg.Round(time.Second).String() when avg>0. internal/ingest/queue.go:362-378 AvgSolveTime computes AVG(julianday(completed_at)-julianday(started_at))*86400.0 FROM queue_entries WHERE status='complete' AND started_at/completed_at NOT NULL, scanning into sql.NullFloat64 (0 when empty). Stats.AvgSolveTime field with json:"avg_solve_time" at internal/graph/store.go:471. Unit test TestStats_AvgSolveTime (internal/api/handlers_test.go:857) inserts a completed solve (10:00:00->10:02:13) plus a pending row, asserts avg_solve_time non-empty and == "2m13s"; ran `go test ./internal/api/ -short -run TestStats_AvgSolveTime -v` -> PASS. go build ./... exit 0; go test ./... -short -count=1 exit 0 (all 13 pkgs ok).
OB-GAP-047 fully implemented: stats handler computes avg_solve_time from completed queue_entries via Queue.AvgSolveTime, unit test asserts non-empty value and passes, and go build + go test -short both pass.

Overall: PASS ✓
