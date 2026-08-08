# Verdict: OB-GAP-003

**Task:** README raw URL must point at the master branch
**Evaluated:** 2026-08-08T21:43:37.928708
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: /bin/sh: 1: gitleaks: not found

  ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
- ✓ **tier2**
  - COMPLETE
  ✓ README.md line ~315 curl -O command uses master (not main) in the raw.githubusercontent.com/totalwindupflightsystems/off-by-one/ URL: README.md line 315: `curl -O https://raw.githubusercontent.com/totalwindupflightsystems/off-by-one/master/data/answers/0001-unknown.json` uses master. Commit a33087e changed main->master.
  ✓ The documented URL https://raw.githubusercontent.com/totalwindupflightsystems/off-by-one/master/data/answers/0001-unknown.json returns HTTP 200: curl -s -o /dev/null -w '%{http_code}' returned 200 for the documented URL.
Both criteria verified: README line 315 uses master in the raw URL, and the documented URL returns HTTP 200.

## Summary

Judge Result: OB-GAP-003

Stage tier1: PASS
    ✓ secrets: /bin/sh: 1: gitleaks: not found

  ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t

Stage tier2: PASS
  COMPLETE
  ✓ README.md line ~315 curl -O command uses master (not main) in the raw.githubusercontent.com/totalwindupflightsystems/off-by-one/ URL: README.md line 315: `curl -O https://raw.githubusercontent.com/totalwindupflightsystems/off-by-one/master/data/answers/0001-unknown.json` uses master. Commit a33087e changed main->master.
  ✓ The documented URL https://raw.githubusercontent.com/totalwindupflightsystems/off-by-one/master/data/answers/0001-unknown.json returns HTTP 200: curl -s -o /dev/null -w '%{http_code}' returned 200 for the documented URL.
Both criteria verified: README line 315 uses master in the raw URL, and the documented URL returns HTTP 200.

Overall: PASS ✓
