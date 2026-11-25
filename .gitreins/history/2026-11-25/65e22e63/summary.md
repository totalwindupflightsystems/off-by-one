# Verdict: DOC-19

**Task:** Repair dead trace markers in board JSONL rows
**Evaluated:** 2026-11-25T07:34:56.003887
**Result:** ✗ FAIL

## Criteria

- — **(prompt|evidence)=/tmp -> 0 on complete/closed rows of .coding-hermes/board/tasks.jsonl (23 occurrences/21 rows per re-measure); witness=live:scratch -> 0; witness=commit: -> 0 on complete/closed rows; trace_marker_check.py reports zero violations naming a complete/closed row; total 'row=' marker count unchanged; DOC-16's source files untouched**
  - Not examined — evaluation ended before this criterion was reached (no judgement was made about it)

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
- ✗ **tier2**
  - INCOMPLETE
  — (prompt|evidence)=/tmp -> 0 on complete/closed rows of .coding-hermes/board/tasks.jsonl (23 occurrences/21 rows per re-measure); witness=live:scratch -> 0; witness=commit: -> 0 on complete/closed rows; trace_marker_check.py reports zero violations naming a complete/closed row; total 'row=' marker count unchanged; DOC-16's source files untouched: Not examined — evaluation ended before this criterion was reached (no judgement was made about it)
  ✗ Coverage FAILED — not passing: (prompt|evidence)=/tmp -> 0 on complete/closed rows of .coding-hermes/board/tasks.jsonl (23 occurrences/21 rows per re-measure); witness=live:scratch -> 0; witness=commit: -> 0 on complete/closed rows; trace_marker_check.py reports zero violations naming a complete/closed row; total 'row=' marker count unchanged; DOC-16's source files untouched
Cap exceeded: Iteration cap (200) reached (201.0 used). Increase max_iterations or split criteria. Partial verdict — 0 of 1 criteria graded, 1 ungraded. The run spent 2,125,489 in / 12,438 out over 201.0 iterations.
- ✓ **fix_verification**
  - fix_verification: 0 finding(s) verified

## Summary

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check

Stage tier2: FAIL
  INCOMPLETE
  — (prompt|evidence)=/tmp -> 0 on complete/closed rows of .coding-hermes/board/tasks.jsonl (23 occurrences/21 rows per re-measure); witness=live:scratch -> 0; witness=commit: -> 0 on complete/closed rows; trace_marker_check.py reports zero violations naming a complete/closed row; total 'row=' marker count unchanged; DOC-16's source files untouched: Not examined — evaluation ended before this criterion was reached (no judgement was made about it)
  ✗ Coverage FAILED — not passing: (prompt|evidence)=/tmp -> 0 on complete/closed rows of .coding-hermes/board/tasks.jsonl (23 occurrences/21 rows per re-measure); witness=live:scratch -> 0; witness=commit: -> 0 on complete/closed rows; trace_marker_check.py reports zero violations naming a complete/closed row; total 'row=' marker count unchanged; DOC-16's source files untouched
Cap exceeded: Iteration cap (200) reached (201.0 used). Increase max_iterations or split criteria. Partial verdict — 0 of 1 criteria graded, 1 ungraded. The run spent 2,125,489 in / 12,438 out over 201.0 iterations.

Stage fix_verification: PASS
  fix_verification: 0 finding(s) verified

Overall: FAIL ✗
