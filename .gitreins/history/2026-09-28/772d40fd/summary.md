# Verdict: REVIEW-OB-008

**Task:** submit response emits related_problems null
**Evaluated:** 2026-09-28T16:07:40.754892
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ POST submit responses serialize related_problems as [] rather than null for both queued and deduplicated responses, with regression coverage and all tests passing: internal/api/handlers.go:55 declares RelatedProblems []string `json:"related_problems"` (no omitempty); nonNilStrings (handlers.go:888-893) converts nil to []string{}. Both construction sites normalize: deduplicated path handlers.go:322 `resp.RelatedProblems = nonNilStrings(s.relatedFor(r, slug))` and queued path handlers.go:346 `RelatedProblems: nonNilStrings(s.relatedFor(r, slug))`. relatedFor (handlers.go:897-908) returns nil on error/no edges, so normalization is load-bearing. Regression coverage in internal/api/handlers_test.go: assertRelatedProblemsArray (:247) asserts the key is present and a JSON array; TestSubmit_Queued_RelatedProblemsEmptyArray (:268) and TestSubmit_Dedup_RelatedProblemsEmptyArray (:285, 409 path) lock the empty-array contract; TestSubmit_RelatedProblemsPopulated (:307) guards non-empty behavior. Mutation proof: reverting nonNilStrings at both sites makes `go test ./internal/api/ -run RelatedProblems -count=1 -v` FAIL with body `"related_problems":null` on BOTH queued and deduplicated paths, proving the tests genuinely catch the bug; fix restored (git diff --stat internal/api/handlers.go empty). Fresh runs: `go test ./internal/api/ -run TestSubmit -count=1 -v` => PASS, ok internal/api 0.045s (all 18 TestSubmit* incl. the 3 related_problems tests); `go test ./... -short -count=1 -p 1 -timeout 180s` => exit_code 0, all 14 packages ok (cmd/off-by-one, internal/api, cron, export, graph, import, ingest, muster, sandbox, seed, solver, tools, web, pkg/api).
Both queued and deduplicated submit responses normalize related_problems to [] via nonNilStrings, with regression tests that provably fail on the null bug, and the full suite passes.

## Summary

Judge Result: REVIEW-OB-008

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ POST submit responses serialize related_problems as [] rather than null for both queued and deduplicated responses, with regression coverage and all tests passing: internal/api/handlers.go:55 declares RelatedProblems []string `json:"related_problems"` (no omitempty); nonNilStrings (handlers.go:888-893) converts nil to []string{}. Both construction sites normalize: deduplicated path handlers.go:322 `resp.RelatedProblems = nonNilStrings(s.relatedFor(r, slug))` and queued path handlers.go:346 `RelatedProblems: nonNilStrings(s.relatedFor(r, slug))`. relatedFor (handlers.go:897-908) returns nil on error/no edges, so normalization is load-bearing. Regression coverage in internal/api/handlers_test.go: assertRelatedProblemsArray (:247) asserts the key is present and a JSON array; TestSubmit_Queued_RelatedProblemsEmptyArray (:268) and TestSubmit_Dedup_RelatedProblemsEmptyArray (:285, 409 path) lock the empty-array contract; TestSubmit_RelatedProblemsPopulated (:307) guards non-empty behavior. Mutation proof: reverting nonNilStrings at both sites makes `go test ./internal/api/ -run RelatedProblems -count=1 -v` FAIL with body `"related_problems":null` on BOTH queued and deduplicated paths, proving the tests genuinely catch the bug; fix restored (git diff --stat internal/api/handlers.go empty). Fresh runs: `go test ./internal/api/ -run TestSubmit -count=1 -v` => PASS, ok internal/api 0.045s (all 18 TestSubmit* incl. the 3 related_problems tests); `go test ./... -short -count=1 -p 1 -timeout 180s` => exit_code 0, all 14 packages ok (cmd/off-by-one, internal/api, cron, export, graph, import, ingest, muster, sandbox, seed, solver, tools, web, pkg/api).
Both queued and deduplicated submit responses normalize related_problems to [] via nonNilStrings, with regression tests that provably fail on the null bug, and the full suite passes.

Overall: PASS ✓
