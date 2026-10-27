# Verdict: DOC-5

**Task:** Document resource safety configuration
**Evaluated:** 2026-10-27T16:02:09.694921
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ docs/integration.md documents resource safety environment variables and flags with exact semantics: docs/integration.md:454-457 documents the resource-safety env vars with exact semantics: OFF_BY_ONE_SOLVE_TIMEOUT (default 30m), OFF_BY_ONE_SOLVE_MEM_MB (default 0=unlimited, applied via oby-memcap wrapper as RLIMIT_AS), OFF_BY_ONE_SOLVE_REJECT_RSS_MB (default -1=auto = 80% of --solve-mem-mb, 0=disabled, >0=explicit threshold), OB1_BWRAP_TIMEOUT (default 300 seconds, positive integer, invalid values fall back to 300 with a warning). Line 463 adds a 'Two timeouts, two layers' explanation distinguishing the outer bwrap cap from the solver-level timeout. The flags table at lines 482-484 documents --solve-timeout, --solve-mem-mb, --solve-reject-rss-mb, and line 490 states flags override env vars. Semantics verified against code: cmd/off-by-one/main.go:377-378,387 (flag defaults + envInt wiring), main.go:408-416 sandboxTimeout() (Atoi, secs>0 else log warning + sandbox.DefaultBwrapTimeout = 5*time.Minute at internal/sandbox/bwrap.go:74), main.go:431-441 resolveSolveRejectBytes (rejectMB>0 verbatim, ==0 disabled, <0 auto via cron.SolveRejectThreshold, no cap => disabled), internal/cron/loop.go:195-207 (DefaultSolveRejectFraction = 0.8), internal/solver/piagent.go:35 (DefaultSolveTimeout = 30*time.Minute), internal/sandbox/memcap.go (RLIMIT_AS pinned via oby-memcap). All documented defaults and edge-case semantics match the implementation. No test suite is relevant to this docs-only criterion (no test command in .gitreins/config.yaml guards for docs; criterion is documentation content). [resolution 0.24; docs/integration.md]
docs/integration.md fully documents the resource-safety env vars (OFF_BY_ONE_SOLVE_TIMEOUT, OFF_BY_ONE_SOLVE_MEM_MB, OFF_BY_ONE_SOLVE_REJECT_RSS_MB, OB1_BWRAP_TIMEOUT) and matching flags with defaults and edge-case semantics that match the code in cmd/off-by-one/main.go, internal/cron/loop.go, internal/solver/piagent.go and internal/sandbox.

## Summary

Judge Result: DOC-5

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ docs/integration.md documents resource safety environment variables and flags with exact semantics: docs/integration.md:454-457 documents the resource-safety env vars with exact semantics: OFF_BY_ONE_SOLVE_TIMEOUT (default 30m), OFF_BY_ONE_SOLVE_MEM_MB (default 0=unlimited, applied via oby-memcap wrapper as RLIMIT_AS), OFF_BY_ONE_SOLVE_REJECT_RSS_MB (default -1=auto = 80% of --solve-mem-mb, 0=disabled, >0=explicit threshold), OB1_BWRAP_TIMEOUT (default 300 seconds, positive integer, invalid values fall back to 300 with a warning). Line 463 adds a 'Two timeouts, two layers' explanation distinguishing the outer bwrap cap from the solver-level timeout. The flags table at lines 482-484 documents --solve-timeout, --solve-mem-mb, --solve-reject-rss-mb, and line 490 states flags override env vars. Semantics verified against code: cmd/off-by-one/main.go:377-378,387 (flag defaults + envInt wiring), main.go:408-416 sandboxTimeout() (Atoi, secs>0 else log warning + sandbox.DefaultBwrapTimeout = 5*time.Minute at internal/sandbox/bwrap.go:74), main.go:431-441 resolveSolveRejectBytes (rejectMB>0 verbatim, ==0 disabled, <0 auto via cron.SolveRejectThreshold, no cap => disabled), internal/cron/loop.go:195-207 (DefaultSolveRejectFraction = 0.8), internal/solver/piagent.go:35 (DefaultSolveTimeout = 30*time.Minute), internal/sandbox/memcap.go (RLIMIT_AS pinned via oby-memcap). All documented defaults and edge-case semantics match the implementation. No test suite is relevant to this docs-only criterion (no test command in .gitreins/config.yaml guards for docs; criterion is documentation content). [resolution 0.24; docs/integration.md]
docs/integration.md fully documents the resource-safety env vars (OFF_BY_ONE_SOLVE_TIMEOUT, OFF_BY_ONE_SOLVE_MEM_MB, OFF_BY_ONE_SOLVE_REJECT_RSS_MB, OB1_BWRAP_TIMEOUT) and matching flags with defaults and edge-case semantics that match the code in cmd/off-by-one/main.go, internal/cron/loop.go, internal/solver/piagent.go and internal/sandbox.

Overall: PASS ✓
