# Verdict: DF-OFF-BY-ONE-41

**Task:** Surface resolved/missing required_tools via API
**Evaluated:** 2026-10-13T18:06:46.925267
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✗ **tier2**
  - INCOMPLETE

Cap exceeded: Time cap (25m) exceeded (25m4s elapsed). Increase max_time or simplify criteria.

## Summary

Judge Result: DF-OFF-BY-ONE-41

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: FAIL
  INCOMPLETE

Cap exceeded: Time cap (25m) exceeded (25m4s elapsed). Increase max_time or simplify criteria.

Overall: FAIL ✗
