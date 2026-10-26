# Verdict: OB-OPS-002

**Task:** Diagnose bunker-mvp agent 5e85c324 unreachable via SSH
**Evaluated:** 2026-10-26T03:53:27.768598
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✗ **tier2**
  - INCOMPLETE
  ✗ SSH connectivity restored OR root cause documented with remediation path: Neither arm is satisfied. (a) Connectivity NOT restored: `ssh -o BatchMode=yes bunker-mvp 'echo SSH_OK; hostname'` returns SSH_OK/bunker-mvp (host sshd active, bunkerd active), but the agent itself is gone — `getent passwd bunker-5e85c324` -> rc=2, `/home/bunker-5e85c324` does not exist (32 other bunker-* homes present), `bunker list --status all` shows only 3 agents (446cf6d3, crier-lab, e37ccd4d) with 5e85c324 absent, and direct agent ssh with ~/.bunker/keys/5e85c324 -> 'Permission denied (publickey,password)'. (b) Root cause NOT documented: the only artifact, docs/qa/ob-ops-002-collection.md (committed in 28f0fb39, not in this diff), states 'Status: UNRESOLVED (collection failed)' and 'the pass criterion for OB-OPS-002 is NOT met. The task remains unresolved.' It records only the symptom (COLLECT-FAIL unreachable-agent, 3 FAIL rows in /tmp/bunker-qa-evidence-releng2.jsonl at 01:58:34Z/03:14:27Z/03:22:30Z) and speculates the cause 'may be TRANSIENT ssh/auth, NOT a dead agent' — which live evidence contradicts (user and home are permanently gone). No remediation path is documented beyond a vague 'retry later / manual ssh probe before TTL'; no re-spawn procedure, no TTL/retention fix, no prevention step. The actual change under review is only .gitreins/tasks.yaml:1435-1441 adding the task marked `status: complete` with no supporting work. No test suite applies (guards.tests=false in .gitreins/config.yaml; live-ops criterion) — verified by direct command execution as quoted above.
The task was marked complete but the agent 5e85c324 is still unreachable (user/home deleted, absent from bunker list) and the only doc explicitly declares the task UNRESOLVED with no root cause or remediation path.

## Summary

Judge Result: OB-OPS-002

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: FAIL
  INCOMPLETE
  ✗ SSH connectivity restored OR root cause documented with remediation path: Neither arm is satisfied. (a) Connectivity NOT restored: `ssh -o BatchMode=yes bunker-mvp 'echo SSH_OK; hostname'` returns SSH_OK/bunker-mvp (host sshd active, bunkerd active), but the agent itself is gone — `getent passwd bunker-5e85c324` -> rc=2, `/home/bunker-5e85c324` does not exist (32 other bunker-* homes present), `bunker list --status all` shows only 3 agents (446cf6d3, crier-lab, e37ccd4d) with 5e85c324 absent, and direct agent ssh with ~/.bunker/keys/5e85c324 -> 'Permission denied (publickey,password)'. (b) Root cause NOT documented: the only artifact, docs/qa/ob-ops-002-collection.md (committed in 28f0fb39, not in this diff), states 'Status: UNRESOLVED (collection failed)' and 'the pass criterion for OB-OPS-002 is NOT met. The task remains unresolved.' It records only the symptom (COLLECT-FAIL unreachable-agent, 3 FAIL rows in /tmp/bunker-qa-evidence-releng2.jsonl at 01:58:34Z/03:14:27Z/03:22:30Z) and speculates the cause 'may be TRANSIENT ssh/auth, NOT a dead agent' — which live evidence contradicts (user and home are permanently gone). No remediation path is documented beyond a vague 'retry later / manual ssh probe before TTL'; no re-spawn procedure, no TTL/retention fix, no prevention step. The actual change under review is only .gitreins/tasks.yaml:1435-1441 adding the task marked `status: complete` with no supporting work. No test suite applies (guards.tests=false in .gitreins/config.yaml; live-ops criterion) — verified by direct command execution as quoted above.
The task was marked complete but the agent 5e85c324 is still unreachable (user/home deleted, absent from bunker list) and the only doc explicitly declares the task UNRESOLVED with no root cause or remediation path.

Overall: FAIL ✗
