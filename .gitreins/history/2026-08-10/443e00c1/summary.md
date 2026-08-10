# Verdict: OB-GAP-025

**Task:** Public corpus hygiene: exclude self-test/canary/probe classes from export outputs
**Evaluated:** 2026-08-10T13:34:20.896558
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
  ✓ secrets: [90m8:33AM[0m [32mINF[0m [1mscanned ~8222103 bytes (8.22 MB) in 2.4s[0m
[90m8:33AM[0m [33mW
- ✓ **tier2**
  - COMPLETE
  ✓ grep -iE 'self-test|dogfood|canary|field-test|shell-say-hello|shell-echo-hello|test-gap-sweep|test-foreman|e2e-tick' data/INDEX.md returns 0: grep on data/INDEX.md returns exit 1 (0 matches); wc -l confirms 0 lines
  ✓ Legit classes test-mocking-http-requests (110) and test-property-based-shrinking (111) still in data/INDEX.md: data/INDEX.md contains '| 110 | test-mocking-http-requests | 1 | go |' and '| 111 | test-property-based-shrinking | 1 | python |'
  ✓ data/answers.jsonl and data/answers/ contain no excluded probe classes; stale per-class files deleted: data/answers.jsonl (856 answers) has no probe classes (grep exit 1); data/answers/ (809 files) has no probe files (grep exit 1); stale files for excluded classes 0010/0581/0873/0003/0009/0017 deleted
  ✓ Export script runs clean end-to-end: python3 scripts/export-answers.py exits 0, excludes 74 self-test/canary/probe classes, exports 809 classes/856 answers to data/
All four corpus-hygiene criteria verified: probe classes excluded from INDEX.md, answers.jsonl, and answers/; legit classes 110/111 retained; export script runs clean.

## Summary

Judge Result: OB-GAP-025

Stage tier1: PASS
    ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
  ✓ secrets: [90m8:33AM[0m [32mINF[0m [1mscanned ~8222103 bytes (8.22 MB) in 2.4s[0m
[90m8:33AM[0m [33mW

Stage tier2: PASS
  COMPLETE
  ✓ grep -iE 'self-test|dogfood|canary|field-test|shell-say-hello|shell-echo-hello|test-gap-sweep|test-foreman|e2e-tick' data/INDEX.md returns 0: grep on data/INDEX.md returns exit 1 (0 matches); wc -l confirms 0 lines
  ✓ Legit classes test-mocking-http-requests (110) and test-property-based-shrinking (111) still in data/INDEX.md: data/INDEX.md contains '| 110 | test-mocking-http-requests | 1 | go |' and '| 111 | test-property-based-shrinking | 1 | python |'
  ✓ data/answers.jsonl and data/answers/ contain no excluded probe classes; stale per-class files deleted: data/answers.jsonl (856 answers) has no probe classes (grep exit 1); data/answers/ (809 files) has no probe files (grep exit 1); stale files for excluded classes 0010/0581/0873/0003/0009/0017 deleted
  ✓ Export script runs clean end-to-end: python3 scripts/export-answers.py exits 0, excludes 74 self-test/canary/probe classes, exports 809 classes/856 answers to data/
All four corpus-hygiene criteria verified: probe classes excluded from INDEX.md, answers.jsonl, and answers/; legit classes 110/111 retained; export script runs clean.

Overall: PASS ✓
