# Verdict: OBS-OB-006

**Task:** answers.jsonl monolith distribution: untrack + deterministic rebuild from per-class corpus
**Evaluated:** 2026-11-21T11:36:03.112688
**Result:** ✗ FAIL

## Criteria

- — **git ls-files data/answers.jsonl is empty and data/answers.jsonl is gitignored; a documented command (make corpus-jsonl / scripts/build-answers-jsonl.py) rebuilds byte-identical 7450-line corpus from data/answers/*.json verified by sha256 against the pre-change blob; make check-corpus-hygiene passes without the tracked monolith; no script or doc references a tracked data/answers.jsonl; docs state per-class files are canonical and the bulk file is generated**
  - Not examined — evaluation ended before this criterion was reached (no judgement was made about it)

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
- ✗ **tier2**
  - INCOMPLETE
  — git ls-files data/answers.jsonl is empty and data/answers.jsonl is gitignored; a documented command (make corpus-jsonl / scripts/build-answers-jsonl.py) rebuilds byte-identical 7450-line corpus from data/answers/*.json verified by sha256 against the pre-change blob; make check-corpus-hygiene passes without the tracked monolith; no script or doc references a tracked data/answers.jsonl; docs state per-class files are canonical and the bulk file is generated: Not examined — evaluation ended before this criterion was reached (no judgement was made about it)
  ✗ Coverage FAILED — not passing: git ls-files data/answers.jsonl is empty and data/answers.jsonl is gitignored; a documented command (make corpus-jsonl / scripts/build-answers-jsonl.py) rebuilds byte-identical 7450-line corpus from data/answers/*.json verified by sha256 against the pre-change blob; make check-corpus-hygiene passes without the tracked monolith; no script or doc references a tracked data/answers.jsonl; docs state per-class files are canonical and the bulk file is generated
Cap exceeded: Time cap (50m) exceeded (50m1s elapsed). Increase max_time or simplify criteria. Partial verdict — 0 of 1 criteria graded, 1 ungraded. The run spent 3,725,208 in / 13,961 out over 112.9 iterations.

## Summary

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check

Stage tier2: FAIL
  INCOMPLETE
  — git ls-files data/answers.jsonl is empty and data/answers.jsonl is gitignored; a documented command (make corpus-jsonl / scripts/build-answers-jsonl.py) rebuilds byte-identical 7450-line corpus from data/answers/*.json verified by sha256 against the pre-change blob; make check-corpus-hygiene passes without the tracked monolith; no script or doc references a tracked data/answers.jsonl; docs state per-class files are canonical and the bulk file is generated: Not examined — evaluation ended before this criterion was reached (no judgement was made about it)
  ✗ Coverage FAILED — not passing: git ls-files data/answers.jsonl is empty and data/answers.jsonl is gitignored; a documented command (make corpus-jsonl / scripts/build-answers-jsonl.py) rebuilds byte-identical 7450-line corpus from data/answers/*.json verified by sha256 against the pre-change blob; make check-corpus-hygiene passes without the tracked monolith; no script or doc references a tracked data/answers.jsonl; docs state per-class files are canonical and the bulk file is generated
Cap exceeded: Time cap (50m) exceeded (50m1s elapsed). Increase max_time or simplify criteria. Partial verdict — 0 of 1 criteria graded, 1 ungraded. The run spent 3,725,208 in / 13,961 out over 112.9 iterations.

Overall: FAIL ✗
