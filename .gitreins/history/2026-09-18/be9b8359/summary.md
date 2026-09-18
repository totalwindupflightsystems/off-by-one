# Verdict: OB-GAP-076

**Task:** P3 docs: live docs teach the bare go build ./cmd/off-by-one path that the freshness guard rejects
**Evaluated:** 2026-09-18T18:16:05.044204
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	2.508s
- ✗ **tier2**
  - INCOMPLETE

Cap exceeded: Input token budget (4.0M) exceeded (4.1M used). Increase max_input_tokens or reduce message context.

## Summary

Judge Result: OB-GAP-076

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	2.508s

Stage tier2: FAIL
  INCOMPLETE

Cap exceeded: Input token budget (4.0M) exceeded (4.1M used). Increase max_input_tokens or reduce message context.

Overall: FAIL ✗
