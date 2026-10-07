# Verdict: INT-CI-003

**Task:** Fix runner-env-dependent TestApplyMemLimitMB_RefusesAllocationPastCap (RLIMIT_AS panics live Go runtime before mmap ENOMEM probe)
**Evaluated:** 2026-10-06T18:37:26.874084
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ under-cap arm observes ENOMEM deterministically without post-setrlimit Go runtime allocation; full go test ./... green; CI Go 1.26 job green on push: (1) Deterministic ENOMEM without post-setrlimit Go allocation: internal/sandbox/memcap_test.go:78-107 'alloc-under-cap' arm resolves py path and argv BEFORE ApplyMemLimitMB(64) (line 98); after the cap the only code is the error branch, then syscall.Exec(py, argv, nil) at line 104 — no fmt.Println/os.Exit on the success path. The 256 MiB mmap ENOMEM is observed by the exec'd python3 -S probe (allocUnderCapProbePy, lines 31-43), not live Go code. Empirical: direct helper child run printed 'ALLOC-REFUSED' (exit 0); non-vacuousness confirmed — the same probe WITHOUT the cap prints '256 MiB mmap SUCCEEDED ... cap not enforced' and exits 1. Control arm (lines 108-119) still mandatory and runs first. (2) Full suite green: `go test ./... -count=1 -timeout 300s` exit_code=0, all 15 packages 'ok' (internal/sandbox ok 1.424s), 3 '[no test files]'; targeted `go test ./internal/sandbox/ -run TestApplyMemLimitMB_RefusesAllocationPastCap -count=1 -v` => '--- PASS ... ok 0.015s'. (3) CI Go 1.26 green on push: pre-fix push run 37510484624 (commit 4d950ada) FAILED the Go 1.26 job at 'Test (short)' with 'fatal error: runtime: cannot allocate memory' / runtime.persistentalloc1 — the exact INT-CI-003 symptom; post-fix push run 37512285601 (commit c8c1fe58, the INT-CI-003 merge) shows '✓ Go 1.26 in 1m3s' with all 6 jobs success. .github/workflows/ci.yml:19 pins go-version: ['1.26'].
The alloc-under-cap arm now pins RLIMIT_AS then immediately syscall.Exec's a non-Go python3 ENOMEM probe (no post-setrlimit Go allocation), the full go test ./... suite is green, and the CI Go 1.26 job that previously failed with the runtime.persistentalloc1 panic now passes on the fix's push.

## Summary

Judge Result: INT-CI-003

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ under-cap arm observes ENOMEM deterministically without post-setrlimit Go runtime allocation; full go test ./... green; CI Go 1.26 job green on push: (1) Deterministic ENOMEM without post-setrlimit Go allocation: internal/sandbox/memcap_test.go:78-107 'alloc-under-cap' arm resolves py path and argv BEFORE ApplyMemLimitMB(64) (line 98); after the cap the only code is the error branch, then syscall.Exec(py, argv, nil) at line 104 — no fmt.Println/os.Exit on the success path. The 256 MiB mmap ENOMEM is observed by the exec'd python3 -S probe (allocUnderCapProbePy, lines 31-43), not live Go code. Empirical: direct helper child run printed 'ALLOC-REFUSED' (exit 0); non-vacuousness confirmed — the same probe WITHOUT the cap prints '256 MiB mmap SUCCEEDED ... cap not enforced' and exits 1. Control arm (lines 108-119) still mandatory and runs first. (2) Full suite green: `go test ./... -count=1 -timeout 300s` exit_code=0, all 15 packages 'ok' (internal/sandbox ok 1.424s), 3 '[no test files]'; targeted `go test ./internal/sandbox/ -run TestApplyMemLimitMB_RefusesAllocationPastCap -count=1 -v` => '--- PASS ... ok 0.015s'. (3) CI Go 1.26 green on push: pre-fix push run 37510484624 (commit 4d950ada) FAILED the Go 1.26 job at 'Test (short)' with 'fatal error: runtime: cannot allocate memory' / runtime.persistentalloc1 — the exact INT-CI-003 symptom; post-fix push run 37512285601 (commit c8c1fe58, the INT-CI-003 merge) shows '✓ Go 1.26 in 1m3s' with all 6 jobs success. .github/workflows/ci.yml:19 pins go-version: ['1.26'].
The alloc-under-cap arm now pins RLIMIT_AS then immediately syscall.Exec's a non-Go python3 ENOMEM probe (no post-setrlimit Go allocation), the full go test ./... suite is green, and the CI Go 1.26 job that previously failed with the runtime.persistentalloc1 panic now passes on the fix's push.

Overall: PASS ✓
