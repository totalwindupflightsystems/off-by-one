# Verdict: REVIEW-OB-013

**Task:** README quickstart submit example omits required cadence field
**Evaluated:** 2026-10-15T11:21:59.983222
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README submit example includes cadence field and matches live wire format (submit without cadence 400s; docs example succeeds): README.md:176-190 'Example: Submit a Problem' curl block includes `"cadence": "post-debug"` at line 188, and its field names (problem_class, environment, language, version, description, error_message, cadence) match internal/api/handlers.go:30-39 submitProblemRequest exactly; README.md:204 documents cadence as Required=Yes with values pre-phase/end-of-day/post-debug. LIVE WIRE VERIFICATION: built cmd/off-by-one, ran `serve` on :8801, and POSTed via a Go http client — the README example verbatim returned HTTP 200 {"submission_id":"sub_b88aea","problem_class":"so-nil-pointer-deref","status":"queued","position":1,...}, while the identical body with cadence omitted returned HTTP 400 {"error":"invalid_request","message":"ingest: invalid cadence (accepted: pre-phase, end-of-day, post-debug): got \"\""}. Code path confirmed: internal/ingest/queue.go:322-334 validate() rejects empty/unknown cadence with ErrInvalidCadence, mapped to 400 at internal/api/handlers.go:357-359. Tests: `go test ./internal/api/ -run 'TestSubmit_InvalidCadence|TestSubmit_Queued$' -count=1 -v` => PASS, ok github.com/totalwindupflightsystems/off-by-one/internal/api 0.008s; `go build ./...` exit 0.
README submit example includes the required cadence field and matches the live wire format — verified end-to-end that the docs example returns 200 while a cadence-less submit returns 400.

## Summary

Judge Result: REVIEW-OB-013

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README submit example includes cadence field and matches live wire format (submit without cadence 400s; docs example succeeds): README.md:176-190 'Example: Submit a Problem' curl block includes `"cadence": "post-debug"` at line 188, and its field names (problem_class, environment, language, version, description, error_message, cadence) match internal/api/handlers.go:30-39 submitProblemRequest exactly; README.md:204 documents cadence as Required=Yes with values pre-phase/end-of-day/post-debug. LIVE WIRE VERIFICATION: built cmd/off-by-one, ran `serve` on :8801, and POSTed via a Go http client — the README example verbatim returned HTTP 200 {"submission_id":"sub_b88aea","problem_class":"so-nil-pointer-deref","status":"queued","position":1,...}, while the identical body with cadence omitted returned HTTP 400 {"error":"invalid_request","message":"ingest: invalid cadence (accepted: pre-phase, end-of-day, post-debug): got \"\""}. Code path confirmed: internal/ingest/queue.go:322-334 validate() rejects empty/unknown cadence with ErrInvalidCadence, mapped to 400 at internal/api/handlers.go:357-359. Tests: `go test ./internal/api/ -run 'TestSubmit_InvalidCadence|TestSubmit_Queued$' -count=1 -v` => PASS, ok github.com/totalwindupflightsystems/off-by-one/internal/api 0.008s; `go build ./...` exit 0.
README submit example includes the required cadence field and matches the live wire format — verified end-to-end that the docs example returns 200 while a cadence-less submit returns 400.

Overall: PASS ✓
