# Verdict: OB-GAP-094

**Task:** Fix check-binary-fresh false-positive on Makefile-only edits
**Evaluated:** 2026-10-20T08:52:28.911798
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✗ **tier2**
  - INCOMPLETE
  ✗ check-binary-fresh exits 0 when only Makefile changed since stamped revision, while still exiting 1 when Go source differs: The Makefile target itself is fixed and verified: Makefile:66 (commit 7bd02be5) dropped Makefile from the code-path filter (`git diff --name-only "$rev" -- cmd/ internal/ web/ sql/ pkg/ go.mod go.sum`). With only Makefile + .gitreins/tasks.yaml changed since the stamped rev 139b6fca, `make check-binary-fresh` -> exit 0 ("up to date with source (version stamp changed, but code paths are unchanged)"); after appending a line to internal/api/handlers.go it -> exit 2 (make's code for recipe exit 1) with "ERROR: ./off-by-one is stale — source changed since it was built" (reverted). HOWEVER the same freshness gate is still false-positive on Makefile-only edits through its sibling leg: scripts/check-deploy:62 still filters `... go.mod go.sum Makefile`, and `./scripts/check-deploy --resolve-stamp v0.1.1-368-g139b6fca` (the exact stamp of the on-disk artifact, i.e. the Makefile-only scenario) exits 1 with "FAIL: stamp 'v0.1.1-368-g139b6fca' resolves to 139b6fca, but HEAD's code paths changed since it: Makefile". check-deploy leg 1 chains make check-binary-fresh (now passes) but leg 4 uses resolve_stamp, so `make check-deploy`/`make gate-deploy` still fail on a Makefile-only edit — the reported false positive is only half fixed. scripts/check-deploy-test.sh:97 likewise still lists Makefile in its dirty-code-path set, and no test was added asserting the Makefile-only case (check-deploy-test.sh only covers fresh/divergence/garbage stamps; `bash scripts/check-deploy-test.sh` exit 0, all assertions passed, but none exercise Makefile-only).
Makefile's check-binary-fresh now passes on Makefile-only edits and still fails on Go-source drift, but the identical code-path filter in scripts/check-deploy:62 (leg 4 / --resolve-stamp) still flags Makefile, so the deploy gate keeps the false positive and the fix is incomplete.

## Summary

Judge Result: OB-GAP-094

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: FAIL
  INCOMPLETE
  ✗ check-binary-fresh exits 0 when only Makefile changed since stamped revision, while still exiting 1 when Go source differs: The Makefile target itself is fixed and verified: Makefile:66 (commit 7bd02be5) dropped Makefile from the code-path filter (`git diff --name-only "$rev" -- cmd/ internal/ web/ sql/ pkg/ go.mod go.sum`). With only Makefile + .gitreins/tasks.yaml changed since the stamped rev 139b6fca, `make check-binary-fresh` -> exit 0 ("up to date with source (version stamp changed, but code paths are unchanged)"); after appending a line to internal/api/handlers.go it -> exit 2 (make's code for recipe exit 1) with "ERROR: ./off-by-one is stale — source changed since it was built" (reverted). HOWEVER the same freshness gate is still false-positive on Makefile-only edits through its sibling leg: scripts/check-deploy:62 still filters `... go.mod go.sum Makefile`, and `./scripts/check-deploy --resolve-stamp v0.1.1-368-g139b6fca` (the exact stamp of the on-disk artifact, i.e. the Makefile-only scenario) exits 1 with "FAIL: stamp 'v0.1.1-368-g139b6fca' resolves to 139b6fca, but HEAD's code paths changed since it: Makefile". check-deploy leg 1 chains make check-binary-fresh (now passes) but leg 4 uses resolve_stamp, so `make check-deploy`/`make gate-deploy` still fail on a Makefile-only edit — the reported false positive is only half fixed. scripts/check-deploy-test.sh:97 likewise still lists Makefile in its dirty-code-path set, and no test was added asserting the Makefile-only case (check-deploy-test.sh only covers fresh/divergence/garbage stamps; `bash scripts/check-deploy-test.sh` exit 0, all assertions passed, but none exercise Makefile-only).
Makefile's check-binary-fresh now passes on Makefile-only edits and still fails on Go-source drift, but the identical code-path filter in scripts/check-deploy:62 (leg 4 / --resolve-stamp) still flags Makefile, so the deploy gate keeps the false positive and the fix is incomplete.

Overall: FAIL ✗
