# Verdict: DOC-19

**Task:** Repair dead trace markers in board JSONL rows
**Evaluated:** 2026-11-25T07:06:38.647710
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ Coverage FAILED — ungraded: (prompt|evidence)=/tmp -> 0 on complete/closed rows of .coding-hermes/board/tasks.jsonl (23 occurrences/21 rows per re-measure); witness=live:scratch -> 0; witness=commit: -> 0 on complete/closed rows; trace_marker_check.py reports zero violations naming a complete/closed row; total 'row=' marker count unchanged; DOC-16's source files untouched
Evaluator error: LLM call failed: LLM request failed after 3 attempts (provider=openai model=glm-5.3-flash url=https://api.z.ai/api/coding/paas/v4/chat/completions key=<GITREINS_LLM_API_KEY>) — last error: 429 Client Error: Too Many Requests for url: https://api.z.ai/api/coding/paas/v4/chat/completions
- ✓ **fix_verification**
  - fix_verification: 0 finding(s) verified

## Summary

Stage tier1: PASS
    ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ Coverage FAILED — ungraded: (prompt|evidence)=/tmp -> 0 on complete/closed rows of .coding-hermes/board/tasks.jsonl (23 occurrences/21 rows per re-measure); witness=live:scratch -> 0; witness=commit: -> 0 on complete/closed rows; trace_marker_check.py reports zero violations naming a complete/closed row; total 'row=' marker count unchanged; DOC-16's source files untouched
Evaluator error: LLM call failed: LLM request failed after 3 attempts (provider=openai model=glm-5.3-flash url=https://api.z.ai/api/coding/paas/v4/chat/completions key=<GITREINS_LLM_API_KEY>) — last error: 429 Client Error: Too Many Requests for url: https://api.z.ai/api/coding/paas/v4/chat/completions

Stage fix_verification: PASS
  fix_verification: 0 finding(s) verified

Overall: FAIL ✗
