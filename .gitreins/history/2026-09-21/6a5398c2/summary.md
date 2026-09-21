# Verdict: OB-GAP-082

**Task:** openapi.yaml: declare documented error codes for export/import/problems list
**Evaluated:** 2026-09-21T01:24:57.667947
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ pkg/api/openapi.yaml carries the documented response objects ('400','501','500' for export/import; '500' for GET /api/v1/problems) with the Error schema; curl /openapi.json shows them; api-reference.md status lines and the spec agree per-endpoint: pkg/api/openapi.yaml: /api/v1/export post responses = 200,400,501,500 (lines 333-367); /api/v1/import post = 200,400,501,500 (lines 370-404); /api/v1/problems get = 200,500 (lines 106-160); every non-2xx has content.application/json.schema.$ref = '#/components/schemas/Error'. Live check: `curl -s http://127.0.0.1:8766/openapi.json` -> HTTP 200, parsed JSON shows export post ['200','400','500','501'], import post ['200','400','500','501'], problems get ['200','500'], each non-2xx with $ref '#/components/schemas/Error'. docs/api-reference.md agrees per-endpoint: GET /api/v1/problems '**Status codes:** `200`, `500`.' (line 157); POST /api/v1/export '`200`, `400` (missing required fields), `501` (export not configured), `500` (export failed).' (line 475); POST /api/v1/import '`200`, `400` (missing required fields), `501` (import not configured), `500` (import failed).' (line 518). Handler codes match: internal/api/handlers.go:884 501 / :889,:893,:897 400 / :946 500 (export); :961 501 / :966,:970 400 / :991 500 (import); :442,:447,:467,:472 500 (handleListProblems). Tests: `go test ./pkg/api/... -run TestOpenAPISpec -count=1 -v` -> all PASS incl. TestOpenAPISpec_ErrorResponsesDocumented; `go test ./pkg/api/... -run TestDocs_StatusCodesMatchSpec -count=1 -v` -> PASS; `go test ./pkg/api/... ./internal/api/... -count=1 -p 1` -> ok both pkgs; `go build ./... && go vet ./... && gofmt -l cmd/ internal/ pkg/ sql/` -> GOFMT_CLEAN (empty output).
openapi.yaml declares 400/501/500 for export/import and 500 for GET /api/v1/problems with the Error schema, the live /openapi.json serves them, and api-reference.md status lines match the spec per-endpoint (all spec/docs tests pass).

## Summary

Judge Result: OB-GAP-082

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ pkg/api/openapi.yaml carries the documented response objects ('400','501','500' for export/import; '500' for GET /api/v1/problems) with the Error schema; curl /openapi.json shows them; api-reference.md status lines and the spec agree per-endpoint: pkg/api/openapi.yaml: /api/v1/export post responses = 200,400,501,500 (lines 333-367); /api/v1/import post = 200,400,501,500 (lines 370-404); /api/v1/problems get = 200,500 (lines 106-160); every non-2xx has content.application/json.schema.$ref = '#/components/schemas/Error'. Live check: `curl -s http://127.0.0.1:8766/openapi.json` -> HTTP 200, parsed JSON shows export post ['200','400','500','501'], import post ['200','400','500','501'], problems get ['200','500'], each non-2xx with $ref '#/components/schemas/Error'. docs/api-reference.md agrees per-endpoint: GET /api/v1/problems '**Status codes:** `200`, `500`.' (line 157); POST /api/v1/export '`200`, `400` (missing required fields), `501` (export not configured), `500` (export failed).' (line 475); POST /api/v1/import '`200`, `400` (missing required fields), `501` (import not configured), `500` (import failed).' (line 518). Handler codes match: internal/api/handlers.go:884 501 / :889,:893,:897 400 / :946 500 (export); :961 501 / :966,:970 400 / :991 500 (import); :442,:447,:467,:472 500 (handleListProblems). Tests: `go test ./pkg/api/... -run TestOpenAPISpec -count=1 -v` -> all PASS incl. TestOpenAPISpec_ErrorResponsesDocumented; `go test ./pkg/api/... -run TestDocs_StatusCodesMatchSpec -count=1 -v` -> PASS; `go test ./pkg/api/... ./internal/api/... -count=1 -p 1` -> ok both pkgs; `go build ./... && go vet ./... && gofmt -l cmd/ internal/ pkg/ sql/` -> GOFMT_CLEAN (empty output).
openapi.yaml declares 400/501/500 for export/import and 500 for GET /api/v1/problems with the Error schema, the live /openapi.json serves them, and api-reference.md status lines match the spec per-endpoint (all spec/docs tests pass).

Overall: PASS ✓
