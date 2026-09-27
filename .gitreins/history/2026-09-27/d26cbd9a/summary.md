# Verdict: DF-OFF-BY-ONE-23

**Task:** Fix extractSection embedded horizontal rule parsing
**Evaluated:** 2026-09-27T17:10:28.510857
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	7.580s
- ✓ **tier2**
  - COMPLETE
  ✓ extractSection must preserve solution text containing an embedded horizontal-rule line and only stop at the actual next section; add a regression test in internal/import/git_test.go and keep existing section parsing behavior green: internal/import/git.go:561-580: extractSection now cuts only at real headings ("\n## ", "\n# ") instead of the old "\n---\n" boundary (git diff HEAD~1 confirms removal of "\n---\n" from the nextMarker list), so embedded horizontal-rule lines are retained. Regression test TestExtractSection_EmbeddedHorizontalRule at internal/import/git_test.go:592-608 asserts the '---' line and 'Appended content after the rule.' are retained while the following '## Notes' section is excluded. Verified genuine regression: reverting the fix makes the test FAIL ('extractSection = "First part of the solution.", want to retain embedded horizontal rule ---'), and it PASSES with the fix. Existing behavior green: `go test ./internal/import/ -short -count=1` => 'ok github.com/totalwindupflightsystems/off-by-one/internal/import 6.983s' EXIT=0; TestExtractSection/TestExtractSection_NotFound/TestParseSolutionMD/TestParseEvidenceMD all PASS; `go build ./...` EXIT=0; `go vet ./internal/import/` EXIT=0; `gofmt -l internal/import/` empty; LSP diagnostics empty. Additional edge cases (HR immediately after marker, HR at section end with no following heading, H1 boundary) all behave correctly. [resolution 0.12; internal/import/git_test.go]
extractSection now preserves embedded horizontal rules and stops only at real next headings, with a regression test that fails pre-fix and passes post-fix while all existing section-parsing tests remain green.

## Summary

Judge Result: DF-OFF-BY-ONE-23

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	7.580s

Stage tier2: PASS
  COMPLETE
  ✓ extractSection must preserve solution text containing an embedded horizontal-rule line and only stop at the actual next section; add a regression test in internal/import/git_test.go and keep existing section parsing behavior green: internal/import/git.go:561-580: extractSection now cuts only at real headings ("\n## ", "\n# ") instead of the old "\n---\n" boundary (git diff HEAD~1 confirms removal of "\n---\n" from the nextMarker list), so embedded horizontal-rule lines are retained. Regression test TestExtractSection_EmbeddedHorizontalRule at internal/import/git_test.go:592-608 asserts the '---' line and 'Appended content after the rule.' are retained while the following '## Notes' section is excluded. Verified genuine regression: reverting the fix makes the test FAIL ('extractSection = "First part of the solution.", want to retain embedded horizontal rule ---'), and it PASSES with the fix. Existing behavior green: `go test ./internal/import/ -short -count=1` => 'ok github.com/totalwindupflightsystems/off-by-one/internal/import 6.983s' EXIT=0; TestExtractSection/TestExtractSection_NotFound/TestParseSolutionMD/TestParseEvidenceMD all PASS; `go build ./...` EXIT=0; `go vet ./internal/import/` EXIT=0; `gofmt -l internal/import/` empty; LSP diagnostics empty. Additional edge cases (HR immediately after marker, HR at section end with no following heading, H1 boundary) all behave correctly. [resolution 0.12; internal/import/git_test.go]
extractSection now preserves embedded horizontal rules and stops only at real next headings, with a regression test that fails pre-fix and passes post-fix while all existing section-parsing tests remain green.

Overall: PASS ✓
