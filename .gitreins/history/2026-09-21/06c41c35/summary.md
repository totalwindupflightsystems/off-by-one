# Verdict: OB-GAP-CI-BUNDLE

**Task:** CI honesty bundle: bwrap in CI, honest go matrix, grouped sqlite3 guard
**Evaluated:** 2026-09-21T22:46:36.696712
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ ci.yml installs bubblewrap before Test (short); go matrix is a single 1.26 leg matching go.mod; sqlite3 ensure-step uses explicit grouping: .github/workflows/ci.yml build job: matrix go-version: ['1.26'] (single leg) matches go.mod line 3 'go 1.26.0'. Step 'Install bubblewrap (exercise sandbox tests)' run: `command -v bwrap >/dev/null 2>&1 || { sudo apt-get update -qq && sudo apt-get install -y -qq bubblewrap; }` is ordered BEFORE 'Test (short)' (yaml.safe_load step order confirms). sqlite3 ensure-step run: `command -v sqlite3 >/dev/null || { sudo apt-get update -qq && sudo apt-get install -y -qq sqlite3; }` — explicit grouping. Commit 4e37192 diff confirms all three edits.
  ✓ bwrap-guarded tests run (PASS not SKIP) with bwrap present; no matrix leg can silently run a mismatched toolchain; guard compound provably skips install when binary exists: With bwrap present (/usr/bin/bwrap, bubblewrap 0.11.1): `go test -count=1 -v -run 'RealBwrap|RoundTrip|Sandbox_Run_Timeout|CommandFailure|BwrapAvailable' ./internal/sandbox/ ./internal/solver/` -> PASS TestSandbox_Run_RealBwrap, TestSandbox_Run_Timeout, TestSandbox_Run_CommandFailure, TestBwrapAvailable (BwrapAvailable=true), TestBSandboxRunner_RoundTrip; no SKIPs. With bwrap hidden from PATH the same 4 tests SKIP, proving the guard (BwrapAvailable via exec.LookPath) is real and the CI install is what makes them run. Matrix is a single ['1.26'] leg matching go.mod 1.26.0; the other jobs (deploy-gate-selftest, binary-seed-probe) pin go-version: '1.26' directly with no matrix, so no leg can run a mismatched toolchain. Guard probe: grouped form with binary present -> 0 apt-get calls, binary absent -> 2 calls; old ungrouped form with binary present -> install ran (the bug). `go test -short -count=1 ./...` -> all packages ok.
CI honesty bundle fully implemented and verified: bwrap installed before Test (short), single 1.26 matrix leg matching go.mod, grouped sqlite3/bwrap guards provably skip install when the binary exists, and bwrap-guarded tests PASS (not SKIP) with bwrap present.

## Summary

Judge Result: OB-GAP-CI-BUNDLE

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ ci.yml installs bubblewrap before Test (short); go matrix is a single 1.26 leg matching go.mod; sqlite3 ensure-step uses explicit grouping: .github/workflows/ci.yml build job: matrix go-version: ['1.26'] (single leg) matches go.mod line 3 'go 1.26.0'. Step 'Install bubblewrap (exercise sandbox tests)' run: `command -v bwrap >/dev/null 2>&1 || { sudo apt-get update -qq && sudo apt-get install -y -qq bubblewrap; }` is ordered BEFORE 'Test (short)' (yaml.safe_load step order confirms). sqlite3 ensure-step run: `command -v sqlite3 >/dev/null || { sudo apt-get update -qq && sudo apt-get install -y -qq sqlite3; }` — explicit grouping. Commit 4e37192 diff confirms all three edits.
  ✓ bwrap-guarded tests run (PASS not SKIP) with bwrap present; no matrix leg can silently run a mismatched toolchain; guard compound provably skips install when binary exists: With bwrap present (/usr/bin/bwrap, bubblewrap 0.11.1): `go test -count=1 -v -run 'RealBwrap|RoundTrip|Sandbox_Run_Timeout|CommandFailure|BwrapAvailable' ./internal/sandbox/ ./internal/solver/` -> PASS TestSandbox_Run_RealBwrap, TestSandbox_Run_Timeout, TestSandbox_Run_CommandFailure, TestBwrapAvailable (BwrapAvailable=true), TestBSandboxRunner_RoundTrip; no SKIPs. With bwrap hidden from PATH the same 4 tests SKIP, proving the guard (BwrapAvailable via exec.LookPath) is real and the CI install is what makes them run. Matrix is a single ['1.26'] leg matching go.mod 1.26.0; the other jobs (deploy-gate-selftest, binary-seed-probe) pin go-version: '1.26' directly with no matrix, so no leg can run a mismatched toolchain. Guard probe: grouped form with binary present -> 0 apt-get calls, binary absent -> 2 calls; old ungrouped form with binary present -> install ran (the bug). `go test -short -count=1 ./...` -> all packages ok.
CI honesty bundle fully implemented and verified: bwrap installed before Test (short), single 1.26 matrix leg matching go.mod, grouped sqlite3/bwrap guards provably skip install when the binary exists, and bwrap-guarded tests PASS (not SKIP) with bwrap present.

Overall: PASS ✓
