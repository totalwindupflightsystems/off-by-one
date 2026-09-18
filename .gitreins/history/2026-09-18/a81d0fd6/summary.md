# Verdict: OB-GAP-069

**Task:** Document solver_unavailable submit behavior
**Evaluated:** 2026-09-18T14:13:47.573694
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ docs/api-reference.md lists HTTP 503 solver_unavailable for submit; docs/integration.md no longer claims rejected submissions stay queued; documented statuses match internal/api/handlers.go: docs/api-reference.md:65 lists `503` (`solver_unavailable`) in the submit status set, and lines 88-96 document the exact 503 body {"error":"solver_unavailable","message":"solver is not available (bwrap + pi-agent not configured); submissions cannot be queued — use POST /api/v1/problems/discover to look up existing answers"} which matches internal/api/handlers.go:206-210 writeError(w, http.StatusServiceUnavailable, "solver_unavailable", ...) verbatim. docs/integration.md:315 now states submit does NOT queue anything when solver_available is false (rejects up front with HTTP 503 / solver_unavailable, no submission id, no queue entry); grep -niE 'stay in the queue|remain queued|until the server is restarted|stay queued' docs/integration.md returns 0 matches (exit=1), so the stale claim is gone. Documented status set 200/400/409/500/503 matches handlers.go exactly: 400 invalid_request (lines 221/228/232/255/261/304), 409 duplicate via ingest.StatusForHTTP (submit.go:35 StatusConflict), 500 internal_error (line 307), 503 solver_unavailable (line 207), 200 writeJSON success (line 300). Commit 986f7f7 is docs-only (docs/api-reference.md + docs/integration.md, no Go code touched); go build ./... exits 0.
Docs-only change correctly documents HTTP 503 solver_unavailable for submit, removes the stale queue-until-restart claim from integration.md, and the documented status set matches internal/api/handlers.go.

## Summary

Judge Result: OB-GAP-069

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ docs/api-reference.md lists HTTP 503 solver_unavailable for submit; docs/integration.md no longer claims rejected submissions stay queued; documented statuses match internal/api/handlers.go: docs/api-reference.md:65 lists `503` (`solver_unavailable`) in the submit status set, and lines 88-96 document the exact 503 body {"error":"solver_unavailable","message":"solver is not available (bwrap + pi-agent not configured); submissions cannot be queued — use POST /api/v1/problems/discover to look up existing answers"} which matches internal/api/handlers.go:206-210 writeError(w, http.StatusServiceUnavailable, "solver_unavailable", ...) verbatim. docs/integration.md:315 now states submit does NOT queue anything when solver_available is false (rejects up front with HTTP 503 / solver_unavailable, no submission id, no queue entry); grep -niE 'stay in the queue|remain queued|until the server is restarted|stay queued' docs/integration.md returns 0 matches (exit=1), so the stale claim is gone. Documented status set 200/400/409/500/503 matches handlers.go exactly: 400 invalid_request (lines 221/228/232/255/261/304), 409 duplicate via ingest.StatusForHTTP (submit.go:35 StatusConflict), 500 internal_error (line 307), 503 solver_unavailable (line 207), 200 writeJSON success (line 300). Commit 986f7f7 is docs-only (docs/api-reference.md + docs/integration.md, no Go code touched); go build ./... exits 0.
Docs-only change correctly documents HTTP 503 solver_unavailable for submit, removes the stale queue-until-restart claim from integration.md, and the documented status set matches internal/api/handlers.go.

Overall: PASS ✓
