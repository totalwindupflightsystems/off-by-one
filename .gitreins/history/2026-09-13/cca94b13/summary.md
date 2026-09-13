# Verdict: DF-OFF-BY-ONE-7

**Task:** Import reuses a stale clone when source_repo differs (silent wrong-source import)
**Evaluated:** 2026-09-13T01:06:31.990953
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.120s
ok  	github.com/totalwindu
  ✓ secrets: [90m8:05PM[0m [32mINF[0m [1mscanned ~9619844 bytes (9.62 MB) in 2.27s[0m
[90m8:05PM[0m [32m
- ✗ **tier2**
  - INCOMPLETE
  ✓ internal/export prepareClone verifies the existing clone's origin URL against cfg.RepoURL (whitespace, trailing slash and trailing .git stripped) and returns an error naming BOTH URLs without running fetch when they differ: internal/export/git.go:224-240: prepareClone stats .git, runs gitOutput(ctx, LocalDir, "remote","get-url","origin"), compares normalizeRepoURL(origin) != normalizeRepoURL(e.cfg.RepoURL) and returns fmt.Errorf("%w: existing clone origin %q, requested RepoURL %q", ErrRepoMismatch, strings.TrimSpace(origin), e.cfg.RepoURL) — names BOTH URLs — before the fetch at git.go:243. normalizeRepoURL (git.go:399-410) does strings.TrimSpace, TrimSuffix "/", TrimSuffix ".git", TrimSuffix "/". ErrRepoMismatch declared at git.go:142.
  ✗ The import handler maps that error to HTTP 409 with error source_repo_mismatch (never 200 with skipped>=1); a handler test asserts 409 for a mismatching source_repo and an export test asserts the mismatch error with no fetch: Handler mapping is correct: internal/api/handlers.go:873-876 uses errors.Is(err, importgit.ErrRepoMismatch) -> writeError(w, http.StatusConflict, "source_repo_mismatch", err.Error()). Handler test TestImportSourceRepoMismatch_Conflict PASSES (asserts 409, error==source_repo_mismatch, message names both URLs). Import test TestImport_ExistingCloneMismatchedRepo_NoFetch PASSES. BUT the required EXPORT test is missing: grep -rn 'ErrRepoMismatch' internal/export/ returns only git.go (lines 138,142,235) — no test file reference. internal/export/git_test.go contains only TestExport_ClassMismatch (unrelated) and TestExport_PullExistingClone (matching origin); no test asserts the export mismatch error with no fetch.
  ✓ go build ./... and gofmt -l cmd internal pkg sql are clean; go test ./... -short passes; new tests cover mismatch-rejected AND matching-origin-reuse-still-imports: go build ./... exit_code=0; gofmt -l cmd internal pkg sql produced empty output (exit 0); go test ./... -short -count=1 exit 0 with all 13 packages 'ok'. New tests verified running (not skipped): TestImport_ExistingCloneMismatchedRepo_Errors PASS, TestImport_ExistingCloneMismatchedRepo_NoFetch PASS, TestImport_ExistingCloneMatchingRepo_Proceeds PASS (Added=1, matching-origin reuse still imports), TestNormalizeRepoURL PASS, TestImportSourceRepoMismatch_Conflict PASS.
The export/import origin-mismatch guard and 409 handler mapping are implemented and all builds/tests pass, but the criterion-required export test asserting the mismatch error with no fetch is absent, so the task is incomplete.

## Summary

Judge Result: DF-OFF-BY-ONE-7

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.120s
ok  	github.com/totalwindu
  ✓ secrets: [90m8:05PM[0m [32mINF[0m [1mscanned ~9619844 bytes (9.62 MB) in 2.27s[0m
[90m8:05PM[0m [32m

Stage tier2: FAIL
  INCOMPLETE
  ✓ internal/export prepareClone verifies the existing clone's origin URL against cfg.RepoURL (whitespace, trailing slash and trailing .git stripped) and returns an error naming BOTH URLs without running fetch when they differ: internal/export/git.go:224-240: prepareClone stats .git, runs gitOutput(ctx, LocalDir, "remote","get-url","origin"), compares normalizeRepoURL(origin) != normalizeRepoURL(e.cfg.RepoURL) and returns fmt.Errorf("%w: existing clone origin %q, requested RepoURL %q", ErrRepoMismatch, strings.TrimSpace(origin), e.cfg.RepoURL) — names BOTH URLs — before the fetch at git.go:243. normalizeRepoURL (git.go:399-410) does strings.TrimSpace, TrimSuffix "/", TrimSuffix ".git", TrimSuffix "/". ErrRepoMismatch declared at git.go:142.
  ✗ The import handler maps that error to HTTP 409 with error source_repo_mismatch (never 200 with skipped>=1); a handler test asserts 409 for a mismatching source_repo and an export test asserts the mismatch error with no fetch: Handler mapping is correct: internal/api/handlers.go:873-876 uses errors.Is(err, importgit.ErrRepoMismatch) -> writeError(w, http.StatusConflict, "source_repo_mismatch", err.Error()). Handler test TestImportSourceRepoMismatch_Conflict PASSES (asserts 409, error==source_repo_mismatch, message names both URLs). Import test TestImport_ExistingCloneMismatchedRepo_NoFetch PASSES. BUT the required EXPORT test is missing: grep -rn 'ErrRepoMismatch' internal/export/ returns only git.go (lines 138,142,235) — no test file reference. internal/export/git_test.go contains only TestExport_ClassMismatch (unrelated) and TestExport_PullExistingClone (matching origin); no test asserts the export mismatch error with no fetch.
  ✓ go build ./... and gofmt -l cmd internal pkg sql are clean; go test ./... -short passes; new tests cover mismatch-rejected AND matching-origin-reuse-still-imports: go build ./... exit_code=0; gofmt -l cmd internal pkg sql produced empty output (exit 0); go test ./... -short -count=1 exit 0 with all 13 packages 'ok'. New tests verified running (not skipped): TestImport_ExistingCloneMismatchedRepo_Errors PASS, TestImport_ExistingCloneMismatchedRepo_NoFetch PASS, TestImport_ExistingCloneMatchingRepo_Proceeds PASS (Added=1, matching-origin reuse still imports), TestNormalizeRepoURL PASS, TestImportSourceRepoMismatch_Conflict PASS.
The export/import origin-mismatch guard and 409 handler mapping are implemented and all builds/tests pass, but the criterion-required export test asserting the mismatch error with no fetch is absent, so the task is incomplete.

Overall: FAIL ✗
