# Verdict: DF-OFF-BY-ONE-1

**Task:** Documented solve setup cannot solve (P0 dogfood)
**Evaluated:** 2026-09-11T10:21:55.376920
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m5:19AM[0m [32mINF[0m [1mscanned ~5845260 bytes (5.85 MB) in 1.25s[0m
[90m5:19AM[0m [32m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
- ✗ **tier2**
  - INCOMPLETE

Cap exceeded: Iteration cap (50) reached (50.0 used). Increase max_iterations or split criteria.

## Summary

Judge Result: DF-OFF-BY-ONE-1

Stage tier1: PASS
    ✓ secrets: [90m5:19AM[0m [32mINF[0m [1mscanned ~5845260 bytes (5.85 MB) in 1.25s[0m
[90m5:19AM[0m [32m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin

Stage tier2: FAIL
  INCOMPLETE

Cap exceeded: Iteration cap (50) reached (50.0 used). Increase max_iterations or split criteria.

Overall: FAIL ✗
