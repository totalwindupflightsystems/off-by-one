# Verdict: REVIEW-OB-022

**Task:** readJSON rejects trailing JSON documents (400 after the first document)
**Evaluated:** 2026-10-22T21:06:03.301250
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ 1) readJSON (internal/api/server.go:251) after json.Decode verifies the stream is exhausted via dec.Token(): io.EOF -> OK, any other value -> 400 with an invalid-JSON trailing-data error. Live repro on :8766 was a discover body ending in a second document ({..}{"junk":1}) returning 200. 2) A trailing-WHITESPACE body still decodes and returns 200 (Token returns io.EOF). 3) New tests in internal/api assert the trailing-document 400 and the trailing-whitespace 200; duplicate-key and unknown-field behavior unchanged. 4) go build ./... , go vet ./... and go test ./... -count=1 all pass. 5) ch:trace row=REVIEW-OB-022 marker present in a committed test artifact; committed tree greppable: grep -c 'ch:trace row=REVIEW-OB-022' internal/api/server_test.go >= 1 (create server_test.go in package api if it does not exist).: (1) internal/api/server.go:258-273 readJSON: after dec.Decode it runs `if _, err := dec.Token(); !errors.Is(err, io.EOF) { return fmt.Errorf("invalid JSON: trailing data") }`; handleDiscover (internal/api/handlers.go:472-477) maps any readJSON error to 400 invalid_request. (2) Trailing whitespace still OK — TestReadJSONAcceptsTrailingWhitespace covers 5 whitespace bodies (spaces/tabs/CRLF/newlines/leading whitespace) and passes. (3) internal/api/server_test.go: TestReadJSONRejectsTrailingDocument (8 trailing-data bodies incl. `{"problem_class":"unknown"}{"junk":1}`), TestReadJSONAcceptsTrailingWhitespace, TestReadJSONDuplicateKeysLastWins, TestReadJSONRejectsUnknownField, TestReadJSONKeepsSizeCap, TestDiscoverRejectsTrailingDocumentAs400 (asserts 400 + error=invalid_request + message mentions trailing data). `go test ./internal/api -run 'TestReadJSON|TestDiscoverRejectsTrailingDocumentAs400' -v -count=1` -> all 6 PASS, `ok github.com/.../internal/api 0.009s`. (4) `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./... -count=1` exit 0 with 15 `ok` packages and zero FAIL/panic lines. (5) grep -c 'ch:trace row=REVIEW-OB-022' internal/api/server_test.go = 1 (line 46); file is git-tracked (git ls-files) and the marker is present in HEAD:internal/api/server_test.go (grep -c = 1), so it is committed. [resolution 0.78; internal/api/server.go:251, internal/api/server_test.go, server_test.go]
readJSON now rejects trailing JSON documents with a 400 invalid_request while still accepting trailing whitespace, backed by committed tests and a clean build/vet/test run.

## Summary

Judge Result: REVIEW-OB-022

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ 1) readJSON (internal/api/server.go:251) after json.Decode verifies the stream is exhausted via dec.Token(): io.EOF -> OK, any other value -> 400 with an invalid-JSON trailing-data error. Live repro on :8766 was a discover body ending in a second document ({..}{"junk":1}) returning 200. 2) A trailing-WHITESPACE body still decodes and returns 200 (Token returns io.EOF). 3) New tests in internal/api assert the trailing-document 400 and the trailing-whitespace 200; duplicate-key and unknown-field behavior unchanged. 4) go build ./... , go vet ./... and go test ./... -count=1 all pass. 5) ch:trace row=REVIEW-OB-022 marker present in a committed test artifact; committed tree greppable: grep -c 'ch:trace row=REVIEW-OB-022' internal/api/server_test.go >= 1 (create server_test.go in package api if it does not exist).: (1) internal/api/server.go:258-273 readJSON: after dec.Decode it runs `if _, err := dec.Token(); !errors.Is(err, io.EOF) { return fmt.Errorf("invalid JSON: trailing data") }`; handleDiscover (internal/api/handlers.go:472-477) maps any readJSON error to 400 invalid_request. (2) Trailing whitespace still OK — TestReadJSONAcceptsTrailingWhitespace covers 5 whitespace bodies (spaces/tabs/CRLF/newlines/leading whitespace) and passes. (3) internal/api/server_test.go: TestReadJSONRejectsTrailingDocument (8 trailing-data bodies incl. `{"problem_class":"unknown"}{"junk":1}`), TestReadJSONAcceptsTrailingWhitespace, TestReadJSONDuplicateKeysLastWins, TestReadJSONRejectsUnknownField, TestReadJSONKeepsSizeCap, TestDiscoverRejectsTrailingDocumentAs400 (asserts 400 + error=invalid_request + message mentions trailing data). `go test ./internal/api -run 'TestReadJSON|TestDiscoverRejectsTrailingDocumentAs400' -v -count=1` -> all 6 PASS, `ok github.com/.../internal/api 0.009s`. (4) `go build ./...` exit 0; `go vet ./...` exit 0; `go test ./... -count=1` exit 0 with 15 `ok` packages and zero FAIL/panic lines. (5) grep -c 'ch:trace row=REVIEW-OB-022' internal/api/server_test.go = 1 (line 46); file is git-tracked (git ls-files) and the marker is present in HEAD:internal/api/server_test.go (grep -c = 1), so it is committed. [resolution 0.78; internal/api/server.go:251, internal/api/server_test.go, server_test.go]
readJSON now rejects trailing JSON documents with a 400 invalid_request while still accepting trailing whitespace, backed by committed tests and a clean build/vet/test run.

Overall: PASS ✓
