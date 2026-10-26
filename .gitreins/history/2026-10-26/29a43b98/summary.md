# Verdict: OB-OPS-002

**Task:** Diagnose bunker-mvp agent 5e85c324 unreachable via SSH
**Evaluated:** 2026-10-26T03:54:54.490082
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ SSH connectivity restored OR root cause documented with remediation path: Connectivity was not restored (agent is gone, not merely unreachable), but the second arm is satisfied. docs/qa/ob-ops-002-collection.md (committed in 60fa16e6) states 'Status: ROOT-CAUSE IDENTIFIED (owner intervention required)' and documents the root cause with concrete evidence: `getent passwd bunker-5e85c324` -> rc=2, `/home/bunker-5e85c324` does not exist (32 other bunker-* homes present), `bunker list --status all` shows only 3 agents with 5e85c324 absent, direct agent SSH with ~/.bunker/keys/5e85c324 -> 'Permission denied (publickey,password)', no hermes processes / ~/.bunker/agents/ / systemd --user units; host itself healthy (`ssh bunker-mvp` SSH_OK, uptime 23d, `tailscale ping` pong). The '## Remediation path' section gives 3 numbered, actionable steps: (1) reprovision via `bunker create 5e85c324`, (2) or reassign QA-OFF-BY-ONE-RELENG-2 to one of the 3 surviving agents (446cf6d3, crier-lab, e37ccd4d), (3) prevent recurrence by reviewing buckerrd `reconciliation.mode=adopt`. The same diagnosis and owner-intervention path is recorded on the board row .coding-hermes/board/tasks.jsonl:270 (foreman_note, tick 2026-10-26-03-50). No test suite applies to this live-ops criterion: .gitreins/config.yaml sets guards.tests=false (verified by reading config; the prior tier-1 run in .gitreins/history/2026-10-26/36ef069e/verdict.json also shows tests skipped/passing with exit_code 0).
Root cause (agent 5e85c324 permanently deleted from bunker-mvp, host healthy) is documented with live command evidence and a concrete 3-step remediation path in docs/qa/ob-ops-002-collection.md and the board row, satisfying the OR arm of the criterion.

## Summary

Judge Result: OB-OPS-002

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ SSH connectivity restored OR root cause documented with remediation path: Connectivity was not restored (agent is gone, not merely unreachable), but the second arm is satisfied. docs/qa/ob-ops-002-collection.md (committed in 60fa16e6) states 'Status: ROOT-CAUSE IDENTIFIED (owner intervention required)' and documents the root cause with concrete evidence: `getent passwd bunker-5e85c324` -> rc=2, `/home/bunker-5e85c324` does not exist (32 other bunker-* homes present), `bunker list --status all` shows only 3 agents with 5e85c324 absent, direct agent SSH with ~/.bunker/keys/5e85c324 -> 'Permission denied (publickey,password)', no hermes processes / ~/.bunker/agents/ / systemd --user units; host itself healthy (`ssh bunker-mvp` SSH_OK, uptime 23d, `tailscale ping` pong). The '## Remediation path' section gives 3 numbered, actionable steps: (1) reprovision via `bunker create 5e85c324`, (2) or reassign QA-OFF-BY-ONE-RELENG-2 to one of the 3 surviving agents (446cf6d3, crier-lab, e37ccd4d), (3) prevent recurrence by reviewing buckerrd `reconciliation.mode=adopt`. The same diagnosis and owner-intervention path is recorded on the board row .coding-hermes/board/tasks.jsonl:270 (foreman_note, tick 2026-10-26-03-50). No test suite applies to this live-ops criterion: .gitreins/config.yaml sets guards.tests=false (verified by reading config; the prior tier-1 run in .gitreins/history/2026-10-26/36ef069e/verdict.json also shows tests skipped/passing with exit_code 0).
Root cause (agent 5e85c324 permanently deleted from bunker-mvp, host healthy) is documented with live command evidence and a concrete 3-step remediation path in docs/qa/ob-ops-002-collection.md and the board row, satisfying the OR arm of the criterion.

Overall: PASS ✓
