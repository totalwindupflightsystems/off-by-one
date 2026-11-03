# Verdict: OB-GAP-096

**Task:** check-deploy-test not-a-repo leg must not inherit an enclosing git repo from TMPDIR
**Evaluated:** 2026-11-03T13:42:07.391743
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ scripts/check-deploy-test.sh: 'make check-deploy-test' exits 0 both with TMPDIR=/tmp and with TMPDIR set to a directory inside a git worktree (e.g. TMPDIR=/home/kara/.hermes/cache/scratch/oby-ob096 make check-deploy-test). The not-a-repo fixture leg either creates its fixture at a path where 'git -C <fixture> rev-parse --show-toplevel' fails, or asserts that precondition and SKIPs with a NOTE naming the enclosing repo found. The leg still fails loudly (adds a failure) if gate-deploy stops refusing foreign checkouts. Do not delete the leg and do not widen the assertion to accept the foreign-checkout message from a fixture that still inherits an enclosing repo.: Both runs exit 0: `TMPDIR=/tmp make check-deploy-test` -> EXIT=0, 'PASS: gate FAILs outside a git checkout (exit 1)' + 'check-deploy-test: all assertions passed'; `TMPDIR=/home/kara/.hermes/cache/scratch/oby-ob096 make check-deploy-test` (that dir resolves toplevel=/home/kara, git-dir=/home/kara/.git) -> EXIT=0, same PASS line, 'all assertions passed'. Implementation scripts/check-deploy-test.sh:618-659: the NOTAREPO leg builds its fixture under the first candidate dir (/tmp, /var/tmp, $TMPDIR) where `git -C <candidate> rev-parse --show-toplevel` FAILS (lines 619-627), so the fixture never inherits an enclosing repo; if no git-free dir exists it prints 'SKIP: NOTAREPO leg not graded ... enclosing repo inherited: $inherited' naming the repo (lines 631-633); otherwise it re-asserts the precondition on the fixture itself and hard-fails if a toplevel is inherited (lines 641-646). Grading is unchanged and strict: rc!=0 AND grep 'not a git checkout' (line 652) — not widened to the foreign-checkout message. Leg not deleted. Loud-failure check verified by mutation: replacing gate-deploy's `fail "not a git checkout ..."` with `exit 0` made the suite EXIT=2 with 'FAIL: gate should FAIL outside a git checkout; rc=0' and 'check-deploy-test: 1 failure(s)'; gate-deploy was then restored (git diff shows only the [MUTANT] marker change). [resolution 0.12; scripts/check-deploy-test.sh]
The not-a-repo leg now builds its fixture only in a proven git-free temp dir (with a NOTE-naming SKIP fallback and a fixture precondition assert), make check-deploy-test exits 0 under both TMPDIR=/tmp and a TMPDIR inside the /home/kara git repo, and a mutation test confirms the leg still fails loudly when gate-deploy stops refusing foreign checkouts.

## Summary

Judge Result: OB-GAP-096

Stage tier1: PASS
    ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ scripts/check-deploy-test.sh: 'make check-deploy-test' exits 0 both with TMPDIR=/tmp and with TMPDIR set to a directory inside a git worktree (e.g. TMPDIR=/home/kara/.hermes/cache/scratch/oby-ob096 make check-deploy-test). The not-a-repo fixture leg either creates its fixture at a path where 'git -C <fixture> rev-parse --show-toplevel' fails, or asserts that precondition and SKIPs with a NOTE naming the enclosing repo found. The leg still fails loudly (adds a failure) if gate-deploy stops refusing foreign checkouts. Do not delete the leg and do not widen the assertion to accept the foreign-checkout message from a fixture that still inherits an enclosing repo.: Both runs exit 0: `TMPDIR=/tmp make check-deploy-test` -> EXIT=0, 'PASS: gate FAILs outside a git checkout (exit 1)' + 'check-deploy-test: all assertions passed'; `TMPDIR=/home/kara/.hermes/cache/scratch/oby-ob096 make check-deploy-test` (that dir resolves toplevel=/home/kara, git-dir=/home/kara/.git) -> EXIT=0, same PASS line, 'all assertions passed'. Implementation scripts/check-deploy-test.sh:618-659: the NOTAREPO leg builds its fixture under the first candidate dir (/tmp, /var/tmp, $TMPDIR) where `git -C <candidate> rev-parse --show-toplevel` FAILS (lines 619-627), so the fixture never inherits an enclosing repo; if no git-free dir exists it prints 'SKIP: NOTAREPO leg not graded ... enclosing repo inherited: $inherited' naming the repo (lines 631-633); otherwise it re-asserts the precondition on the fixture itself and hard-fails if a toplevel is inherited (lines 641-646). Grading is unchanged and strict: rc!=0 AND grep 'not a git checkout' (line 652) — not widened to the foreign-checkout message. Leg not deleted. Loud-failure check verified by mutation: replacing gate-deploy's `fail "not a git checkout ..."` with `exit 0` made the suite EXIT=2 with 'FAIL: gate should FAIL outside a git checkout; rc=0' and 'check-deploy-test: 1 failure(s)'; gate-deploy was then restored (git diff shows only the [MUTANT] marker change). [resolution 0.12; scripts/check-deploy-test.sh]
The not-a-repo leg now builds its fixture only in a proven git-free temp dir (with a NOTE-naming SKIP fallback and a fixture precondition assert), make check-deploy-test exits 0 under both TMPDIR=/tmp and a TMPDIR inside the /home/kara git repo, and a mutation test confirms the leg still fails loudly when gate-deploy stops refusing foreign checkouts.

Overall: PASS ✓
