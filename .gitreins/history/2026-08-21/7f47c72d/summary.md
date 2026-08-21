# Verdict: OB-GAP-054

**Task:** Binary freshness gate trips on data-only commits
**Evaluated:** 2026-08-21T10:50:57.840246
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m5:48AM[0m [32mINF[0m [1mscanned ~5580379 bytes (5.58 MB) in 756ms[0m
[90m5:48AM[0m [32m
- ✗ **tier2**
  - INCOMPLETE

(auto-parsed from non-JSON response) Confirmed: the working-tree Makefile (fixed) differs from b221acc's Makefile, so the code-path diff includes `Makefile`, causing the check to trip.

This is the crux. The scenario (1) as literally described (binary built from b221acc, at HEAD 7c26471 with the fix applied) would trip because the fix 

## Summary

Judge Result: OB-GAP-054

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m5:48AM[0m [32mINF[0m [1mscanned ~5580379 bytes (5.58 MB) in 756ms[0m
[90m5:48AM[0m [32m

Stage tier2: FAIL
  INCOMPLETE

(auto-parsed from non-JSON response) Confirmed: the working-tree Makefile (fixed) differs from b221acc's Makefile, so the code-path diff includes `Makefile`, causing the check to trip.

This is the crux. The scenario (1) as literally described (binary built from b221acc, at HEAD 7c26471 with the fix applied) would trip because the fix 

Overall: FAIL ✗
