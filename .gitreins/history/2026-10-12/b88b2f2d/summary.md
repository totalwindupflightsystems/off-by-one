# Verdict: DF-OFF-BY-ONE-40

**Task:** Reserve or remove dead export pr_url contract
**Evaluated:** 2026-10-12T19:45:05.962350
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ Either assign exportResponse.PRURL from a real pull-request implementation, or amend docs/api-reference.md and pkg/api/openapi.yaml to state pr_url is reserved and never currently populated; add/update focused documentation/spec tests or exact grep evidence; go build ./..., go vet ./..., go test ./... -short -p 1 -count=1 pass.: Reserved branch satisfied (commit 336c3e11). (1) No real PR implementation exists, so 'reserved' is correct: `grep -rn 'PRURL\s*[:=]' --include='*.go' .` -> exit 1, ZERO assignment sites; `grep -rniE 'pull.?request|createPR|/pulls' internal/export/` -> no matches. (2) Docs/spec amended: docs/api-reference.md:473 states pr_url is a **reserved** field and **never populated**, with the old misleading sentence 'is omitted when the repo does not support pull requests' removed; docs/integration.md:275 states reserved/never populated; pkg/api/openapi.yaml:733-735 description = 'Reserved for future pull-request creation and never populated by the current implementation... always omitted.'; internal/api/handlers.go:181-186 carries the reserved doc comment with `json:"pr_url,omitempty"`. (3) Focused tests added: pkg/api/export_pr_url_test.go (new, 95 lines) — TestOpenAPISpec_ExportResponsePRURLIsReserved reads the SERVED spec via JSONBytes() (go:embed openapi.yaml, pkg/api/spec.go:17) and asserts type=string, description contains 'reserved' AND 'never', and pr_url not in required; TestDocs_ExportPRURLReservedWording scopes to the export section with a negative control on the old wording. Served-spec probe confirmed the embedded spec really serves the reserved description. Negative control proves the test is genuine: reverting the docs sentence made it FAIL at export_pr_url_test.go:83/90/93; file restored, git diff empty. (4) Gates, actual output: `go build ./...` -> BUILD_EXIT=0; `go vet ./...` -> VET_EXIT=0; `go test ./... -short -p 1 -count=1` -> REAL_TEST_EXIT=0, 15 packages 'ok', zero FAIL lines; `go test ./pkg/api/ -run PRURL -v -count=1` -> PASS TestOpenAPISpec_ExportResponsePRURLIsReserved, PASS TestDocs_ExportPRURLReservedWording, ok pkg/api 0.009s. Pre-screen's 'missing: test' at 0.29 is contradicted by this evidence. [resolution 0.29; docs/api-reference.md, pkg/api/openapi.yaml]
The dead pr_url contract is correctly resolved via the reserved branch: docs/api-reference.md, docs/integration.md and the embedded pkg/api/openapi.yaml all state pr_url is reserved and never populated, two focused regression tests pin that wording (verified non-vacuous by a negative control), and go build/vet/test -short -p 1 -count=1 all pass with exit 0.

## Summary

Judge Result: DF-OFF-BY-ONE-40

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ Either assign exportResponse.PRURL from a real pull-request implementation, or amend docs/api-reference.md and pkg/api/openapi.yaml to state pr_url is reserved and never currently populated; add/update focused documentation/spec tests or exact grep evidence; go build ./..., go vet ./..., go test ./... -short -p 1 -count=1 pass.: Reserved branch satisfied (commit 336c3e11). (1) No real PR implementation exists, so 'reserved' is correct: `grep -rn 'PRURL\s*[:=]' --include='*.go' .` -> exit 1, ZERO assignment sites; `grep -rniE 'pull.?request|createPR|/pulls' internal/export/` -> no matches. (2) Docs/spec amended: docs/api-reference.md:473 states pr_url is a **reserved** field and **never populated**, with the old misleading sentence 'is omitted when the repo does not support pull requests' removed; docs/integration.md:275 states reserved/never populated; pkg/api/openapi.yaml:733-735 description = 'Reserved for future pull-request creation and never populated by the current implementation... always omitted.'; internal/api/handlers.go:181-186 carries the reserved doc comment with `json:"pr_url,omitempty"`. (3) Focused tests added: pkg/api/export_pr_url_test.go (new, 95 lines) — TestOpenAPISpec_ExportResponsePRURLIsReserved reads the SERVED spec via JSONBytes() (go:embed openapi.yaml, pkg/api/spec.go:17) and asserts type=string, description contains 'reserved' AND 'never', and pr_url not in required; TestDocs_ExportPRURLReservedWording scopes to the export section with a negative control on the old wording. Served-spec probe confirmed the embedded spec really serves the reserved description. Negative control proves the test is genuine: reverting the docs sentence made it FAIL at export_pr_url_test.go:83/90/93; file restored, git diff empty. (4) Gates, actual output: `go build ./...` -> BUILD_EXIT=0; `go vet ./...` -> VET_EXIT=0; `go test ./... -short -p 1 -count=1` -> REAL_TEST_EXIT=0, 15 packages 'ok', zero FAIL lines; `go test ./pkg/api/ -run PRURL -v -count=1` -> PASS TestOpenAPISpec_ExportResponsePRURLIsReserved, PASS TestDocs_ExportPRURLReservedWording, ok pkg/api 0.009s. Pre-screen's 'missing: test' at 0.29 is contradicted by this evidence. [resolution 0.29; docs/api-reference.md, pkg/api/openapi.yaml]
The dead pr_url contract is correctly resolved via the reserved branch: docs/api-reference.md, docs/integration.md and the embedded pkg/api/openapi.yaml all state pr_url is reserved and never populated, two focused regression tests pin that wording (verified non-vacuous by a negative control), and go build/vet/test -short -p 1 -count=1 all pass with exit 0.

Overall: PASS ✓
