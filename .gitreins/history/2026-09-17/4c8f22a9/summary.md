# Verdict: DF-OFF-BY-ONE-9

**Task:** docs truth pass: stats failed-signature semantics, export 501 precondition, env/lang exact-match filter, pi-agent-watchdog documented (DF-9 + DOC-1 + DOC-2)
**Evaluated:** 2026-09-17T19:50:30.057977
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.433s
- ✓ **tier2**
  - COMPLETE
  ✓ README.md and docs/api-reference.md state that GET /api/v1/stats verified_answers/hit_rate exclude answers whose signatures JSON result is failed (no 100% verified / hit_rate 1.0 claim, sample JSON not equal-numbers); docs/integration.md carries the -export-dir / 501 not_configured precondition next to the export example; docs/api-reference.md env/lang filters state exact-match against the stored value; README.md scripts layout documents scripts/pi-agent-watchdog.sh per its header.: All four sub-claims verified. (1) README.md:113 and README.md:417 plus docs/api-reference.md:564 state verified_answers counts only status verified/ci_passed rows whose signatures JSON does not record result:"failed" (predicate COALESCE(json_extract(signatures,'$.result'),'')!='failed', matching internal/graph/signature.go:19 and internal/ingest/queue.go:340); grep for '100% verified'/'hit rate 1.0'/"hit_rate": 1, returns no stale claim; sample JSON is no longer equal-numbers — total_answers 2036 vs verified_answers 2008, hit_rate 0.9862475442043221 (README.md:417, docs/api-reference.md:554, docs/integration.md:216). (2) docs/integration.md:236 places the precondition immediately above the export curl example: server must be started with -export-dir/OFF_BY_ONE_EXPORT_DIR, otherwise 501 {"error":"not_configured","message":"export directory not configured"} — matches internal/api/handlers.go:858. (3) docs/api-reference.md:115-117 state env/lang are matched exactly against the stored value (a.env = ?, a.lang = ?), matching internal/graph/search.go:100-101. (4) README.md:384 documents scripts/pi-agent-watchdog.sh per its header (scripts/pi-agent-watchdog.sh:1-40): probes WRAPPER RESOLUTION (packages/coding-agent/dist/cli.js, package.json, non-empty node_modules/.bin, executable wrapper), silent when healthy, one ALERT per incident with stamp dedup and rebuild recipe, cron ~15 min. Docs-only change (git diff HEAD~1: README.md, docs/api-reference.md, docs/integration.md); no test suite applies.


## Summary

Judge Result: DF-OFF-BY-ONE-9

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.433s

Stage tier2: PASS
  COMPLETE
  ✓ README.md and docs/api-reference.md state that GET /api/v1/stats verified_answers/hit_rate exclude answers whose signatures JSON result is failed (no 100% verified / hit_rate 1.0 claim, sample JSON not equal-numbers); docs/integration.md carries the -export-dir / 501 not_configured precondition next to the export example; docs/api-reference.md env/lang filters state exact-match against the stored value; README.md scripts layout documents scripts/pi-agent-watchdog.sh per its header.: All four sub-claims verified. (1) README.md:113 and README.md:417 plus docs/api-reference.md:564 state verified_answers counts only status verified/ci_passed rows whose signatures JSON does not record result:"failed" (predicate COALESCE(json_extract(signatures,'$.result'),'')!='failed', matching internal/graph/signature.go:19 and internal/ingest/queue.go:340); grep for '100% verified'/'hit rate 1.0'/"hit_rate": 1, returns no stale claim; sample JSON is no longer equal-numbers — total_answers 2036 vs verified_answers 2008, hit_rate 0.9862475442043221 (README.md:417, docs/api-reference.md:554, docs/integration.md:216). (2) docs/integration.md:236 places the precondition immediately above the export curl example: server must be started with -export-dir/OFF_BY_ONE_EXPORT_DIR, otherwise 501 {"error":"not_configured","message":"export directory not configured"} — matches internal/api/handlers.go:858. (3) docs/api-reference.md:115-117 state env/lang are matched exactly against the stored value (a.env = ?, a.lang = ?), matching internal/graph/search.go:100-101. (4) README.md:384 documents scripts/pi-agent-watchdog.sh per its header (scripts/pi-agent-watchdog.sh:1-40): probes WRAPPER RESOLUTION (packages/coding-agent/dist/cli.js, package.json, non-empty node_modules/.bin, executable wrapper), silent when healthy, one ALERT per incident with stamp dedup and rebuild recipe, cron ~15 min. Docs-only change (git diff HEAD~1: README.md, docs/api-reference.md, docs/integration.md); no test suite applies.


Overall: PASS ✓
