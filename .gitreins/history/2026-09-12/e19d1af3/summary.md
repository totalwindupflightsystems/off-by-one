# Verdict: OB-GAP-057

**Task:** Failed-signature answers are stored/served as verified
**Evaluated:** 2026-09-12T05:30:27.652386
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m12:29AM[0m [32mINF[0m [1mscanned ~5892114 bytes (5.89 MB) in 1.35s[0m
[90m12:29AM[0m [3
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.352s
ok  	github.com/totalwindu
- ✓ **tier2**
  - COMPLETE
  ✓ live: curl -s -X POST http://localhost:8766/api/v1/problems/discover -H "Content-Type: application/json" -d {"problem_class":"go-board-audit-idle-tick"} returns found:false with no answer object (that class has exactly one answer and its signature says failed), while a class with a passing answer such as python-sdk-e2e-battery still returns found:true with an answer: Live curl returned {"found":false,"related":[],"version_warnings":[]} for go-board-audit-idle-tick (no answer object). python-sdk-e2e-battery returned {"found":true,"answer":{"id":1281,...,"status":"verified"}}. DB confirms go-board-audit-idle-tick has exactly one answer (id 331) with status=failed and json_extract(signatures,'$.result')='failed'. Server PID 2902013 started 00:29, after fix commit fb8e71d (00:26), so it runs the fixed binary.
  ✓ live: sqlite3 "file:/home/kara/off-by-one/off-by-one.db?mode=ro" "SELECT COUNT(*) FROM answer_nodes WHERE status=verified AND json_extract(signatures,$.result)=failed;" prints 0: sqlite3 (read-only) printed 0, exit 0. Status distribution is failed|28, verified|1687 — the 28 failed-signature rows were reclassified from verified to failed.
  ✓ code: go test ./internal/solver/ ./internal/seed/ ./internal/graph/ -count=1 exits 0 and the new tests assert a solve whose signatures result is failed is stored with status failed and is never returned by discovery nor used for queue dedup: `go test ./internal/solver/ ./internal/seed/ ./internal/graph/ -count=1` exit 0: ok solver 0.137s, ok seed 1.006s, ok graph 0.250s. New tests: piagent_test.go TestExecutor_Commit_FailedSignatureStoredFailed (Commit stores graph.AnswerFailed for result=failed), store_test.go TestStore_Discovery_ExcludesFailedSignature (Discovery never serves a failed-signature row even when status=verified), queue_test.go TestQueue_Submit_FailedSignatureDoesNotDedup (failed-signature row does not suppress re-submission), seed_test.go TestSeedHonoursFailedSignature (failed-signature corpus answer imported as failed, not discoverable). Negative control: reverting only the 4 source files to fb8e71d^ makes these tests FAIL (Discovery served "gave up"; Submit returned ErrDuplicate; undefined answerStatusFromSignatures/corpusAnswerStatus), proving the assertions are real. Working tree restored clean; go build ./... OK, go vet ./... exit 0.
All three criteria pass: live discovery returns found:false for the failed-signature class and found:true for a passing class, the read-only DB query returns 0, and the new solver/seed/graph/ingest tests pass and genuinely fail without the fix.

## Summary

Judge Result: OB-GAP-057

Stage tier1: PASS
    ✓ secrets: [90m12:29AM[0m [32mINF[0m [1mscanned ~5892114 bytes (5.89 MB) in 1.35s[0m
[90m12:29AM[0m [3
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.352s
ok  	github.com/totalwindu

Stage tier2: PASS
  COMPLETE
  ✓ live: curl -s -X POST http://localhost:8766/api/v1/problems/discover -H "Content-Type: application/json" -d {"problem_class":"go-board-audit-idle-tick"} returns found:false with no answer object (that class has exactly one answer and its signature says failed), while a class with a passing answer such as python-sdk-e2e-battery still returns found:true with an answer: Live curl returned {"found":false,"related":[],"version_warnings":[]} for go-board-audit-idle-tick (no answer object). python-sdk-e2e-battery returned {"found":true,"answer":{"id":1281,...,"status":"verified"}}. DB confirms go-board-audit-idle-tick has exactly one answer (id 331) with status=failed and json_extract(signatures,'$.result')='failed'. Server PID 2902013 started 00:29, after fix commit fb8e71d (00:26), so it runs the fixed binary.
  ✓ live: sqlite3 "file:/home/kara/off-by-one/off-by-one.db?mode=ro" "SELECT COUNT(*) FROM answer_nodes WHERE status=verified AND json_extract(signatures,$.result)=failed;" prints 0: sqlite3 (read-only) printed 0, exit 0. Status distribution is failed|28, verified|1687 — the 28 failed-signature rows were reclassified from verified to failed.
  ✓ code: go test ./internal/solver/ ./internal/seed/ ./internal/graph/ -count=1 exits 0 and the new tests assert a solve whose signatures result is failed is stored with status failed and is never returned by discovery nor used for queue dedup: `go test ./internal/solver/ ./internal/seed/ ./internal/graph/ -count=1` exit 0: ok solver 0.137s, ok seed 1.006s, ok graph 0.250s. New tests: piagent_test.go TestExecutor_Commit_FailedSignatureStoredFailed (Commit stores graph.AnswerFailed for result=failed), store_test.go TestStore_Discovery_ExcludesFailedSignature (Discovery never serves a failed-signature row even when status=verified), queue_test.go TestQueue_Submit_FailedSignatureDoesNotDedup (failed-signature row does not suppress re-submission), seed_test.go TestSeedHonoursFailedSignature (failed-signature corpus answer imported as failed, not discoverable). Negative control: reverting only the 4 source files to fb8e71d^ makes these tests FAIL (Discovery served "gave up"; Submit returned ErrDuplicate; undefined answerStatusFromSignatures/corpusAnswerStatus), proving the assertions are real. Working tree restored clean; go build ./... OK, go vet ./... exit 0.
All three criteria pass: live discovery returns found:false for the failed-signature class and found:true for a passing class, the read-only DB query returns 0, and the new solver/seed/graph/ingest tests pass and genuinely fail without the fix.

Overall: PASS ✓
