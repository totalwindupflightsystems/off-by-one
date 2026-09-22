# Verdict: OB-GAP-091

**Task:** CI Go leg red: bwrap uid map Permission denied on ubuntu-24.04 runners
**Evaluated:** 2026-09-22T15:43:57.774234
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	1.403s
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ Test (short) green on GitHub Actions after lifting unprivileged-userns restriction; all three sandbox tests (TestSandbox_Run_RealBwrap, TestSandbox_Run_GitAvailable, TestBSandboxRunner_RoundTrip) execute bwrap and pass in CI; OB-GAP-087 goal preserved (sandbox exercised, not skipped): Fix commit a45239c adds to .github/workflows/ci.yml build job (valid YAML, verified via yaml.safe_load) three steps ordered before 'Test (short)': 'Install bubblewrap' (line 49-50, OB-GAP-087 preserved), 'Lift AppArmor unprivileged-userns restriction' (line 59-62: sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0 || fail-open echo), 'Ensure setuid-root bubblewrap fallback' (line 66-71: chmod 4755 if not already), then 'Test (short)' (line 73-77: go test -short -count=1 ./...). DECISIVE CI EVIDENCE via gh: pre-fix run 35731609515 (commit 7a0c455) had 'Install bubblewrap' OK but 'Test (short)' FAILED (the SKIP->FAIL flip); post-fix run 35748864969 (commit a45239c) shows ALL 4 jobs success (Go 1.26 in 1m2s). In that green run the 'Lift AppArmor' step logged 'kernel.apparmor_restrict_unprivileged_userns = 0' (knob actually lifted) and the setuid fallback step ran; the Test (short) log shows 'ok internal/sandbox 1.041s' and 'ok internal/solver 0.153s' with zero SKIP/FAIL lines (non-trivial durations = real bwrap execution). Local reproduction: go test ./internal/sandbox/... ./internal/solver/... -short -count=1 -p 1 -run 'TestSandbox_Run_RealBwrap|TestSandbox_Run_GitAvailable|TestBSandboxRunner_RoundTrip' -v => all three '--- PASS' (0.03s/0.03s/0.04s), 0 SKIP; full go test ./... -short -count=1 -p 1 -timeout 300s => 14 pkgs 'ok', exit 0. OB-GAP-087 goal preserved: the three tests use lookupBwrap()/exec.LookPath (real bwrap, bwrap_test.go:192,370; bsandbox_runner_test.go:29), the only skip guards are 'bwrap not installed' (bwrap_test.go:190,365,368; bsandbox_runner_test.go:23,28) with no skip-on-error escape hatch, and CI installs bwrap so they execute rather than skip.
CI fix verified green on GitHub Actions (run 35748864969, all jobs success) with all three sandbox tests executing real bwrap and passing, while the pre-fix run failed at Test (short) — OB-GAP-087's goal of exercising the sandbox is preserved.

## Summary

Judge Result: OB-GAP-091

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	1.403s
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ Test (short) green on GitHub Actions after lifting unprivileged-userns restriction; all three sandbox tests (TestSandbox_Run_RealBwrap, TestSandbox_Run_GitAvailable, TestBSandboxRunner_RoundTrip) execute bwrap and pass in CI; OB-GAP-087 goal preserved (sandbox exercised, not skipped): Fix commit a45239c adds to .github/workflows/ci.yml build job (valid YAML, verified via yaml.safe_load) three steps ordered before 'Test (short)': 'Install bubblewrap' (line 49-50, OB-GAP-087 preserved), 'Lift AppArmor unprivileged-userns restriction' (line 59-62: sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0 || fail-open echo), 'Ensure setuid-root bubblewrap fallback' (line 66-71: chmod 4755 if not already), then 'Test (short)' (line 73-77: go test -short -count=1 ./...). DECISIVE CI EVIDENCE via gh: pre-fix run 35731609515 (commit 7a0c455) had 'Install bubblewrap' OK but 'Test (short)' FAILED (the SKIP->FAIL flip); post-fix run 35748864969 (commit a45239c) shows ALL 4 jobs success (Go 1.26 in 1m2s). In that green run the 'Lift AppArmor' step logged 'kernel.apparmor_restrict_unprivileged_userns = 0' (knob actually lifted) and the setuid fallback step ran; the Test (short) log shows 'ok internal/sandbox 1.041s' and 'ok internal/solver 0.153s' with zero SKIP/FAIL lines (non-trivial durations = real bwrap execution). Local reproduction: go test ./internal/sandbox/... ./internal/solver/... -short -count=1 -p 1 -run 'TestSandbox_Run_RealBwrap|TestSandbox_Run_GitAvailable|TestBSandboxRunner_RoundTrip' -v => all three '--- PASS' (0.03s/0.03s/0.04s), 0 SKIP; full go test ./... -short -count=1 -p 1 -timeout 300s => 14 pkgs 'ok', exit 0. OB-GAP-087 goal preserved: the three tests use lookupBwrap()/exec.LookPath (real bwrap, bwrap_test.go:192,370; bsandbox_runner_test.go:29), the only skip guards are 'bwrap not installed' (bwrap_test.go:190,365,368; bsandbox_runner_test.go:23,28) with no skip-on-error escape hatch, and CI installs bwrap so they execute rather than skip.
CI fix verified green on GitHub Actions (run 35748864969, all jobs success) with all three sandbox tests executing real bwrap and passing, while the pre-fix run failed at Test (short) — OB-GAP-087's goal of exercising the sandbox is preserved.

Overall: PASS ✓
