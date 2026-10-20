# Verdict: DF-OFF-BY-ONE-49-fix

**Task:** Fix board validator to allow legacy event ID anomalies
**Evaluated:** 2026-10-17T22:46:23.201384
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ boardctl validate on .coding-hermes/board returns RESULT: FAIL due to 2 descending event IDs in legacy data; fix validator to pass (either renumber legacy ids upward or accept legacy anomalies): Original FAIL state confirmed: .coding-hermes/board/events.jsonl.bak (pre-fix snapshot) contains exactly 2 descending numeric event IDs — line 387 (id 367 after running max 370) and line 427 (id 424 after running max 1425), matching the criterion's '2 descending event IDs'. Fix applied via the allowed 'renumber legacy ids upward' option: line 387 id 367->371, line 427 id 424->1426; current events.jsonl has zero descending numeric IDs (line count unchanged 466->466). `boardctl validate .coding-hermes/board` now returns 'RESULT: OK (181 warning(s))' exit 0 (boardctl repo board also 'RESULT: OK (6 warning(s))' exit 0). Validator was NOT weakened: internal/board/validate.go is unchanged (git diff empty) and still enforces ascending ids at validate.go:534 ('event id %d descends below earlier id %d (ids must ascend; gaps tolerated)'); a synthetic board with ids 1,5,3 still yields 'RESULT: FAIL (4 error(s))' including the descending-id error. Tests: `go test ./internal/board/ -count=1` => 'ok github.com/coding-hermes/boardctl/internal/board 1.090s' (exit 0); `go build ./...` exit 0; `go vet ./internal/board/` exit 0.
The 2 legacy descending event IDs (367 and 424) were renumbered upward to 371 and 1426, so boardctl validate now returns RESULT: OK while the validator's ascending-ID check remains intact and still fails synthetic descending data.

## Summary

Judge Result: DF-OFF-BY-ONE-49-fix

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ boardctl validate on .coding-hermes/board returns RESULT: FAIL due to 2 descending event IDs in legacy data; fix validator to pass (either renumber legacy ids upward or accept legacy anomalies): Original FAIL state confirmed: .coding-hermes/board/events.jsonl.bak (pre-fix snapshot) contains exactly 2 descending numeric event IDs — line 387 (id 367 after running max 370) and line 427 (id 424 after running max 1425), matching the criterion's '2 descending event IDs'. Fix applied via the allowed 'renumber legacy ids upward' option: line 387 id 367->371, line 427 id 424->1426; current events.jsonl has zero descending numeric IDs (line count unchanged 466->466). `boardctl validate .coding-hermes/board` now returns 'RESULT: OK (181 warning(s))' exit 0 (boardctl repo board also 'RESULT: OK (6 warning(s))' exit 0). Validator was NOT weakened: internal/board/validate.go is unchanged (git diff empty) and still enforces ascending ids at validate.go:534 ('event id %d descends below earlier id %d (ids must ascend; gaps tolerated)'); a synthetic board with ids 1,5,3 still yields 'RESULT: FAIL (4 error(s))' including the descending-id error. Tests: `go test ./internal/board/ -count=1` => 'ok github.com/coding-hermes/boardctl/internal/board 1.090s' (exit 0); `go build ./...` exit 0; `go vet ./internal/board/` exit 0.
The 2 legacy descending event IDs (367 and 424) were renumbered upward to 371 and 1426, so boardctl validate now returns RESULT: OK while the validator's ascending-ID check remains intact and still fails synthetic descending data.

Overall: PASS ✓
