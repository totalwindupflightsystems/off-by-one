# Verdict: OB-GAP-021

**Task:** README corpus counts must not hardcode stale numbers; export stamps data/COUNTS.md; README references canonical sources
**Evaluated:** 2026-08-10T13:33:32.893133
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
  ✓ secrets: [90m8:33AM[0m [32mINF[0m [1mscanned ~8213478 bytes (8.21 MB) in 2.17s[0m
[90m8:33AM[0m [33m
- ✓ **tier2**
  - COMPLETE
  ✓ grep -E '812|948' README.md returns 0: grep -E '812|948' README.md returns exit 1 (no matches)
  ✓ README.md references data/INDEX.md as canonical count source: README.md:316 states 'canonical corpus counts: [data/INDEX.md](data/INDEX.md) and [data/COUNTS.md](data/COUNTS.md)'
  ✓ python3 scripts/export-answers.py regenerates data/COUNTS.md with live counts: Script ran exit 0 and regenerated data/COUNTS.md with live counts (809 classes / 856 answers, stamped 2026-08-10 13:33 UTC); scripts/export-answers.py:180-187 writes COUNTS.md with live counts
All three criteria pass: no stale numbers in README, README references data/INDEX.md as canonical source, and the export script regenerates data/COUNTS.md with live counts.

## Summary

Judge Result: OB-GAP-021

Stage tier1: PASS
    ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
  ✓ secrets: [90m8:33AM[0m [32mINF[0m [1mscanned ~8213478 bytes (8.21 MB) in 2.17s[0m
[90m8:33AM[0m [33m

Stage tier2: PASS
  COMPLETE
  ✓ grep -E '812|948' README.md returns 0: grep -E '812|948' README.md returns exit 1 (no matches)
  ✓ README.md references data/INDEX.md as canonical count source: README.md:316 states 'canonical corpus counts: [data/INDEX.md](data/INDEX.md) and [data/COUNTS.md](data/COUNTS.md)'
  ✓ python3 scripts/export-answers.py regenerates data/COUNTS.md with live counts: Script ran exit 0 and regenerated data/COUNTS.md with live counts (809 classes / 856 answers, stamped 2026-08-10 13:33 UTC); scripts/export-answers.py:180-187 writes COUNTS.md with live counts
All three criteria pass: no stale numbers in README, README references data/INDEX.md as canonical source, and the export script regenerates data/COUNTS.md with live counts.

Overall: PASS ✓
