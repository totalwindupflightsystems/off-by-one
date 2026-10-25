# Verdict: DF-OFF-BY-ONE-55

**Task:** Enforce repo-local board lint
**Evaluated:** 2026-10-25T23:00:50.069986
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ Duplicate task ids in a throwaway clone are rejected by the repo-local pre-commit hook with unchanged HEAD: Live reproduction in a fresh clone (/tmp/obytest55 of /home/kara/off-by-one): `bash scripts/install-hooks.sh` exited 0, installed .git/hooks/pre-commit and set repo-local core.hooksPath (overriding the global /var/tmp/foreign-hooks); `git rev-parse --git-path hooks/pre-commit` -> /tmp/obytest55/.git/hooks/pre-commit. Appending a duplicate board row (id JSONL-NORM-001, already line 1) to .coding-hermes/board/tasks.jsonl then `git add` + `git commit` gave commit_rc=1, HEAD unchanged (75420a1f before and after), with hook output 'board-lint: commit rejected (boardctl validate failed, mode=fail)' and '[error] tasks.jsonl line 270: duplicate task id "JSONL-NORM-001" (also line 1)'. Repeated after a valid commit: commit_rc=1, HEAD dbc7545b unchanged. Control arm: a valid new board row committed rc=0 and HEAD advanced (gate is not a blanket wedge). Code: scripts/hooks/pre-commit:31 boardctl_lint_mode="fail", lines 73-79 return 1 on validation error, epilogue exits with that status; scripts/install-hooks.sh installs the tracked hook and writes the repo-local core.hooksPath override. Regression self-test run fresh: `bash scripts/tests/install-hooks-selftest.sh` -> exit_code 0, 'SELFTEST PASS: install-hooks + fail-closed board-lint'.
Verified by live reproduction in a throwaway clone: the repo-local pre-commit hook rejects a duplicate board id with exit 1 and HEAD unmoved, while a valid row still commits.

## Summary

Judge Result: DF-OFF-BY-ONE-55

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ Duplicate task ids in a throwaway clone are rejected by the repo-local pre-commit hook with unchanged HEAD: Live reproduction in a fresh clone (/tmp/obytest55 of /home/kara/off-by-one): `bash scripts/install-hooks.sh` exited 0, installed .git/hooks/pre-commit and set repo-local core.hooksPath (overriding the global /var/tmp/foreign-hooks); `git rev-parse --git-path hooks/pre-commit` -> /tmp/obytest55/.git/hooks/pre-commit. Appending a duplicate board row (id JSONL-NORM-001, already line 1) to .coding-hermes/board/tasks.jsonl then `git add` + `git commit` gave commit_rc=1, HEAD unchanged (75420a1f before and after), with hook output 'board-lint: commit rejected (boardctl validate failed, mode=fail)' and '[error] tasks.jsonl line 270: duplicate task id "JSONL-NORM-001" (also line 1)'. Repeated after a valid commit: commit_rc=1, HEAD dbc7545b unchanged. Control arm: a valid new board row committed rc=0 and HEAD advanced (gate is not a blanket wedge). Code: scripts/hooks/pre-commit:31 boardctl_lint_mode="fail", lines 73-79 return 1 on validation error, epilogue exits with that status; scripts/install-hooks.sh installs the tracked hook and writes the repo-local core.hooksPath override. Regression self-test run fresh: `bash scripts/tests/install-hooks-selftest.sh` -> exit_code 0, 'SELFTEST PASS: install-hooks + fail-closed board-lint'.
Verified by live reproduction in a throwaway clone: the repo-local pre-commit hook rejects a duplicate board id with exit 1 and HEAD unmoved, while a valid row still commits.

Overall: PASS ✓
