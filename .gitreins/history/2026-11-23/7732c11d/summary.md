# Verdict: DOC-11

**Task:** README CI/CD census: seven jobs -> eight jobs incl corpus-jsonl
**Evaluated:** 2026-11-23T22:37:40.514777
**Result:** ✓ PASS

## Criteria

- ✓ **README.md CI/CD section lists all 8 ci.yml jobs in file order including corpus-jsonl with its Corpus bulk artifact rebuild description; phrase 'seven jobs' absent from README.md; census re-derived at fix time**
  - Census re-derived at review time: `grep -nE "^  [a-zA-Z0-9_-]+:" .github/workflows/ci.yml` returns exactly 8 jobs in file order — build:11, transport-retry-selftest:86, deploy-gate-selftest:104, binary-seed-probe:143, corpus-hygiene:180, corpus-jsonl:211, readme-release-pin:231, golangci-lint:249. README.md:597 states "eight jobs" and lists those same 8 ids in the identical order; the CI/CD bullet count is 8 (`grep -c "^- \*\*"` on the section = 8) and each bullet title matches the job's `name:` field (ci.yml:212 `name: Corpus bulk artifact rebuild`, ci.yml:181 `name: Corpus host-path hygiene`, ci.yml:87 `name: Transport retry self-test`). The corpus-jsonl bullet's description is factually wired: ci.yml:214-217 sets `fetch-depth: 0` and ci.yml:223 runs `make corpus-jsonl-selftest`; Makefile:305-306 maps that target to scripts/tests/build_answers_jsonl_test.py, executed fresh in this review — exit 0, "Ran 25 tests in 26.351s / OK". 'seven jobs' absent: `grep -n "seven jobs" README.md` exit=1; wider stale-census sweep (seven|six jobs|7 jobs|8 jobs|nine jobs) exit=1, and "jobs" occurs in README only at line 597 with "eight jobs". Repo test suite passed in tier-1 (tests step exit_code 0). [resolution 0.17; README.md, ci.yml]

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
- ✓ **tier2**
  - COMPLETE
  ✓ README.md CI/CD section lists all 8 ci.yml jobs in file order including corpus-jsonl with its Corpus bulk artifact rebuild description; phrase 'seven jobs' absent from README.md; census re-derived at fix time: Census re-derived at review time: `grep -nE "^  [a-zA-Z0-9_-]+:" .github/workflows/ci.yml` returns exactly 8 jobs in file order — build:11, transport-retry-selftest:86, deploy-gate-selftest:104, binary-seed-probe:143, corpus-hygiene:180, corpus-jsonl:211, readme-release-pin:231, golangci-lint:249. README.md:597 states "eight jobs" and lists those same 8 ids in the identical order; the CI/CD bullet count is 8 (`grep -c "^- \*\*"` on the section = 8) and each bullet title matches the job's `name:` field (ci.yml:212 `name: Corpus bulk artifact rebuild`, ci.yml:181 `name: Corpus host-path hygiene`, ci.yml:87 `name: Transport retry self-test`). The corpus-jsonl bullet's description is factually wired: ci.yml:214-217 sets `fetch-depth: 0` and ci.yml:223 runs `make corpus-jsonl-selftest`; Makefile:305-306 maps that target to scripts/tests/build_answers_jsonl_test.py, executed fresh in this review — exit 0, "Ran 25 tests in 26.351s / OK". 'seven jobs' absent: `grep -n "seven jobs" README.md` exit=1; wider stale-census sweep (seven|six jobs|7 jobs|8 jobs|nine jobs) exit=1, and "jobs" occurs in README only at line 597 with "eight jobs". Repo test suite passed in tier-1 (tests step exit_code 0). [resolution 0.17; README.md, ci.yml]
README.md:597 now lists all 8 ci.yml jobs in exact file order with an accurate Corpus bulk artifact rebuild (corpus-jsonl) description whose cited make target runs green (25 tests OK), and no stale 'seven jobs' census remains anywhere in the file.
- ✓ **fix_verification**
  - fix_verification: 0 finding(s) verified

## Summary

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check

Stage tier2: PASS
  COMPLETE
  ✓ README.md CI/CD section lists all 8 ci.yml jobs in file order including corpus-jsonl with its Corpus bulk artifact rebuild description; phrase 'seven jobs' absent from README.md; census re-derived at fix time: Census re-derived at review time: `grep -nE "^  [a-zA-Z0-9_-]+:" .github/workflows/ci.yml` returns exactly 8 jobs in file order — build:11, transport-retry-selftest:86, deploy-gate-selftest:104, binary-seed-probe:143, corpus-hygiene:180, corpus-jsonl:211, readme-release-pin:231, golangci-lint:249. README.md:597 states "eight jobs" and lists those same 8 ids in the identical order; the CI/CD bullet count is 8 (`grep -c "^- \*\*"` on the section = 8) and each bullet title matches the job's `name:` field (ci.yml:212 `name: Corpus bulk artifact rebuild`, ci.yml:181 `name: Corpus host-path hygiene`, ci.yml:87 `name: Transport retry self-test`). The corpus-jsonl bullet's description is factually wired: ci.yml:214-217 sets `fetch-depth: 0` and ci.yml:223 runs `make corpus-jsonl-selftest`; Makefile:305-306 maps that target to scripts/tests/build_answers_jsonl_test.py, executed fresh in this review — exit 0, "Ran 25 tests in 26.351s / OK". 'seven jobs' absent: `grep -n "seven jobs" README.md` exit=1; wider stale-census sweep (seven|six jobs|7 jobs|8 jobs|nine jobs) exit=1, and "jobs" occurs in README only at line 597 with "eight jobs". Repo test suite passed in tier-1 (tests step exit_code 0). [resolution 0.17; README.md, ci.yml]
README.md:597 now lists all 8 ci.yml jobs in exact file order with an accurate Corpus bulk artifact rebuild (corpus-jsonl) description whose cited make target runs green (25 tests OK), and no stale 'seven jobs' census remains anywhere in the file.

Stage fix_verification: PASS
  fix_verification: 0 finding(s) verified

Overall: PASS ✓
