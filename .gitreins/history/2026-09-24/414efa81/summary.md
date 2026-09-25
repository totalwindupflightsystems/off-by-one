# Verdict: OB-DF14

**Task:** Corpus-vs-stats relationship corrected in docs
**Evaluated:** 2026-09-24T22:52:43.354106
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	2.833s
- ✓ **tier2**
  - COMPLETE
  ✓ README.md states the flat-file corpus is the stats-verified set minus placeholder/self-test classes (subset direction, corpus counts lower than live verified_answers); scripts/export-answers.py and data/README.md reference /master/ not /main/ with zero /main/ occurrences and cite only example filenames that exist under data/answers/: README.md:542 states 'The flat-file corpus is a strict subset of the live stats' verified set ... the corpus additionally excludes placeholder/self-test/canary/probe classes (EXCLUDED_CLASS_PATTERNS in scripts/export-answers.py), so corpus counts are slightly lower than the live stats' verified_answers, never higher' — correct subset direction. grep -n '/main/' scripts/export-answers.py data/README.md returned exit 1 (zero occurrences); both use /master/ (scripts/export-answers.py:219, data/README.md:20). Only example filename cited is data/answers/0001-unknown.json (export-answers.py:219,231; data/README.md:20,32) and it exists under data/answers/ (ls OK; 2189 json files present).
README documents the corpus as a strict subset of live verified_answers excluding placeholder/self-test classes, and both scripts/export-answers.py and data/README.md use /master/ with zero /main/ occurrences and only existing example filenames.

## Summary

Judge Result: OB-DF14

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	2.833s

Stage tier2: PASS
  COMPLETE
  ✓ README.md states the flat-file corpus is the stats-verified set minus placeholder/self-test classes (subset direction, corpus counts lower than live verified_answers); scripts/export-answers.py and data/README.md reference /master/ not /main/ with zero /main/ occurrences and cite only example filenames that exist under data/answers/: README.md:542 states 'The flat-file corpus is a strict subset of the live stats' verified set ... the corpus additionally excludes placeholder/self-test/canary/probe classes (EXCLUDED_CLASS_PATTERNS in scripts/export-answers.py), so corpus counts are slightly lower than the live stats' verified_answers, never higher' — correct subset direction. grep -n '/main/' scripts/export-answers.py data/README.md returned exit 1 (zero occurrences); both use /master/ (scripts/export-answers.py:219, data/README.md:20). Only example filename cited is data/answers/0001-unknown.json (export-answers.py:219,231; data/README.md:20,32) and it exists under data/answers/ (ls OK; 2189 json files present).
README documents the corpus as a strict subset of live verified_answers excluding placeholder/self-test classes, and both scripts/export-answers.py and data/README.md use /master/ with zero /main/ occurrences and only existing example filenames.

Overall: PASS ✓
