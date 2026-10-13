# Verdict: DF-OFF-BY-ONE-43

**Task:** Submit view renders 409 deduplicated as failure
**Evaluated:** 2026-10-13T06:49:24.374319
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ web/js/submit.js treats 409+deduplicated as success showing submission_id and position; failure branch preserved; Go-convention test covers it; go suite passes: web/js/submit.js:419-422 computes `deduplicated = res.status === 409 && res.data && res.data.status === 'deduplicated'` and the failure branch is preserved as `if (!res.ok && !deduplicated) { ... renderError(area, msg); return; }`. Success renderer shows position at :458-462 ('Already queued (' + d.submission_id + '), position ' + d.position) and submission_id at :493-495 ('Submission ID: ' + d.submission_id). API side confirms 409: internal/ingest/submit.go:34-35 maps ErrDuplicate -> http.StatusConflict, and internal/api/handlers.go:320 emits Status:"deduplicated" with SubmissionID/Position. Go-convention test internal/web/submit_dedup_test.go TestHandler_SubmitJSDedup409IsSuccess asserts the dedup detection precedes the failure branch and that submission_id/position markers exist. Ran `go test ./internal/web/... -run TestHandler_SubmitJSDedup409IsSuccess -count=1 -v` => '--- PASS: TestHandler_SubmitJSDedup409IsSuccess (0.00s)' / 'ok ... internal/web 0.006s' (exit 0). Full suite `go test ./... -short -count=1 -p 1 -timeout 180s` => all packages ok (internal/web ok 2.526s, internal/api ok 2.408s), exit 0. [resolution 0.46; web/js/submit.js]
409+deduplicated is rendered as success with submission_id and queue position, the failure branch is intact, the Go test covers it, and the full go suite passes.

## Summary

Judge Result: DF-OFF-BY-ONE-43

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ web/js/submit.js treats 409+deduplicated as success showing submission_id and position; failure branch preserved; Go-convention test covers it; go suite passes: web/js/submit.js:419-422 computes `deduplicated = res.status === 409 && res.data && res.data.status === 'deduplicated'` and the failure branch is preserved as `if (!res.ok && !deduplicated) { ... renderError(area, msg); return; }`. Success renderer shows position at :458-462 ('Already queued (' + d.submission_id + '), position ' + d.position) and submission_id at :493-495 ('Submission ID: ' + d.submission_id). API side confirms 409: internal/ingest/submit.go:34-35 maps ErrDuplicate -> http.StatusConflict, and internal/api/handlers.go:320 emits Status:"deduplicated" with SubmissionID/Position. Go-convention test internal/web/submit_dedup_test.go TestHandler_SubmitJSDedup409IsSuccess asserts the dedup detection precedes the failure branch and that submission_id/position markers exist. Ran `go test ./internal/web/... -run TestHandler_SubmitJSDedup409IsSuccess -count=1 -v` => '--- PASS: TestHandler_SubmitJSDedup409IsSuccess (0.00s)' / 'ok ... internal/web 0.006s' (exit 0). Full suite `go test ./... -short -count=1 -p 1 -timeout 180s` => all packages ok (internal/web ok 2.526s, internal/api ok 2.408s), exit 0. [resolution 0.46; web/js/submit.js]
409+deduplicated is rendered as success with submission_id and queue position, the failure branch is intact, the Go test covers it, and the full go suite passes.

Overall: PASS ✓
