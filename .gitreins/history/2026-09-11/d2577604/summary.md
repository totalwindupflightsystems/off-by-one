# Verdict: DF-OFF-BY-ONE-1

**Task:** Documented solve setup cannot solve (P0 dogfood)
**Evaluated:** 2026-09-11T10:25:29.812095
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m5:23AM[0m [32mINF[0m [1mscanned ~6030957 bytes (6.03 MB) in 994ms[0m
[90m5:23AM[0m [32m
- ✗ **tier2**
  - INCOMPLETE

Cap exceeded: Input token budget (2.0M) exceeded (2.1M used). Increase max_input_tokens or reduce message context.

## Summary

Judge Result: DF-OFF-BY-ONE-1

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m5:23AM[0m [32mINF[0m [1mscanned ~6030957 bytes (6.03 MB) in 994ms[0m
[90m5:23AM[0m [32m

Stage tier2: FAIL
  INCOMPLETE

Cap exceeded: Input token budget (2.0M) exceeded (2.1M used). Increase max_input_tokens or reduce message context.

Overall: FAIL ✗
