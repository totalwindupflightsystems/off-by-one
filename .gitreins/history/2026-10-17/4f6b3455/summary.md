# Verdict: DF-OFF-BY-ONE-49-fix

**Task:** Fix board validator to allow legacy event ID anomalies
**Evaluated:** 2026-10-17T22:44:58.269536
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ boardctl validate on .coding-hermes/board returns RESULT: FAIL due to 2 descending event IDs in legacy data; fix validator to pass (either renumber legacy ids upward or accept legacy anomalies): The descending-ID half was genuinely fixed, but the required end state (validate PASSES) is not met. PRE-FIX confirmed: `git show 8226aa63^:.coding-hermes/board/events.jsonl` scanned with python yields exactly 2 descending numeric id pairs — (line 387, 370->367) and (line 427, 1425->424) — matching the criterion's '2 descending event IDs'. Fix commit 8226aa63 'chore(board): fix descending event IDs (DF-OFF-BY-ONE-49)' remapped 367->371 and 424->1426 (166 insertions/166 deletions in .coding-hermes/board/events.jsonl), and a re-scan of the current file reports 'descending numeric id pairs: 0'. HOWEVER, running the actual command `boardctl validate .coding-hermes/board` still outputs `RESULT: FAIL (1 error(s), 181 warning(s))` / `boardctl: validation failed`. The remaining error is a different one: `[error] tasks.jsonl line 223 (task DF-OFF-BY-ONE-47): status "superseded" not in vocabulary {blocked,complete,failed,in_progress,pending,review}` — introduced by the earlier commit cd49eb99 'chore(board): supersede DF-OFF-BY-ONE-47 (resolved by DF-45)' and still present at 8226aa63 (grep -c '"superseded"' = 1). The criterion's explicit success condition is 'fix validator to pass', which is not satisfied: validate returns FAIL. Additionally, the task's own working-tree diff contains only .gitreins/tasks.yaml bookkeeping (no validator code change and no 'accept legacy anomalies' logic), so the board was left in a still-failing state.
The 2 descending event IDs were correctly renumbered (0 remain), but `boardctl validate .coding-hermes/board` still returns RESULT: FAIL due to an unresolved 'superseded' status error on tasks.jsonl line 223, so the criterion's requirement that the validator pass is not met.

## Summary

Judge Result: DF-OFF-BY-ONE-49-fix

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ boardctl validate on .coding-hermes/board returns RESULT: FAIL due to 2 descending event IDs in legacy data; fix validator to pass (either renumber legacy ids upward or accept legacy anomalies): The descending-ID half was genuinely fixed, but the required end state (validate PASSES) is not met. PRE-FIX confirmed: `git show 8226aa63^:.coding-hermes/board/events.jsonl` scanned with python yields exactly 2 descending numeric id pairs — (line 387, 370->367) and (line 427, 1425->424) — matching the criterion's '2 descending event IDs'. Fix commit 8226aa63 'chore(board): fix descending event IDs (DF-OFF-BY-ONE-49)' remapped 367->371 and 424->1426 (166 insertions/166 deletions in .coding-hermes/board/events.jsonl), and a re-scan of the current file reports 'descending numeric id pairs: 0'. HOWEVER, running the actual command `boardctl validate .coding-hermes/board` still outputs `RESULT: FAIL (1 error(s), 181 warning(s))` / `boardctl: validation failed`. The remaining error is a different one: `[error] tasks.jsonl line 223 (task DF-OFF-BY-ONE-47): status "superseded" not in vocabulary {blocked,complete,failed,in_progress,pending,review}` — introduced by the earlier commit cd49eb99 'chore(board): supersede DF-OFF-BY-ONE-47 (resolved by DF-45)' and still present at 8226aa63 (grep -c '"superseded"' = 1). The criterion's explicit success condition is 'fix validator to pass', which is not satisfied: validate returns FAIL. Additionally, the task's own working-tree diff contains only .gitreins/tasks.yaml bookkeeping (no validator code change and no 'accept legacy anomalies' logic), so the board was left in a still-failing state.
The 2 descending event IDs were correctly renumbered (0 remain), but `boardctl validate .coding-hermes/board` still returns RESULT: FAIL due to an unresolved 'superseded' status error on tasks.jsonl line 223, so the criterion's requirement that the validator pass is not met.

Overall: FAIL ✗
