# Verdict: DF-OFF-BY-ONE-29

**Task:** Restore idle gate threshold -load-threshold 4
**Evaluated:** 2026-10-04T06:11:18.705615
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ deploy/off-by-one.service ships -load-threshold 4 (not -1); guard PASS: deploy/off-by-one.service:62 ExecStart contains `-load-threshold 4`; `grep -c 'load-threshold -1' deploy/off-by-one.service` returns 0. Commit fc8b9364 diff shows the exact change `-load-threshold -1` -> `-load-threshold 4` plus an inline comment ('Idle gate: solver only runs when loadavg(1) < 4 (16-core box threshold; DF-OFF-BY-ONE-29)'). Guard: `gitreins guard` exit_code=0, output 'Tier 1: DEGRADED PASS (skips: go_build=No Go files staged, go_tests=No Go files staged)' — skips are explicitly permitted by .gitreins/config.yaml (allow_skips: true) since this is a config/docs-only change with no Go files. scripts/check-deploy-test.sh also exits 0 with 'all assertions passed', including the UNIT leg verifying deploy/off-by-one.service exists with the documented ExecStart/WorkingDirectory.
deploy/off-by-one.service ships -load-threshold 4 (the -1 value is removed, proven by commit fc8b9364) and gitreins guard exits 0 with a DEGRADED PASS whose skips are allowed by config.

## Summary

Judge Result: DF-OFF-BY-ONE-29

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ deploy/off-by-one.service ships -load-threshold 4 (not -1); guard PASS: deploy/off-by-one.service:62 ExecStart contains `-load-threshold 4`; `grep -c 'load-threshold -1' deploy/off-by-one.service` returns 0. Commit fc8b9364 diff shows the exact change `-load-threshold -1` -> `-load-threshold 4` plus an inline comment ('Idle gate: solver only runs when loadavg(1) < 4 (16-core box threshold; DF-OFF-BY-ONE-29)'). Guard: `gitreins guard` exit_code=0, output 'Tier 1: DEGRADED PASS (skips: go_build=No Go files staged, go_tests=No Go files staged)' — skips are explicitly permitted by .gitreins/config.yaml (allow_skips: true) since this is a config/docs-only change with no Go files. scripts/check-deploy-test.sh also exits 0 with 'all assertions passed', including the UNIT leg verifying deploy/off-by-one.service exists with the documented ExecStart/WorkingDirectory.
deploy/off-by-one.service ships -load-threshold 4 (the -1 value is removed, proven by commit fc8b9364) and gitreins guard exits 0 with a DEGRADED PASS whose skips are allowed by config.

Overall: PASS ✓
