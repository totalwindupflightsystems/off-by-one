# Verdict: SKIPPED-install-bunker-OB-2026-09-25

**Task:** Prove release-binary installability on bunker-las-03
**Evaluated:** 2026-10-02T11:50:44.279314
**Result:** ✗ FAIL

## Pipeline Stages

- ✗ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: Command timed out
- ✗ **tier2**
  - INCOMPLETE
  ✗ A fresh bunker agent on bunker-las-03 downloads off-by-one-v0.1.1-linux-amd64 from GitHub Releases, verifies it against SHA256SUMS, prints --version v0.1.1, serves /health ok on :8766, and a discover call returns 200: Partial evidence only. PASS sub-clauses: fresh agent on bunker-las-03 (docs/dogfood/evidence/2026-10-02-install-bunker-las-03-raw.log L22 `bunker exec oby-install-test2 --server bunker-las-03`, agent created); download from GitHub Releases (md L47-53 curl -sfLO .../releases/download/v0.1.1/off-by-one-v0.1.1-linux-amd64); SHA256SUMS verify (raw.log L28 'off-by-one-v0.1.1-linux-amd64: OK'); --version (raw.log L29 'off-by-one v0.1.1'). FAIL sub-clauses: (1) '/health ok on :8766' — raw.log L32-34 shows {"status":"ok","uptime":"54m5s"} HTTPSTATUS:200 on :8766, but L35 explicitly flags it as an ANOMALY: 'orphaned serve from DESTROYED agent1 (shared host netns; port 8766 not agent-private). Re-proving on port 8799.' The re-proof (L36-38) is on port 8799, not 8766, so the :8766 health response is not attributable to the fresh agent. (2) 'a discover call returns 200' — raw.log L39-43 pre-seed discover on 8799 returns HTTPSTATUS:404; the 200 (L49-57, found:true) is only reached after an out-of-band corpus deploy (tar czf data + bunker cp, L45-48) plus a seed step, and the evidence doc itself states 'the release asset ships no corpus' — i.e. the bare release binary does not satisfy the discover clause, and the 200 was obtained on :8799, not :8766. No test exists for this path (guards.tests=false; no Go code changed this tick), and the three prior verdicts on this task (e560e642, e1079cf6, 0774890c) all FAILED for these same defects; HEAD 9b750778 only adds a raw log that documents the :8766 anomaly and relocates the passing proof to :8799.
The install proof is transcript-only and fails two clauses: the :8766 /health 200 is an orphaned serve from a destroyed agent (re-proved on :8799), and the discover 200 requires an out-of-band corpus seed the release asset cannot provide.

## Summary

Judge Result: SKIPPED-install-bunker-OB-2026-09-25

Stage tier1: FAIL
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✗ tests: Command timed out

Stage tier2: FAIL
  INCOMPLETE
  ✗ A fresh bunker agent on bunker-las-03 downloads off-by-one-v0.1.1-linux-amd64 from GitHub Releases, verifies it against SHA256SUMS, prints --version v0.1.1, serves /health ok on :8766, and a discover call returns 200: Partial evidence only. PASS sub-clauses: fresh agent on bunker-las-03 (docs/dogfood/evidence/2026-10-02-install-bunker-las-03-raw.log L22 `bunker exec oby-install-test2 --server bunker-las-03`, agent created); download from GitHub Releases (md L47-53 curl -sfLO .../releases/download/v0.1.1/off-by-one-v0.1.1-linux-amd64); SHA256SUMS verify (raw.log L28 'off-by-one-v0.1.1-linux-amd64: OK'); --version (raw.log L29 'off-by-one v0.1.1'). FAIL sub-clauses: (1) '/health ok on :8766' — raw.log L32-34 shows {"status":"ok","uptime":"54m5s"} HTTPSTATUS:200 on :8766, but L35 explicitly flags it as an ANOMALY: 'orphaned serve from DESTROYED agent1 (shared host netns; port 8766 not agent-private). Re-proving on port 8799.' The re-proof (L36-38) is on port 8799, not 8766, so the :8766 health response is not attributable to the fresh agent. (2) 'a discover call returns 200' — raw.log L39-43 pre-seed discover on 8799 returns HTTPSTATUS:404; the 200 (L49-57, found:true) is only reached after an out-of-band corpus deploy (tar czf data + bunker cp, L45-48) plus a seed step, and the evidence doc itself states 'the release asset ships no corpus' — i.e. the bare release binary does not satisfy the discover clause, and the 200 was obtained on :8799, not :8766. No test exists for this path (guards.tests=false; no Go code changed this tick), and the three prior verdicts on this task (e560e642, e1079cf6, 0774890c) all FAILED for these same defects; HEAD 9b750778 only adds a raw log that documents the :8766 anomaly and relocates the passing proof to :8799.
The install proof is transcript-only and fails two clauses: the :8766 /health 200 is an orphaned serve from a destroyed agent (re-proved on :8799), and the discover 200 requires an out-of-band corpus seed the release asset cannot provide.

Overall: FAIL ✗
