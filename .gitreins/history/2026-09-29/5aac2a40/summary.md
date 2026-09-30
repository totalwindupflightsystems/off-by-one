# Verdict: QA-OFF-BY-ONE-25

**Task:** Fix 32 golangci-lint issues at HEAD
**Evaluated:** 2026-09-29T18:11:38.627411
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	3.014s
- ✓ **tier2**
  - COMPLETE
  ✓ golangci-lint run --timeout=2m ./... returns 0 issues at HEAD: Ran the exact command at HEAD 980a17e0 (golangci-lint v2.12.2, go1.26.5): exit_code=0, output '0 issues.' Working tree has no modified *.go files (git status --short -- '*.go' empty), so the result reflects HEAD. Non-vacuity verified: reverting one fix (internal/graph/search.go:129 to `defer rows.Close()`) yielded exit_code=1 with 'internal/graph/search.go:129:18: Error return value of `rows.Close` is not checked (errcheck)' and '1 issues: * errcheck: 1'; after restoring the file the run returned exit_code=0 / '0 issues.' again. Fix commit 980a17e0 resolves the issues genuinely (27x errcheck explicit error handling on deferred Close/CloseNow, ST1023 + 4x QF1003 in internal/api + pkg/api/yaml.go) with zero `nolint` directives anywhere in the repo, and CI now gates the identical command (.github/workflows/ci.yml:198-215, args: --timeout=2m ./...).
golangci-lint run --timeout=2m ./... exits 0 with '0 issues.' at HEAD 980a17e0, and the run is proven non-vacuous by a temporary injected errcheck violation that was correctly detected.

## Summary

Judge Result: QA-OFF-BY-ONE-25

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	3.014s

Stage tier2: PASS
  COMPLETE
  ✓ golangci-lint run --timeout=2m ./... returns 0 issues at HEAD: Ran the exact command at HEAD 980a17e0 (golangci-lint v2.12.2, go1.26.5): exit_code=0, output '0 issues.' Working tree has no modified *.go files (git status --short -- '*.go' empty), so the result reflects HEAD. Non-vacuity verified: reverting one fix (internal/graph/search.go:129 to `defer rows.Close()`) yielded exit_code=1 with 'internal/graph/search.go:129:18: Error return value of `rows.Close` is not checked (errcheck)' and '1 issues: * errcheck: 1'; after restoring the file the run returned exit_code=0 / '0 issues.' again. Fix commit 980a17e0 resolves the issues genuinely (27x errcheck explicit error handling on deferred Close/CloseNow, ST1023 + 4x QF1003 in internal/api + pkg/api/yaml.go) with zero `nolint` directives anywhere in the repo, and CI now gates the identical command (.github/workflows/ci.yml:198-215, args: --timeout=2m ./...).
golangci-lint run --timeout=2m ./... exits 0 with '0 issues.' at HEAD 980a17e0, and the run is proven non-vacuous by a temporary injected errcheck violation that was correctly detected.

Overall: PASS ✓
