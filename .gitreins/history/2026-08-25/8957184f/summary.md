# Verdict: OB-GAP-056

**Task:** Document /api/v1/stats coverage formula
**Evaluated:** 2026-08-25T00:39:26.251075
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m7:38PM[0m [32mINF[0m [1mscanned ~9215657 bytes (9.22 MB) in 1.47s[0m
[90m7:38PM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ README.md and pkg/api/openapi.yaml define coverage = verified_answers / total_problems and state values > 1 are normal (a problem class can hold multiple verified answers); docs/integration.md and docs/api-reference.md stats examples show a coverage value matching live /api/v1/stats within tolerance: README.md:113 defines 'coverage = verified_answers/total_problems and can exceed 1.0 (a class may hold multiple verified answers)'. pkg/api/openapi.yaml:685-688 defines coverage = 'verified_answers / total_problems. Can exceed 1.0 — a problem class may hold multiple verified answers, so a value above 1 is normal, not corruption.' docs/integration.md:219 and docs/api-reference.md:522 both show coverage=1.137 in stats examples. Live curl http://localhost:8766/api/v1/stats returns coverage=1.1373926619828258 (verified_answers=1457, total_problems=1281; 1457/1281=1.13739), which rounds to 1.137 — matching the documented examples within tolerance.
All documentation (README.md, pkg/api/openapi.yaml, docs/integration.md, docs/api-reference.md) correctly define coverage = verified_answers/total_problems, state values >1 are normal, and the documented coverage example (1.137) matches the live /api/v1/stats value (1.1374) within tolerance.

## Summary

Judge Result: OB-GAP-056

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m7:38PM[0m [32mINF[0m [1mscanned ~9215657 bytes (9.22 MB) in 1.47s[0m
[90m7:38PM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ README.md and pkg/api/openapi.yaml define coverage = verified_answers / total_problems and state values > 1 are normal (a problem class can hold multiple verified answers); docs/integration.md and docs/api-reference.md stats examples show a coverage value matching live /api/v1/stats within tolerance: README.md:113 defines 'coverage = verified_answers/total_problems and can exceed 1.0 (a class may hold multiple verified answers)'. pkg/api/openapi.yaml:685-688 defines coverage = 'verified_answers / total_problems. Can exceed 1.0 — a problem class may hold multiple verified answers, so a value above 1 is normal, not corruption.' docs/integration.md:219 and docs/api-reference.md:522 both show coverage=1.137 in stats examples. Live curl http://localhost:8766/api/v1/stats returns coverage=1.1373926619828258 (verified_answers=1457, total_problems=1281; 1457/1281=1.13739), which rounds to 1.137 — matching the documented examples within tolerance.
All documentation (README.md, pkg/api/openapi.yaml, docs/integration.md, docs/api-reference.md) correctly define coverage = verified_answers/total_problems, state values >1 are normal, and the documented coverage example (1.137) matches the live /api/v1/stats value (1.1374) within tolerance.

Overall: PASS ✓
