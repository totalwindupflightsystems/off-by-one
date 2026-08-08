# Verdict: OB-GAP-005

**Task:** Server must WARN when solver unavailable and expose solver availability in stats
**Evaluated:** 2026-08-08T21:42:59.802936
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: /bin/sh: 1: gitleaks: not found

  ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
- ✓ **tier2**
  - COMPLETE
  ✓ cmd/off-by-one/main.go logs a prominent WARN line when the solver is unavailable (literal WARN present, mentions missing/disabled solver, states queued submissions will not be processed): cmd/off-by-one/main.go:154 log.Printf("WARN: cron loop not started — no solver available (bwrap/pi-agent missing or sandbox skipped); queued submissions will NOT be processed until the server is restarted with a solver") — literal WARN, mentions missing/disabled solver, states queued submissions not processed.
  ✓ GET /api/v1/stats response exposes a solver_available boolean field, wired from main.go (apiServer.SolverAvailable = solverExec != nil): cmd/off-by-one/main.go:163 apiServer.SolverAvailable = solverExec != nil; internal/api/handlers.go:647-648 SolverAvailable bool json:"solver_available" and SolverAvailable: s.SolverAvailable in the /api/v1/stats response.
  ✓ internal/api/handlers_test.go covers the solver_available field (present, false by default, true when set); all existing tests pass: internal/api/handlers_test.go TestStats_SolverAvailable (lines 645-676) asserts field present (656-659), false by default (661), true when set (673-674). Test passes; go test ./... all packages ok; go build ./... exit 0.
All three criteria are satisfied: WARN log in main.go, solver_available wired from main.go into the stats response, and a passing test covering the field's presence/default/true states.

## Summary

Judge Result: OB-GAP-005

Stage tier1: PASS
    ✓ secrets: /bin/sh: 1: gitleaks: not found

  ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t

Stage tier2: PASS
  COMPLETE
  ✓ cmd/off-by-one/main.go logs a prominent WARN line when the solver is unavailable (literal WARN present, mentions missing/disabled solver, states queued submissions will not be processed): cmd/off-by-one/main.go:154 log.Printf("WARN: cron loop not started — no solver available (bwrap/pi-agent missing or sandbox skipped); queued submissions will NOT be processed until the server is restarted with a solver") — literal WARN, mentions missing/disabled solver, states queued submissions not processed.
  ✓ GET /api/v1/stats response exposes a solver_available boolean field, wired from main.go (apiServer.SolverAvailable = solverExec != nil): cmd/off-by-one/main.go:163 apiServer.SolverAvailable = solverExec != nil; internal/api/handlers.go:647-648 SolverAvailable bool json:"solver_available" and SolverAvailable: s.SolverAvailable in the /api/v1/stats response.
  ✓ internal/api/handlers_test.go covers the solver_available field (present, false by default, true when set); all existing tests pass: internal/api/handlers_test.go TestStats_SolverAvailable (lines 645-676) asserts field present (656-659), false by default (661), true when set (673-674). Test passes; go test ./... all packages ok; go build ./... exit 0.
All three criteria are satisfied: WARN log in main.go, solver_available wired from main.go into the stats response, and a passing test covering the field's presence/default/true states.

Overall: PASS ✓
