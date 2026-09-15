# Verdict: DF-OFF-BY-ONE-4

**Task:** Solve failures are invisible to the submitter — no failure_reason exposed via API
**Evaluated:** 2026-09-15T01:45:34.411178
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m8:44PM[0m [32mINF[0m [1mscanned ~5607291 bytes (5.61 MB) in 1.23s[0m
[90m8:44PM[0m [32m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
- ✓ **tier2**
  - COMPLETE
  ✓ queue_entries gains a failure_reason TEXT column (schema + defensive ALTER for existing DBs); Queue.MarkFailed persists the reason; Queue.scanEntry and Entry carry it; entryToWire exposes failure_reason in GET /api/v1/queue/{submission_id} responses (empty when not failed); cron loop's MarkFailed calls pass the error text through unchanged: sql/schema/queue.sql:23 adds `failure_reason TEXT NOT NULL DEFAULT ''`; internal/ingest/queue.go:157 runs the defensive `ALTER TABLE queue_entries ADD COLUMN failure_reason TEXT NOT NULL DEFAULT ''` on Open with duplicate-column ignored (mirrors required_tools block at :150). Entry.FailureReason at queue.go:108 (json:"failure_reason,omitempty"); scanEntry scans &e.FailureReason at queue.go:599; all explicit SELECT lists include the column (queue.go:292, 349, 372, 499). MarkFailed at queue.go:530-537 persists `failure_reason = ?` via truncateFailureReason(reason). internal/api/handlers.go:141 queueEntryWire.FailureReason with omitempty (absent when not failed); entryToWire sets it at handlers.go:731; handleGetQueueStatus at handlers.go:629-640 returns entryToWire(e) for GET /api/v1/queue/{submission_id}. internal/cron/loop.go:369 and :379 call MarkFailed(ctx, entry.ID, err.Error()) — error text passed through unchanged. pkg/api/openapi.yaml:604 documents the field. go build ./... exit 0, go vet ./... exit 0, gofmt -l cmd/ internal/ pkg/ sql/ empty.
  ✓ A test in internal/ingest/queue_test.go marks an entry failed with a reason and asserts ReadBack/Get returns the reason; a test in internal/api/handlers_test.go asserts GET /api/v1/queue/{id} on a failed entry returns JSON with non-empty failure_reason matching the reason passed to MarkFailed: internal/ingest/queue_test.go:417 TestQueue_MarkFailed calls q.MarkFailed(ctx, id, reason) then asserts got.FailureReason == reason via Get (:439) and List (:452), plus pending entry carries empty reason (:464); queue_test.go:513 TestQueue_Open_MigratesFailureReason covers the legacy-DB ALTER. internal/api/handlers_test.go:1024 TestGetQueueStatus_FailureReason submits, marks failed with reason, then GET /api/v1/queue/{id} asserts 200, non-empty entry.FailureReason, equality with reason (:1062-1065), and a raw-JSON map check (:1070). Command evidence: `go test ./internal/ingest/... ./internal/api/... -run 'MarkFailed|FailureReason|Migrat' -count=1 -v` => PASS: TestQueue_MarkFailed, TestQueue_MarkFailed_TruncatesLongReason, TestQueue_Open_MigratesFailureReason, TestGetQueueStatus_FailureReason all PASS. Full suite `go test ./... -short -p 1 -count=1 -timeout 180s` => all packages ok, exit_code 0.
failure_reason is persisted end-to-end (schema + defensive ALTER, MarkFailed, Entry/scanEntry, entryToWire on GET /api/v1/queue/{id}, cron passes err.Error()) with passing ingest and API tests plus a clean build/vet/gofmt and full test suite.

## Summary

Judge Result: DF-OFF-BY-ONE-4

Stage tier1: PASS
    ✓ secrets: [90m8:44PM[0m [32mINF[0m [1mscanned ~5607291 bytes (5.61 MB) in 1.23s[0m
[90m8:44PM[0m [32m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin

Stage tier2: PASS
  COMPLETE
  ✓ queue_entries gains a failure_reason TEXT column (schema + defensive ALTER for existing DBs); Queue.MarkFailed persists the reason; Queue.scanEntry and Entry carry it; entryToWire exposes failure_reason in GET /api/v1/queue/{submission_id} responses (empty when not failed); cron loop's MarkFailed calls pass the error text through unchanged: sql/schema/queue.sql:23 adds `failure_reason TEXT NOT NULL DEFAULT ''`; internal/ingest/queue.go:157 runs the defensive `ALTER TABLE queue_entries ADD COLUMN failure_reason TEXT NOT NULL DEFAULT ''` on Open with duplicate-column ignored (mirrors required_tools block at :150). Entry.FailureReason at queue.go:108 (json:"failure_reason,omitempty"); scanEntry scans &e.FailureReason at queue.go:599; all explicit SELECT lists include the column (queue.go:292, 349, 372, 499). MarkFailed at queue.go:530-537 persists `failure_reason = ?` via truncateFailureReason(reason). internal/api/handlers.go:141 queueEntryWire.FailureReason with omitempty (absent when not failed); entryToWire sets it at handlers.go:731; handleGetQueueStatus at handlers.go:629-640 returns entryToWire(e) for GET /api/v1/queue/{submission_id}. internal/cron/loop.go:369 and :379 call MarkFailed(ctx, entry.ID, err.Error()) — error text passed through unchanged. pkg/api/openapi.yaml:604 documents the field. go build ./... exit 0, go vet ./... exit 0, gofmt -l cmd/ internal/ pkg/ sql/ empty.
  ✓ A test in internal/ingest/queue_test.go marks an entry failed with a reason and asserts ReadBack/Get returns the reason; a test in internal/api/handlers_test.go asserts GET /api/v1/queue/{id} on a failed entry returns JSON with non-empty failure_reason matching the reason passed to MarkFailed: internal/ingest/queue_test.go:417 TestQueue_MarkFailed calls q.MarkFailed(ctx, id, reason) then asserts got.FailureReason == reason via Get (:439) and List (:452), plus pending entry carries empty reason (:464); queue_test.go:513 TestQueue_Open_MigratesFailureReason covers the legacy-DB ALTER. internal/api/handlers_test.go:1024 TestGetQueueStatus_FailureReason submits, marks failed with reason, then GET /api/v1/queue/{id} asserts 200, non-empty entry.FailureReason, equality with reason (:1062-1065), and a raw-JSON map check (:1070). Command evidence: `go test ./internal/ingest/... ./internal/api/... -run 'MarkFailed|FailureReason|Migrat' -count=1 -v` => PASS: TestQueue_MarkFailed, TestQueue_MarkFailed_TruncatesLongReason, TestQueue_Open_MigratesFailureReason, TestGetQueueStatus_FailureReason all PASS. Full suite `go test ./... -short -p 1 -count=1 -timeout 180s` => all packages ok, exit_code 0.
failure_reason is persisted end-to-end (schema + defensive ALTER, MarkFailed, Entry/scanEntry, entryToWire on GET /api/v1/queue/{id}, cron passes err.Error()) with passing ingest and API tests plus a clean build/vet/gofmt and full test suite.

Overall: PASS ✓
