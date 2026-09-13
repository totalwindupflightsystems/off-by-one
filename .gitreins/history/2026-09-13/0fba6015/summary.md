# Verdict: DF-OFF-BY-ONE-7

**Task:** Import reuses a stale clone when source_repo differs (silent wrong-source import)
**Evaluated:** 2026-09-13T01:11:44.132814
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m8:11PM[0m [32mINF[0m [1mscanned ~6019214 bytes (6.02 MB) in 1.02s[0m
[90m8:11PM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ internal/export prepareClone verifies the existing clone's origin URL against cfg.RepoURL (whitespace, trailing slash and trailing .git stripped) and returns an error naming BOTH URLs without running fetch when they differ: internal/export/git.go prepareClone (commit 1af356e): reads origin via e.gitOutput(ctx, LocalDir, "remote","get-url","origin"), then `if normalizeRepoURL(origin) != normalizeRepoURL(e.cfg.RepoURL) { return fmt.Errorf("%w: existing clone origin %q, requested RepoURL %q", ErrRepoMismatch, strings.TrimSpace(origin), e.cfg.RepoURL) }` — names both URLs and returns before the fetch/checkout/pull block. normalizeRepoURL does TrimSpace + TrimSuffix("/") + TrimSuffix(".git") + TrimSuffix("/"). TestNormalizeRepoURL pins whitespace/trailing-slash/.git cases and strictness.
  ✓ The import handler maps that error to HTTP 409 with error source_repo_mismatch (never 200 with skipped>=1); a handler test asserts 409 for a mismatching source_repo and an export test asserts the mismatch error with no fetch: internal/api/handlers.go:873 `if errors.Is(err, importgit.ErrRepoMismatch) { writeError(w, http.StatusConflict, "source_repo_mismatch", err.Error()); return }` — placed before the 200 writeJSON path. TestImportSourceRepoMismatch_Conflict (handlers_test.go) asserts status==409, body error=="source_repo_mismatch", and message names both repoA and bogusRepo. TestExport_ExistingCloneMismatchedRepo_Errors asserts errors.Is(err, ErrRepoMismatch) and no fetch (refs/remotes/origin/main unchanged after remote advanced). Both PASS.
  ✓ go build ./... and gofmt -l cmd internal pkg sql are clean; go test ./... -short passes; new tests cover mismatch-rejected AND matching-origin-reuse-still-imports: `go build ./...` exit 0; `gofmt -l cmd internal pkg sql` produced no output (exit 0); `go test ./... -short -count=1` all packages ok (api, export, import, etc.). Targeted run: TestImport_ExistingCloneMismatchedRepo_Errors PASS, TestImport_ExistingCloneMismatchedRepo_NoFetch PASS, TestImport_ExistingCloneMatchingRepo_Proceeds PASS (matching-origin reuse still imports), TestExport_ExistingCloneMismatchedRepo_Errors PASS, TestImportSourceRepoMismatch_Conflict PASS — none skipped.
All three criteria verified: prepareClone origin guard with URL normalization and both-URL error, handler 409 source_repo_mismatch mapping, and clean build/gofmt/tests with mismatch-rejected and matching-reuse coverage.

## Summary

Judge Result: DF-OFF-BY-ONE-7

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m8:11PM[0m [32mINF[0m [1mscanned ~6019214 bytes (6.02 MB) in 1.02s[0m
[90m8:11PM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ internal/export prepareClone verifies the existing clone's origin URL against cfg.RepoURL (whitespace, trailing slash and trailing .git stripped) and returns an error naming BOTH URLs without running fetch when they differ: internal/export/git.go prepareClone (commit 1af356e): reads origin via e.gitOutput(ctx, LocalDir, "remote","get-url","origin"), then `if normalizeRepoURL(origin) != normalizeRepoURL(e.cfg.RepoURL) { return fmt.Errorf("%w: existing clone origin %q, requested RepoURL %q", ErrRepoMismatch, strings.TrimSpace(origin), e.cfg.RepoURL) }` — names both URLs and returns before the fetch/checkout/pull block. normalizeRepoURL does TrimSpace + TrimSuffix("/") + TrimSuffix(".git") + TrimSuffix("/"). TestNormalizeRepoURL pins whitespace/trailing-slash/.git cases and strictness.
  ✓ The import handler maps that error to HTTP 409 with error source_repo_mismatch (never 200 with skipped>=1); a handler test asserts 409 for a mismatching source_repo and an export test asserts the mismatch error with no fetch: internal/api/handlers.go:873 `if errors.Is(err, importgit.ErrRepoMismatch) { writeError(w, http.StatusConflict, "source_repo_mismatch", err.Error()); return }` — placed before the 200 writeJSON path. TestImportSourceRepoMismatch_Conflict (handlers_test.go) asserts status==409, body error=="source_repo_mismatch", and message names both repoA and bogusRepo. TestExport_ExistingCloneMismatchedRepo_Errors asserts errors.Is(err, ErrRepoMismatch) and no fetch (refs/remotes/origin/main unchanged after remote advanced). Both PASS.
  ✓ go build ./... and gofmt -l cmd internal pkg sql are clean; go test ./... -short passes; new tests cover mismatch-rejected AND matching-origin-reuse-still-imports: `go build ./...` exit 0; `gofmt -l cmd internal pkg sql` produced no output (exit 0); `go test ./... -short -count=1` all packages ok (api, export, import, etc.). Targeted run: TestImport_ExistingCloneMismatchedRepo_Errors PASS, TestImport_ExistingCloneMismatchedRepo_NoFetch PASS, TestImport_ExistingCloneMatchingRepo_Proceeds PASS (matching-origin reuse still imports), TestExport_ExistingCloneMismatchedRepo_Errors PASS, TestImportSourceRepoMismatch_Conflict PASS — none skipped.
All three criteria verified: prepareClone origin guard with URL normalization and both-URL error, handler 409 source_repo_mismatch mapping, and clean build/gofmt/tests with mismatch-rejected and matching-reuse coverage.

Overall: PASS ✓
