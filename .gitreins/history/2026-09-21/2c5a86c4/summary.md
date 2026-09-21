# Verdict: INT-CI-1

**Task:** De-flake TestQueue_Dequeue_ConcurrentSubmits (CI run 35580590205)
**Evaluated:** 2026-09-21T12:14:17.234839
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ TestQueue_Dequeue_ConcurrentSubmits no longer swallows Submit errors (chan-collected, asserted), still asserts depth==100 and exact-once drain, and pins the contention source by serialising the pool (SetMaxOpenConns(1)) for this test. Verified live on branch wt/INT-CI-1 and on the merged main: count=20 ok (0.375s), -race count=5 ok, go build ./... + go vet ./... + full go test ./... green.: internal/ingest/queue_test.go:357-478 (fix commit da32a07, ancestor of HEAD on master). (1) No longer swallows errors: `errs := make(chan error, workers*perWorker)` L396, `_, _, err := q.Submit(...)` L404, `errs <- err` L408, `close(errs)` L413, then collected L419-424 and asserted with t.Errorf per error (first 3) + t.Fatalf("Submit: %d/%d concurrent submits failed; aborting before drain") L425-433 — the old `_, _, _ = q.Submit(...)` is gone. (2) depth==100 still asserted: L439-441 `if depth != workers*perWorker` (10*10=100). (3) Exact-once drain preserved: `seen` map L451-462 with duplicate check `if seen[e.ID] { t.Errorf("duplicate dequeue: %s", e.ID) }` L457-459, `len(seen) != workers*perWorker` L463-465, and pending==0 L468-475. (4) Contention pinned: `store.DB().SetMaxOpenConns(1)` L391 (only occurrence in internal/, L375 is the explanatory comment). LIVE EVIDENCE: `go test ./internal/ingest -run TestQueue_Dequeue_ConcurrentSubmits -count=20` -> `ok github.com/.../internal/ingest 0.388s`, exit 0 (matches claimed 0.375s); `go test -race ./internal/ingest -run TestQueue_Dequeue_ConcurrentSubmits -count=5` -> `ok ... 4.239s`, exit 0, no race reports; `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` exit 0 with all 14 packages `ok` (cmd/off-by-one, internal/api, cron, export, graph, import, ingest, muster, sandbox, seed, solver, tools, web, pkg/api).
The de-flake fix is present and complete on merged main: Submit errors are chan-collected and asserted, depth==100 and exact-once drain are preserved, SetMaxOpenConns(1) serialises the pool, and count=20/-race count=5/build/vet/full test suite all pass live.

## Summary

Judge Result: INT-CI-1

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ TestQueue_Dequeue_ConcurrentSubmits no longer swallows Submit errors (chan-collected, asserted), still asserts depth==100 and exact-once drain, and pins the contention source by serialising the pool (SetMaxOpenConns(1)) for this test. Verified live on branch wt/INT-CI-1 and on the merged main: count=20 ok (0.375s), -race count=5 ok, go build ./... + go vet ./... + full go test ./... green.: internal/ingest/queue_test.go:357-478 (fix commit da32a07, ancestor of HEAD on master). (1) No longer swallows errors: `errs := make(chan error, workers*perWorker)` L396, `_, _, err := q.Submit(...)` L404, `errs <- err` L408, `close(errs)` L413, then collected L419-424 and asserted with t.Errorf per error (first 3) + t.Fatalf("Submit: %d/%d concurrent submits failed; aborting before drain") L425-433 — the old `_, _, _ = q.Submit(...)` is gone. (2) depth==100 still asserted: L439-441 `if depth != workers*perWorker` (10*10=100). (3) Exact-once drain preserved: `seen` map L451-462 with duplicate check `if seen[e.ID] { t.Errorf("duplicate dequeue: %s", e.ID) }` L457-459, `len(seen) != workers*perWorker` L463-465, and pending==0 L468-475. (4) Contention pinned: `store.DB().SetMaxOpenConns(1)` L391 (only occurrence in internal/, L375 is the explanatory comment). LIVE EVIDENCE: `go test ./internal/ingest -run TestQueue_Dequeue_ConcurrentSubmits -count=20` -> `ok github.com/.../internal/ingest 0.388s`, exit 0 (matches claimed 0.375s); `go test -race ./internal/ingest -run TestQueue_Dequeue_ConcurrentSubmits -count=5` -> `ok ... 4.239s`, exit 0, no race reports; `go build ./...` exit 0; `go vet ./...` exit 0; `go test -count=1 ./...` exit 0 with all 14 packages `ok` (cmd/off-by-one, internal/api, cron, export, graph, import, ingest, muster, sandbox, seed, solver, tools, web, pkg/api).
The de-flake fix is present and complete on merged main: Submit errors are chan-collected and asserted, depth==100 and exact-once drain are preserved, SetMaxOpenConns(1) serialises the pool, and count=20/-race count=5/build/vet/full test suite all pass live.

Overall: PASS ✓
