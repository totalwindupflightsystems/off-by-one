# Verdict: OB-DIST-002

**Task:** Verify corpus distribution commit no-op fix
**Evaluated:** 2026-10-25T23:22:17.069480
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ The tracked scripts/ob1-distribute.sh must fail non-zero with a named error when git commit is rejected or HEAD does not advance, must not print the success claim on rejection, and must verify origin/master carries the new commit; scripts/tests/ob1-distribute-regression.sh must reproduce the old false-green and pass the fixed cases.: Both files are tracked (git ls-files returns scripts/ob1-distribute.sh and scripts/tests/ob1-distribute-regression.sh). (a) Commit rejection: ob1-distribute.sh:72 captures COMMIT_RC=$?, :74-77 `if [ "$COMMIT_RC" -ne 0 ]` prints "❌ corpus commit FAILED (rc=$COMMIT_RC) — nothing was distributed" and exits 1; the old `git commit ... || true` is gone (only referenced in the comment at :59). (b) HEAD not advancing: :80-82 `if [ "$NEW_HEAD" = "$PRE_HEAD" ]` prints "❌ corpus commit did not advance HEAD ... refusing to report success" and exits 1. (c) origin/master verification: :94 `git ls-remote origin refs/heads/master`, :96 `git merge-base --is-ancestor "$NEW_HEAD" origin/master`, :97-98 "❌ push did not land: origin/master=... does not contain ..." + exit 1. (d) The success claim "📦 Answer corpus synced to GitHub:" at :101 sits AFTER all three guards (exit 1 at :77, :82, :98). Independently reproduced by executing the sliced PART 1 block in scratch repos: failing pre-commit hook -> RC=1, "corpus commit FAILED (rc=1)", success-claim count 0; git wrapper faking commit success -> RC=1, "corpus commit did not advance HEAD (e976b78...)", success-claim count 0; push remote redirected to a different bare repo -> RC=1, "push did not land: origin/master=934c269... does not contain 3e4baab...", success-claim count 0. (e) Regression suite run fresh: `bash scripts/tests/ob1-distribute-regression.sh` -> REAL_EXIT_CODE=0 with "[1] ✔ old block exits 0 claiming success over the stale commit (RED shape reproduced)" (old false-green reproduced from git history), "[2] ✔ non-zero exit, failure named, no success claim, HEAD untouched", "[3] ✔ head advanced and the remote carries cd3810b", "[4] ✔ no-op push caught: push did not land: origin/master=63d6259...", and "PASS — all OB-DIST-001 cases hold (real checkout at 285b861 untouched)". Real checkout HEAD remained 285b861 with no leakage. The tier-1.5 pre-screen (0.00/UNRESOLVED) is contradicted by this direct execution evidence. [resolution 0.00; scripts/ob1-distribute.sh, scripts/tests/ob1-distribute-regression.sh]
scripts/ob1-distribute.sh fails non-zero with named errors on commit rejection, non-advancing HEAD, and unlanded push (success claim only after all guards), and scripts/tests/ob1-distribute-regression.sh exits 0 reproducing the old false-green plus all three fixed cases.

## Summary

Judge Result: OB-DIST-002

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ The tracked scripts/ob1-distribute.sh must fail non-zero with a named error when git commit is rejected or HEAD does not advance, must not print the success claim on rejection, and must verify origin/master carries the new commit; scripts/tests/ob1-distribute-regression.sh must reproduce the old false-green and pass the fixed cases.: Both files are tracked (git ls-files returns scripts/ob1-distribute.sh and scripts/tests/ob1-distribute-regression.sh). (a) Commit rejection: ob1-distribute.sh:72 captures COMMIT_RC=$?, :74-77 `if [ "$COMMIT_RC" -ne 0 ]` prints "❌ corpus commit FAILED (rc=$COMMIT_RC) — nothing was distributed" and exits 1; the old `git commit ... || true` is gone (only referenced in the comment at :59). (b) HEAD not advancing: :80-82 `if [ "$NEW_HEAD" = "$PRE_HEAD" ]` prints "❌ corpus commit did not advance HEAD ... refusing to report success" and exits 1. (c) origin/master verification: :94 `git ls-remote origin refs/heads/master`, :96 `git merge-base --is-ancestor "$NEW_HEAD" origin/master`, :97-98 "❌ push did not land: origin/master=... does not contain ..." + exit 1. (d) The success claim "📦 Answer corpus synced to GitHub:" at :101 sits AFTER all three guards (exit 1 at :77, :82, :98). Independently reproduced by executing the sliced PART 1 block in scratch repos: failing pre-commit hook -> RC=1, "corpus commit FAILED (rc=1)", success-claim count 0; git wrapper faking commit success -> RC=1, "corpus commit did not advance HEAD (e976b78...)", success-claim count 0; push remote redirected to a different bare repo -> RC=1, "push did not land: origin/master=934c269... does not contain 3e4baab...", success-claim count 0. (e) Regression suite run fresh: `bash scripts/tests/ob1-distribute-regression.sh` -> REAL_EXIT_CODE=0 with "[1] ✔ old block exits 0 claiming success over the stale commit (RED shape reproduced)" (old false-green reproduced from git history), "[2] ✔ non-zero exit, failure named, no success claim, HEAD untouched", "[3] ✔ head advanced and the remote carries cd3810b", "[4] ✔ no-op push caught: push did not land: origin/master=63d6259...", and "PASS — all OB-DIST-001 cases hold (real checkout at 285b861 untouched)". Real checkout HEAD remained 285b861 with no leakage. The tier-1.5 pre-screen (0.00/UNRESOLVED) is contradicted by this direct execution evidence. [resolution 0.00; scripts/ob1-distribute.sh, scripts/tests/ob1-distribute-regression.sh]
scripts/ob1-distribute.sh fails non-zero with named errors on commit rejection, non-advancing HEAD, and unlanded push (success claim only after all guards), and scripts/tests/ob1-distribute-regression.sh exits 0 reproducing the old false-green plus all three fixed cases.

Overall: PASS ✓
