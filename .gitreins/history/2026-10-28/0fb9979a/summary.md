# Verdict: OB-GAP-077-R2

**Task:** P2 - DEPLOY DRIFT RECURRENCE: live :8766 served a superseded artifact (MainPID 988251 /proc/exe -> deleted inode) while every tick gate read green; the repo's own close-out gate make tick-closeout was RED
**Evaluated:** 2026-10-28T05:48:53.192238
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE

(auto-parsed from non-JSON response — JSON parse failed: Missing required fields 'verdict' or 'items') All four legs of the criterion are independently verified with live command output.

**Evidence summary:**

| Leg | Result |
|---|---|
| `make check-deploy` | exit **0** — `PASS: stamp 'v0.2.0-130-g151e8de3' resolves to 151e8de3 — code paths match HEAD`; ExecStart flags match `deploy/off-by-one.serv

## Summary

Judge Result: OB-GAP-077-R2

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE

(auto-parsed from non-JSON response — JSON parse failed: Missing required fields 'verdict' or 'items') All four legs of the criterion are independently verified with live command output.

**Evidence summary:**

| Leg | Result |
|---|---|
| `make check-deploy` | exit **0** — `PASS: stamp 'v0.2.0-130-g151e8de3' resolves to 151e8de3 — code paths match HEAD`; ExecStart flags match `deploy/off-by-one.serv

Overall: PASS ✓
