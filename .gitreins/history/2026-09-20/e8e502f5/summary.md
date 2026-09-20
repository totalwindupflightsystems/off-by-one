# Verdict: OB-GAP-074

**Task:** board hygiene: duplicate status + prose in categorical fields
**Evaluated:** 2026-09-20T13:52:16.810301
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ boardctl -C . validate reports errors only for the duplicate-slot ids (writer-side, QA lane) and zero vocabulary/prose diagnostics; rows carrying status 'duplicate' read a vocabulary status with superseded_by preserved; guard_result/ci_result carry PASS/FAIL/SKIP and GREEN/RED/SKIP with the run detail moved to foreman_note: `boardctl -C . validate` (exit 1) emits 13 errors, ALL 13 'duplicate task id' (grep -vc 'duplicate task id' = 0); duplicate ids are exclusively QA-OFF-BY-ONE-1/2/3 and DF-OFF-BY-ONE-1..5,10,11 (writer-side, QA lane). Zero vocabulary/prose diagnostics: grep of validate output for vocab|prose|status|guard|ci_result|superseded = no matches and grep -cE 'not in vocabulary|not in write vocabulary|read alias' = 0, while the boardctl binary does implement those checks ('status %q not in vocabulary', 'guard %q not in vocabulary {PASS,FAIL,SKIP}', 'ci %q not in vocabulary {GREEN,RED,SKIP}'). The 3 rows that carried status 'duplicate' pre-change (git show 415cc18^: line68 QA-OFF-BY-ONE-1 superseded_by QA-OFF-BY-ONE-16, line77 QA-OFF-BY-ONE-3 superseded_by QA-OFF-BY-ONE-4, line98 QA-OFnBY-ONE-8 superseded_by QA-OFF-BY-ONE-1) now read status='complete' with superseded_by preserved; current statuses = {complete:121, pending:15}, no 'duplicate'. guard_result Counter = {PASS:80, FAIL:2, None:54} and ci_result Counter = {GREEN:29, SKIP:4, None:103}, with 0 out-of-vocabulary values under strict {PASS,FAIL,SKIP}/{GREEN,RED,SKIP} checks. Run detail moved to foreman_note: 39 rows carry 'board hygiene (OB-GAP-074): guard: <original prose>; ci: <original prose>' (e.g. OB-GAP-045 'guard: PASS 4/4 (secrets/build/lint/tests); ci: green (pending CI run on edcd27f)'; OB-GAP-047 'ci: not run (unpushed — push blocked by OB-OPS-001 dead GH token)' -> ci_result SKIP), preserving the pre-state prose ('pass 4/4', 'PASS (board-only)', 'green (pending CI run on edcd27f)'). Landed in commit 415cc18.
boardctl validate reports only the 13 QA/DF duplicate-slot id errors with zero vocabulary/prose diagnostics, duplicate-status rows now read 'complete' with superseded_by preserved, and guard_result/ci_result are canonical PASS/FAIL/SKIP and GREEN/RED/SKIP with the original run prose preserved in foreman_note.

## Summary

Judge Result: OB-GAP-074

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ boardctl -C . validate reports errors only for the duplicate-slot ids (writer-side, QA lane) and zero vocabulary/prose diagnostics; rows carrying status 'duplicate' read a vocabulary status with superseded_by preserved; guard_result/ci_result carry PASS/FAIL/SKIP and GREEN/RED/SKIP with the run detail moved to foreman_note: `boardctl -C . validate` (exit 1) emits 13 errors, ALL 13 'duplicate task id' (grep -vc 'duplicate task id' = 0); duplicate ids are exclusively QA-OFF-BY-ONE-1/2/3 and DF-OFF-BY-ONE-1..5,10,11 (writer-side, QA lane). Zero vocabulary/prose diagnostics: grep of validate output for vocab|prose|status|guard|ci_result|superseded = no matches and grep -cE 'not in vocabulary|not in write vocabulary|read alias' = 0, while the boardctl binary does implement those checks ('status %q not in vocabulary', 'guard %q not in vocabulary {PASS,FAIL,SKIP}', 'ci %q not in vocabulary {GREEN,RED,SKIP}'). The 3 rows that carried status 'duplicate' pre-change (git show 415cc18^: line68 QA-OFF-BY-ONE-1 superseded_by QA-OFF-BY-ONE-16, line77 QA-OFF-BY-ONE-3 superseded_by QA-OFF-BY-ONE-4, line98 QA-OFnBY-ONE-8 superseded_by QA-OFF-BY-ONE-1) now read status='complete' with superseded_by preserved; current statuses = {complete:121, pending:15}, no 'duplicate'. guard_result Counter = {PASS:80, FAIL:2, None:54} and ci_result Counter = {GREEN:29, SKIP:4, None:103}, with 0 out-of-vocabulary values under strict {PASS,FAIL,SKIP}/{GREEN,RED,SKIP} checks. Run detail moved to foreman_note: 39 rows carry 'board hygiene (OB-GAP-074): guard: <original prose>; ci: <original prose>' (e.g. OB-GAP-045 'guard: PASS 4/4 (secrets/build/lint/tests); ci: green (pending CI run on edcd27f)'; OB-GAP-047 'ci: not run (unpushed — push blocked by OB-OPS-001 dead GH token)' -> ci_result SKIP), preserving the pre-state prose ('pass 4/4', 'PASS (board-only)', 'green (pending CI run on edcd27f)'). Landed in commit 415cc18.
boardctl validate reports only the 13 QA/DF duplicate-slot id errors with zero vocabulary/prose diagnostics, duplicate-status rows now read 'complete' with superseded_by preserved, and guard_result/ci_result are canonical PASS/FAIL/SKIP and GREEN/RED/SKIP with the original run prose preserved in foreman_note.

Overall: PASS ✓
