# Verdict: OB-OPS-002

**Task:** Verify bunker-mvp QA collection reachability
**Evaluated:** 2026-10-26T03:26:05.622792
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ ssh bunker-mvp (or tailscale ping + ssh) succeeds; bunker-qa.sh collect --evidence /tmp/bunker-qa-evidence-releng2.jsonl prints COLLECTED agent=5e85c324 rows=N with N>=1 and no COLLECT-FAIL line: Only the ssh half holds: `ssh -o BatchMode=yes bunker-mvp 'echo SSH_OK; hostname'` returned SSH_OK / bunker-mvp (exit 0). The collect half is contradicted by the evidence file itself: /tmp/bunker-qa-evidence-releng2.jsonl (mtime Oct 25 22:22) contains THREE collect records, all failures — {"cell":"collect","status":"FAIL","detail":"cannot reach agent=5e85c324 on bunker-mvp after 3 probe attempts (last: ) ..."} at ts 2026-10-26T01:58:34Z, 03:14:27Z and 03:22:30Z. There is no 'COLLECTED agent=5e85c324 rows=N' line anywhere in the file, and the task was marked complete at 2026-10-26T03:25:29Z, ~3 minutes after the last COLLECT-FAIL. Live re-check confirms the agent is gone: on bunker-mvp `ls -d /home/bunker-5e85c324` -> 'No such file or directory' (33 bunker-* homes exist, none is 5e85c324) and /tmp/bunker-qa-evidence-releng2.jsonl does not exist on the remote host. No test suite applies (guards.tests=false in .gitreins/config.yaml; this is a live-ops criterion), so verification was by direct command execution as above.
ssh to bunker-mvp works, but the required collect never succeeded — the evidence file records three COLLECT-FAIL lines for agent=5e85c324 and no COLLECTED line, and the agent home no longer exists on the host.

## Summary

Judge Result: OB-OPS-002

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ ssh bunker-mvp (or tailscale ping + ssh) succeeds; bunker-qa.sh collect --evidence /tmp/bunker-qa-evidence-releng2.jsonl prints COLLECTED agent=5e85c324 rows=N with N>=1 and no COLLECT-FAIL line: Only the ssh half holds: `ssh -o BatchMode=yes bunker-mvp 'echo SSH_OK; hostname'` returned SSH_OK / bunker-mvp (exit 0). The collect half is contradicted by the evidence file itself: /tmp/bunker-qa-evidence-releng2.jsonl (mtime Oct 25 22:22) contains THREE collect records, all failures — {"cell":"collect","status":"FAIL","detail":"cannot reach agent=5e85c324 on bunker-mvp after 3 probe attempts (last: ) ..."} at ts 2026-10-26T01:58:34Z, 03:14:27Z and 03:22:30Z. There is no 'COLLECTED agent=5e85c324 rows=N' line anywhere in the file, and the task was marked complete at 2026-10-26T03:25:29Z, ~3 minutes after the last COLLECT-FAIL. Live re-check confirms the agent is gone: on bunker-mvp `ls -d /home/bunker-5e85c324` -> 'No such file or directory' (33 bunker-* homes exist, none is 5e85c324) and /tmp/bunker-qa-evidence-releng2.jsonl does not exist on the remote host. No test suite applies (guards.tests=false in .gitreins/config.yaml; this is a live-ops criterion), so verification was by direct command execution as above.
ssh to bunker-mvp works, but the required collect never succeeded — the evidence file records three COLLECT-FAIL lines for agent=5e85c324 and no COLLECTED line, and the agent home no longer exists on the host.

Overall: FAIL ✗
