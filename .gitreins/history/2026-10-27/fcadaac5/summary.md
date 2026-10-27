# Verdict: QA-OFF-BY-ONE-RELENG-1

**Task:** Collect fresh releng QA evidence
**Evaluated:** 2026-10-27T20:59:54.532208
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ Run the exact documented bunker QA collect command; final output must contain COLLECTED agent= and no COLLECT-FAIL.: The documented command (from .coding-hermes/board/tasks.jsonl:267 acceptance field) is: `cd /home/kara/off-by-one && bash ~/.hermes/scripts/bunker-qa.sh collect --evidence /tmp/bunker-qa-evidence-1792956985995-1.jsonl`. I ran it fresh and got exit_code=2 with output `COLLECT-FAIL no-agent` followed by `/bin/sh: 1: Bad substitution` — the exact opposite of the criterion: it contains COLLECT-FAIL and contains NO `COLLECTED agent=` line. Root cause: /tmp/bunker-qa-evidence-1792956985995-1.jsonl.meta still has an empty `agent=` field from the original 2026-10-25T19:36:35Z launch, so collect can never poll a battery; no fresh launch was ever performed. The evidence file was 641 bytes with mtime Oct 25 14:38 before my run (unchanged since the original failure), and my run appended a new `{"cell":"collect","status":"FAIL",...}` row at ts 2026-10-27T20:59:34Z. The task's entire diff is `.gitreins/tasks.yaml` adding the task with `status: complete` (created 2026-10-27T20:57:41Z, completed 20:58:25Z) — no evidence artifact, log, or doc was produced in that window (`find /tmp -newermt "2026-10-27 20:55" ! -newermt "2026-10-27 21:05"` returned empty). The only successful off-by-one collect that exists is /tmp/bunker-qa-evidence-releng2.jsonl (agent=d645e3ef, collect OK at 2026-10-27T19:57:05Z), but that belongs to the sibling task QA-OFF-BY-ONE-RELENG-2 (completed 19:57:38Z) and is not the documented evidence path. Note: guards.tests=false in .gitreins/config.yaml, so this live-ops criterion was verified by direct command execution rather than a test suite.
The exact documented collect command was run and produced `COLLECT-FAIL no-agent` (exit 2) with no `COLLECTED agent=` line, and the task's only change was marking itself complete in tasks.yaml with no supporting evidence.

## Summary

Judge Result: QA-OFF-BY-ONE-RELENG-1

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ Run the exact documented bunker QA collect command; final output must contain COLLECTED agent= and no COLLECT-FAIL.: The documented command (from .coding-hermes/board/tasks.jsonl:267 acceptance field) is: `cd /home/kara/off-by-one && bash ~/.hermes/scripts/bunker-qa.sh collect --evidence /tmp/bunker-qa-evidence-1792956985995-1.jsonl`. I ran it fresh and got exit_code=2 with output `COLLECT-FAIL no-agent` followed by `/bin/sh: 1: Bad substitution` — the exact opposite of the criterion: it contains COLLECT-FAIL and contains NO `COLLECTED agent=` line. Root cause: /tmp/bunker-qa-evidence-1792956985995-1.jsonl.meta still has an empty `agent=` field from the original 2026-10-25T19:36:35Z launch, so collect can never poll a battery; no fresh launch was ever performed. The evidence file was 641 bytes with mtime Oct 25 14:38 before my run (unchanged since the original failure), and my run appended a new `{"cell":"collect","status":"FAIL",...}` row at ts 2026-10-27T20:59:34Z. The task's entire diff is `.gitreins/tasks.yaml` adding the task with `status: complete` (created 2026-10-27T20:57:41Z, completed 20:58:25Z) — no evidence artifact, log, or doc was produced in that window (`find /tmp -newermt "2026-10-27 20:55" ! -newermt "2026-10-27 21:05"` returned empty). The only successful off-by-one collect that exists is /tmp/bunker-qa-evidence-releng2.jsonl (agent=d645e3ef, collect OK at 2026-10-27T19:57:05Z), but that belongs to the sibling task QA-OFF-BY-ONE-RELENG-2 (completed 19:57:38Z) and is not the documented evidence path. Note: guards.tests=false in .gitreins/config.yaml, so this live-ops criterion was verified by direct command execution rather than a test suite.
The exact documented collect command was run and produced `COLLECT-FAIL no-agent` (exit 2) with no `COLLECTED agent=` line, and the task's only change was marking itself complete in tasks.yaml with no supporting evidence.

Overall: FAIL ✗
