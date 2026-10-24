# Verdict: DF-OFF-BY-ONE-57

**Task:** README answered-arm dedup contract
**Evaluated:** 2026-10-24T09:43:13.494266
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✓ **tier2**
  - COMPLETE
  ✓ README dedup section documents both queue and answered 409 arms; answered example has empty submission_id, position 0, reason answered, and next_action discover: README.md dedup section (lines ~217-255) now states the 409 dedup response 'has two possible arms'. Queue arm bullet + JSON example: submission_id "sub_525a7d", position 1. Answered arm bullet + JSON example: "submission_id": "", "position": 0, "reason": "answered", "next_action": "POST /api/v1/problems/discover". This matches the implementation in internal/api/handlers.go:355-390 (queue arm sets Reason="queued" + existing.ID + queuePosition; corpus arm sets Reason="answered", NextAction="POST /api/v1/problems/discover", leaves SubmissionID empty). Test evidence: `go test ./internal/api/ -run 'TestSubmit' -count=1` => exit_code 0, 'ok github.com/totalwindupflightsystems/off-by-one/internal/api 0.048s'; verbose run shows '--- PASS: TestSubmit_DedupVerifiedAnswer' and '--- PASS: TestSubmit_DedupPending', with handlers_test.go:376-388 asserting reason=="answered", empty submission_id, and next_action containing /api/v1/problems/discover.
README dedup section documents both the queue and answered 409 arms with an answered example carrying empty submission_id, position 0, reason "answered", and next_action discover, matching handlers.go and passing tests.

## Summary

Judge Result: DF-OFF-BY-ONE-57

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: PASS
  COMPLETE
  ✓ README dedup section documents both queue and answered 409 arms; answered example has empty submission_id, position 0, reason answered, and next_action discover: README.md dedup section (lines ~217-255) now states the 409 dedup response 'has two possible arms'. Queue arm bullet + JSON example: submission_id "sub_525a7d", position 1. Answered arm bullet + JSON example: "submission_id": "", "position": 0, "reason": "answered", "next_action": "POST /api/v1/problems/discover". This matches the implementation in internal/api/handlers.go:355-390 (queue arm sets Reason="queued" + existing.ID + queuePosition; corpus arm sets Reason="answered", NextAction="POST /api/v1/problems/discover", leaves SubmissionID empty). Test evidence: `go test ./internal/api/ -run 'TestSubmit' -count=1` => exit_code 0, 'ok github.com/totalwindupflightsystems/off-by-one/internal/api 0.048s'; verbose run shows '--- PASS: TestSubmit_DedupVerifiedAnswer' and '--- PASS: TestSubmit_DedupPending', with handlers_test.go:376-388 asserting reason=="answered", empty submission_id, and next_action containing /api/v1/problems/discover.
README dedup section documents both the queue and answered 409 arms with an answered example carrying empty submission_id, position 0, reason "answered", and next_action discover, matching handlers.go and passing tests.

Overall: PASS ✓
