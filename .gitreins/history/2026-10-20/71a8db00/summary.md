# Verdict: OB-GAP-094

**Task:** Fix check-binary-fresh false-positive on Makefile-only edits
**Evaluated:** 2026-10-20T08:55:38.948032
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ check-binary-fresh exits 0 when only Makefile changed since stamped revision, while still exiting 1 when Go source differs: Makefile:66 now filters only `cmd/ internal/ web/ sql/ pkg/ go.mod go.sum` (Makefile removed by commit 7bd02be5; scripts/check-deploy:63 and scripts/check-deploy-test.sh:97 updated in a8e8014f). Verified live in /home/kara/off-by-one (artifact stamp v0.1.1-370-ga8e8014f, HEAD 55518b36): baseline `make check-binary-fresh` -> './off-by-one is up to date with source (version stamp changed, but code paths are unchanged)' EXIT=0; after appending a probe target to Makefile only (git status ' M Makefile') -> same message, EXIT=0; after appending a comment to internal/api/handlers.go -> 'ERROR: ./off-by-one is stale — source changed since it was built', EXIT=2 (make propagating the target's exit 1); restoring the pre-fix filter (Makefile back in the diff pathspec) -> EXIT=2 stale, proving the fix is causal. All probes reverted; tree left with only .gitreins/tasks.yaml modified.
check-binary-fresh now passes on Makefile-only edits (exit 0) and still fails on Go-source drift (exit 1/2), verified by live runs against the real artifact.

## Summary

Judge Result: OB-GAP-094

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ check-binary-fresh exits 0 when only Makefile changed since stamped revision, while still exiting 1 when Go source differs: Makefile:66 now filters only `cmd/ internal/ web/ sql/ pkg/ go.mod go.sum` (Makefile removed by commit 7bd02be5; scripts/check-deploy:63 and scripts/check-deploy-test.sh:97 updated in a8e8014f). Verified live in /home/kara/off-by-one (artifact stamp v0.1.1-370-ga8e8014f, HEAD 55518b36): baseline `make check-binary-fresh` -> './off-by-one is up to date with source (version stamp changed, but code paths are unchanged)' EXIT=0; after appending a probe target to Makefile only (git status ' M Makefile') -> same message, EXIT=0; after appending a comment to internal/api/handlers.go -> 'ERROR: ./off-by-one is stale — source changed since it was built', EXIT=2 (make propagating the target's exit 1); restoring the pre-fix filter (Makefile back in the diff pathspec) -> EXIT=2 stale, proving the fix is causal. All probes reverted; tree left with only .gitreins/tasks.yaml modified.
check-binary-fresh now passes on Makefile-only edits (exit 0) and still fails on Go-source drift (exit 1/2), verified by live runs against the real artifact.

Overall: PASS ✓
