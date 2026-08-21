# Verdict: OB-GAP-054

**Task:** Binary freshness gate trips on data-only commits
**Evaluated:** 2026-08-21T10:47:47.235174
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ lint: 
  ✓ secrets: 
    ○
    │╲
    │ ○
    ○ ░
    ░    gitleaks

[90m5:45AM[0m [32mINF[0m [1mscanned ~5575863 b
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.019s
ok  	github.com/totalwindu
- ✗ **tier2**
  - INCOMPLETE

Cap exceeded: Input token budget (1.0M) exceeded (1.0M used). Increase max_input_tokens or reduce message context.

## Summary

Judge Result: OB-GAP-054

Stage tier1: PASS
    ✓ lint: 
  ✓ secrets: 
    ○
    │╲
    │ ○
    ○ ░
    ░    gitleaks

[90m5:45AM[0m [32mINF[0m [1mscanned ~5575863 b
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.019s
ok  	github.com/totalwindu

Stage tier2: FAIL
  INCOMPLETE

Cap exceeded: Input token budget (1.0M) exceeded (1.0M used). Increase max_input_tokens or reduce message context.

Overall: FAIL ✗
