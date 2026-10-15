# Verdict: REVIEW-OB-014

**Task:** Import to non-empty --import-dir gives bare git error
**Evaluated:** 2026-10-15T11:20:01.324979
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ internal/import precondition check returns a clear error distinguishing dirty destination dir from unreachable repo, with remediation message, plus a regression test: internal/import/git.go:169 defines typed sentinel ErrDestNotEmpty; prepareClone (git.go:238) precondition at git.go:271-278 does os.ReadDir(LocalDir) and when err==nil && len(entries)>0 returns fmt.Errorf("%w: %q — point the import directory at an empty or new directory, or remove it", ErrDestNotEmpty, e.cfg.LocalDir) — refuses BEFORE invoking git, names the path and the remediation. Distinction from unreachable repo is real: unreachable repo surfaces as "clone %s: %w" (git.go:287) → 500 import_failed, while ErrDestNotEmpty maps to 409 import_dest_not_empty (internal/api/handlers.go:1100). Regression tests: internal/import/git_test.go:700 TestImport_NonEmptyDestDir_Errors (asserts errors.Is(err, ErrDestNotEmpty), message contains path and "empty", sentinel file untouched, graph untouched) plus control TestImport_EmptyDestDir_Clones (git_test.go:749), and HTTP-level internal/api/handlers_test.go:2831 TestImportNonEmptyDestDir_Conflict (409 import_dest_not_empty). Test evidence: `go test ./internal/import/ -run 'TestImport_NonEmptyDestDir_Errors|TestImport_EmptyDestDir_Clones' -count=1 -v` → '--- PASS: TestImport_NonEmptyDestDir_Errors (0.20s)', '--- PASS: TestImport_EmptyDestDir_Clones (0.43s)', 'ok github.com/totalwindupflightsystems/off-by-one/internal/import 0.643s'; `go test ./internal/api/ -run Import -count=1 -v` → '--- PASS: TestImportNonEmptyDestDir_Conflict (0.18s)'; full `go test ./... -short -count=1 -p 1 -timeout 180s` → all packages ok; `go build ./...` exit 0; `go vet ./internal/import/ ./internal/api/` exit 0.
The import precondition check returns a typed, path-naming, remediation-bearing ErrDestNotEmpty that is clearly distinguished from unreachable-repo failures (409 vs 500), with passing regression tests at both the engine and HTTP layers.

## Summary

Judge Result: REVIEW-OB-014

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ internal/import precondition check returns a clear error distinguishing dirty destination dir from unreachable repo, with remediation message, plus a regression test: internal/import/git.go:169 defines typed sentinel ErrDestNotEmpty; prepareClone (git.go:238) precondition at git.go:271-278 does os.ReadDir(LocalDir) and when err==nil && len(entries)>0 returns fmt.Errorf("%w: %q — point the import directory at an empty or new directory, or remove it", ErrDestNotEmpty, e.cfg.LocalDir) — refuses BEFORE invoking git, names the path and the remediation. Distinction from unreachable repo is real: unreachable repo surfaces as "clone %s: %w" (git.go:287) → 500 import_failed, while ErrDestNotEmpty maps to 409 import_dest_not_empty (internal/api/handlers.go:1100). Regression tests: internal/import/git_test.go:700 TestImport_NonEmptyDestDir_Errors (asserts errors.Is(err, ErrDestNotEmpty), message contains path and "empty", sentinel file untouched, graph untouched) plus control TestImport_EmptyDestDir_Clones (git_test.go:749), and HTTP-level internal/api/handlers_test.go:2831 TestImportNonEmptyDestDir_Conflict (409 import_dest_not_empty). Test evidence: `go test ./internal/import/ -run 'TestImport_NonEmptyDestDir_Errors|TestImport_EmptyDestDir_Clones' -count=1 -v` → '--- PASS: TestImport_NonEmptyDestDir_Errors (0.20s)', '--- PASS: TestImport_EmptyDestDir_Clones (0.43s)', 'ok github.com/totalwindupflightsystems/off-by-one/internal/import 0.643s'; `go test ./internal/api/ -run Import -count=1 -v` → '--- PASS: TestImportNonEmptyDestDir_Conflict (0.18s)'; full `go test ./... -short -count=1 -p 1 -timeout 180s` → all packages ok; `go build ./...` exit 0; `go vet ./internal/import/ ./internal/api/` exit 0.
The import precondition check returns a typed, path-naming, remediation-bearing ErrDestNotEmpty that is clearly distinguished from unreachable-repo failures (409 vs 500), with passing regression tests at both the engine and HTTP layers.

Overall: PASS ✓
