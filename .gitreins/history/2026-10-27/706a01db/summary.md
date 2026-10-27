# Verdict: DOC-4

**Task:** Document GET metrics API contract
**Evaluated:** 2026-10-27T16:01:11.999631
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✗ **tier2**
  - INCOMPLETE
  ✗ docs/api-reference.md documents GET metrics response fields, status codes, curl example, and live probe: Partial. docs/api-reference.md:694-739 (commit b4487beb) adds a `GET /metrics` section with: a JSON response sample and a field table (solve_count, skipped_host_pressure, discovery_hits, discovery_misses, over_budget, rejected, peak_rss_bytes_max, alert_threshold_bytes) matching internal/metrics/metrics.go:218-251; status codes `200` and `404 metrics_disabled` matching internal/api/server.go:313-320 handleMetrics; and a curl example at line 738 (`curl -sS http://localhost:8766/metrics`). However the required **live probe** is absent: `grep -i probe docs/api-reference.md` returns only unrelated lines (430 placeholder classes, 844/847 check-deploy probe), and there is no observed/live response captured for /metrics — unlike other endpoints in the same doc which do record live observations (line 623 for GET /api/v1/stats, line 160 for the problems filter). A live probe of the running lab (`curl -sS http://localhost:8766/metrics` → HTTP 200, body `{"solve_count":0,"skipped_host_pressure":115,"discovery_hits":5,"discovery_misses":38,"over_budget":0,"rejected":0,"peak_rss_bytes_max":0,"alert_threshold_bytes":4294967296,"last_host":{...},"last_skip_reason":"load"}`) also shows the documented sample omits the optional `last_solve`/`last_host`/`last_skip_reason` fields that the live endpoint actually returns. No test suite applies to this docs-only criterion (no test command covers markdown content).
The GET /metrics section documents response fields, status codes, and a curl example, but contains no live probe (no observed response from a running instance), so the criterion is only partially met.

## Summary

Judge Result: DOC-4

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: FAIL
  INCOMPLETE
  ✗ docs/api-reference.md documents GET metrics response fields, status codes, curl example, and live probe: Partial. docs/api-reference.md:694-739 (commit b4487beb) adds a `GET /metrics` section with: a JSON response sample and a field table (solve_count, skipped_host_pressure, discovery_hits, discovery_misses, over_budget, rejected, peak_rss_bytes_max, alert_threshold_bytes) matching internal/metrics/metrics.go:218-251; status codes `200` and `404 metrics_disabled` matching internal/api/server.go:313-320 handleMetrics; and a curl example at line 738 (`curl -sS http://localhost:8766/metrics`). However the required **live probe** is absent: `grep -i probe docs/api-reference.md` returns only unrelated lines (430 placeholder classes, 844/847 check-deploy probe), and there is no observed/live response captured for /metrics — unlike other endpoints in the same doc which do record live observations (line 623 for GET /api/v1/stats, line 160 for the problems filter). A live probe of the running lab (`curl -sS http://localhost:8766/metrics` → HTTP 200, body `{"solve_count":0,"skipped_host_pressure":115,"discovery_hits":5,"discovery_misses":38,"over_budget":0,"rejected":0,"peak_rss_bytes_max":0,"alert_threshold_bytes":4294967296,"last_host":{...},"last_skip_reason":"load"}`) also shows the documented sample omits the optional `last_solve`/`last_host`/`last_skip_reason` fields that the live endpoint actually returns. No test suite applies to this docs-only criterion (no test command covers markdown content).
The GET /metrics section documents response fields, status codes, and a curl example, but contains no live probe (no observed response from a running instance), so the criterion is only partially met.

Overall: FAIL ✗
