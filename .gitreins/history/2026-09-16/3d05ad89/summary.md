# Verdict: DF-OFF-BY-ONE-4-GEN3

**Task:** Name accepted cadences in the 400 and document the dedup 409
**Evaluated:** 2026-09-16T06:10:39.487807
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m1:10AM[0m [32mINF[0m [1mscanned ~5732575 bytes (5.73 MB) in 1.99s[0m
[90m1:10AM[0m [32m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.696s
ok  	github.com/totalwindu
- ✓ **tier2**
  - COMPLETE
  ✓ AC1: POST /api/v1/problems/submit with an unknown cadence returns HTTP 400 and the message names all three accepted values (pre-phase, end-of-day, post-debug), asserted by a unit test in internal/ingest. AC2: README.md submit section and docs/api-reference.md document the deduplicated outcome (HTTP 409, status=deduplicated, existing submission_id + position). AC3: internal/solver/failure_hint.go exposes FailureHint which appends an actionable hint naming openrouter.ai/settings/privacy for the OpenRouter guardrail-restriction failure text, tested by TestFailureHint in internal/solver/failure_hint_test.go and called from internal/cron/loop.go. go build ./... and go test ./... -short pass.: AC1: internal/ingest/queue.go:51 defines ErrInvalidCadence = "ingest: invalid cadence (accepted: pre-phase, end-of-day, post-debug)" built from cadenceAcceptedValues (queue.go:42); queue.go:270 wraps it with the rejected value; submit.go:32 StatusForHTTP maps ErrInvalidCadence->400; internal/api/handlers.go:298-300 writes 400 invalid_request with err.Error(). Test internal/ingest/queue_test.go:74 TestSubmit_InvalidCadenceListsAcceptedValues asserts all three accepted values, the rejected value, errors.Is, and StatusForHTTP==400. Ran `go test ./internal/ingest/ -run 'TestSubmit_InvalidCadenceListsAcceptedValues|TestQueue_Submit_ValidatesCadence' -v -count=1` -> both PASS. AC2: README.md:150-166 documents HTTP 409 with "status": "deduplicated" and the existing submission_id plus queue position; docs/api-reference.md:65-77 documents the same (409 duplicate, status=deduplicated, existing submission_id + position); matches handlers.go:283-295 which returns 409 with existing.ID and s.queuePosition. AC3: internal/solver/failure_hint.go:33 FailureHint appends guardrailHint naming https://openrouter.ai/settings/privacy when the text contains the guardrail signature; internal/solver/failure_hint_test.go:12 TestFailureHint PASS (5 subtests, verified via `go test ./internal/solver/ -run TestFailureHint -v -count=1`); called from internal/cron/loop.go:376 `l.cfg.Queue.MarkFailed(ctx, entry.ID, solver.FailureHint(err.Error()))`, with wiring covered by internal/cron/loop_test.go:426 TestTickSolveError_GuardrailHintPersisted. Build/tests: `go build ./...` exit 0; `go test ./... -short -count=1` exit 0 with all 13 packages ok (cmd/off-by-one, internal/api, internal/cron, internal/export, internal/graph, internal/import, internal/ingest, internal/muster, internal/sandbox, internal/seed, internal/solver, internal/web, pkg/api).
All three acceptance criteria are satisfied: the 400 message enumerates the three accepted cadences with a passing internal/ingest unit test, README and docs/api-reference.md document the 409 deduplicated outcome, and FailureHint is implemented, tested, and wired into the cron loop, with go build and go test -short passing.

## Summary

Judge Result: DF-OFF-BY-ONE-4-GEN3

Stage tier1: PASS
    ✓ secrets: [90m1:10AM[0m [32mINF[0m [1mscanned ~5732575 bytes (5.73 MB) in 1.99s[0m
[90m1:10AM[0m [32m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.696s
ok  	github.com/totalwindu

Stage tier2: PASS
  COMPLETE
  ✓ AC1: POST /api/v1/problems/submit with an unknown cadence returns HTTP 400 and the message names all three accepted values (pre-phase, end-of-day, post-debug), asserted by a unit test in internal/ingest. AC2: README.md submit section and docs/api-reference.md document the deduplicated outcome (HTTP 409, status=deduplicated, existing submission_id + position). AC3: internal/solver/failure_hint.go exposes FailureHint which appends an actionable hint naming openrouter.ai/settings/privacy for the OpenRouter guardrail-restriction failure text, tested by TestFailureHint in internal/solver/failure_hint_test.go and called from internal/cron/loop.go. go build ./... and go test ./... -short pass.: AC1: internal/ingest/queue.go:51 defines ErrInvalidCadence = "ingest: invalid cadence (accepted: pre-phase, end-of-day, post-debug)" built from cadenceAcceptedValues (queue.go:42); queue.go:270 wraps it with the rejected value; submit.go:32 StatusForHTTP maps ErrInvalidCadence->400; internal/api/handlers.go:298-300 writes 400 invalid_request with err.Error(). Test internal/ingest/queue_test.go:74 TestSubmit_InvalidCadenceListsAcceptedValues asserts all three accepted values, the rejected value, errors.Is, and StatusForHTTP==400. Ran `go test ./internal/ingest/ -run 'TestSubmit_InvalidCadenceListsAcceptedValues|TestQueue_Submit_ValidatesCadence' -v -count=1` -> both PASS. AC2: README.md:150-166 documents HTTP 409 with "status": "deduplicated" and the existing submission_id plus queue position; docs/api-reference.md:65-77 documents the same (409 duplicate, status=deduplicated, existing submission_id + position); matches handlers.go:283-295 which returns 409 with existing.ID and s.queuePosition. AC3: internal/solver/failure_hint.go:33 FailureHint appends guardrailHint naming https://openrouter.ai/settings/privacy when the text contains the guardrail signature; internal/solver/failure_hint_test.go:12 TestFailureHint PASS (5 subtests, verified via `go test ./internal/solver/ -run TestFailureHint -v -count=1`); called from internal/cron/loop.go:376 `l.cfg.Queue.MarkFailed(ctx, entry.ID, solver.FailureHint(err.Error()))`, with wiring covered by internal/cron/loop_test.go:426 TestTickSolveError_GuardrailHintPersisted. Build/tests: `go build ./...` exit 0; `go test ./... -short -count=1` exit 0 with all 13 packages ok (cmd/off-by-one, internal/api, internal/cron, internal/export, internal/graph, internal/import, internal/ingest, internal/muster, internal/sandbox, internal/seed, internal/solver, internal/web, pkg/api).
All three acceptance criteria are satisfied: the 400 message enumerates the three accepted cadences with a passing internal/ingest unit test, README and docs/api-reference.md document the 409 deduplicated outcome, and FailureHint is implemented, tested, and wired into the cron loop, with go build and go test -short passing.

Overall: PASS ✓
