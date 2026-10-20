# Verdict: OB-DIST-001

**Task:** ob1-distribute false-green: corpus commit failure swallowed by || true, script prints success with the WRONG commit and a no-op push
**Evaluated:** 2026-10-20T02:57:56.647994
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ scripts/ob1-distribute.sh (tracked canonical + the deployed copy at ~/.hermes/scripts/ob1-distribute.sh) MUST fail loudly (non-zero exit, error on stderr/stdout) when git commit does not advance HEAD, and MUST verify the pushed remote actually contains the new commit before printing success. Regression proof required: old script exits 0 printing 'Answer corpus synced' while HEAD is unchanged; new script exits 1 naming the failure.: Canonical scripts/ob1-distribute.sh:49-80 captures PRE_HEAD, checks git commit rc (exit 1, '❌ corpus commit FAILED (rc=$COMMIT_RC)'), then compares NEW_HEAD to PRE_HEAD (exit 1, '❌ corpus commit did not advance HEAD (...) — refusing to report success'), checks push rc, and verifies the remote via `git ls-remote origin refs/heads/master` compared to NEW_HEAD with a `git fetch` + `git merge-base --is-ancestor` fallback (exit 1, '❌ push did not land: origin/master=... does not contain ...') BEFORE the '📦 Answer corpus synced to GitHub' line. Deployed copy is byte-identical: `diff` reports IDENTICAL, md5sum e200c50b0414354474eb556cf7519166 for both scripts/ob1-distribute.sh and /home/kara/.hermes/scripts/ob1-distribute.sh. Regression proof executed: `bash scripts/tests/ob1-distribute-regression.sh` -> EXIT_CODE=0, '[1] ✔ old block exits 0 claiming success over the stale commit (RED shape reproduced)', '[2] ✔ non-zero exit, failure named, no success claim, HEAD untouched', '[3] ✔ head advanced and the remote carries 80d3347', '[4] ✔ no-op push caught: push did not land: origin/master=a1af249...', 'PASS — all OB-DIST-001 cases hold'. Same test against the deployed copy: EXIT_CODE=0, cases [2][3][4] PASS (case [1] skipped only because the pre-fix revision lookup is relative to the script's parent repo, not a script defect). [resolution 0.00; scripts/ob1-distribute.sh, /.hermes/scripts/ob1-distribute.sh]
Both the tracked and deployed ob1-distribute.sh now fail loudly on a non-advancing commit and verify the remote carries the new commit before printing success, with a passing 4-case regression test that reproduces the old silent false-green.

## Summary

Judge Result: OB-DIST-001

Stage tier1: FAIL
    ✗ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ scripts/ob1-distribute.sh (tracked canonical + the deployed copy at ~/.hermes/scripts/ob1-distribute.sh) MUST fail loudly (non-zero exit, error on stderr/stdout) when git commit does not advance HEAD, and MUST verify the pushed remote actually contains the new commit before printing success. Regression proof required: old script exits 0 printing 'Answer corpus synced' while HEAD is unchanged; new script exits 1 naming the failure.: Canonical scripts/ob1-distribute.sh:49-80 captures PRE_HEAD, checks git commit rc (exit 1, '❌ corpus commit FAILED (rc=$COMMIT_RC)'), then compares NEW_HEAD to PRE_HEAD (exit 1, '❌ corpus commit did not advance HEAD (...) — refusing to report success'), checks push rc, and verifies the remote via `git ls-remote origin refs/heads/master` compared to NEW_HEAD with a `git fetch` + `git merge-base --is-ancestor` fallback (exit 1, '❌ push did not land: origin/master=... does not contain ...') BEFORE the '📦 Answer corpus synced to GitHub' line. Deployed copy is byte-identical: `diff` reports IDENTICAL, md5sum e200c50b0414354474eb556cf7519166 for both scripts/ob1-distribute.sh and /home/kara/.hermes/scripts/ob1-distribute.sh. Regression proof executed: `bash scripts/tests/ob1-distribute-regression.sh` -> EXIT_CODE=0, '[1] ✔ old block exits 0 claiming success over the stale commit (RED shape reproduced)', '[2] ✔ non-zero exit, failure named, no success claim, HEAD untouched', '[3] ✔ head advanced and the remote carries 80d3347', '[4] ✔ no-op push caught: push did not land: origin/master=a1af249...', 'PASS — all OB-DIST-001 cases hold'. Same test against the deployed copy: EXIT_CODE=0, cases [2][3][4] PASS (case [1] skipped only because the pre-fix revision lookup is relative to the script's parent repo, not a script defect). [resolution 0.00; scripts/ob1-distribute.sh, /.hermes/scripts/ob1-distribute.sh]
Both the tracked and deployed ob1-distribute.sh now fail loudly on a non-advancing commit and verify the remote carries the new commit before printing success, with a passing 4-case regression test that reproduces the old silent false-green.

Overall: FAIL ✗
