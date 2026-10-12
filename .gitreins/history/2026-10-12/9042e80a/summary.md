# Verdict: DF-OFF-BY-ONE-35

**Task:** Clarify non-bare export target handling
**Evaluated:** 2026-10-12T17:01:43.148040
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ POST /api/v1/export to a non-bare target returns a client-facing 4xx naming target_repo rather than a raw git push 500, and the export documentation states target_repo must be a bare repo or pushable remote; add focused regression coverage.: All three clauses verified in commit 460d304e. (1) 4xx naming target_repo: internal/export/git.go:168 defines ErrNonBareTarget ("export: target_repo must be a bare repository or a remote that accepts pushes"); validatePushTarget (git.go:510) is invoked at git.go:190 BEFORE any clone/write/commit and detects a local non-bare repo whose checked-out branch equals the push branch; internal/api/handlers.go:987-989 maps errors.Is(err, export.ErrNonBareTarget) to writeError(w, http.StatusBadRequest, "invalid_target_repo", err.Error()). (2) Docs: docs/api-reference.md:459 states target_repo "Must be a bare repository (git init --bare) or a remote that accepts pushes", with a dedicated "Non-bare target (400)" section at line 476 including the 400 invalid_target_repo JSON example (lines 480-481) and status-code list (line 485); pkg/api/openapi.yaml:352 and :713 updated identically. (3) Regression coverage: internal/api/handlers_test.go:2871 TestExportNonBareTarget400 asserts 400 + error=="invalid_target_repo" + message names target_repo + does NOT leak "refusing to update checked out branch" + no mutation before refusal; internal/export/git_test.go:313 TestExport_NonBareTargetRefused, :369 TestExport_NonBareTargetOtherBranchAllowed, and TestLocalRepoPath cover the engine. TEST EVIDENCE: `go test ./internal/api/ -run TestExportNonBareTarget400 -count=1 -v` -> `--- PASS: TestExportNonBareTarget400 (0.02s)` / `ok ... internal/api 0.023s`; `go test ./internal/export/ -run 'NonBare|PushTarget|LocalRepoPath|NonCheckedOut' -count=1 -v` -> all PASS; `go test ./... -short -count=1 -p 1 -timeout 180s` -> every package ok (internal/api ok, internal/export ok). RED-PROOF reproduced by me: neutralizing validatePushTarget (early `return nil`) made TestExportNonBareTarget400 FAIL with `status = 500, want 400; body: {"error":"export_failed",...refusing to update checked out branch...}` — the exact pre-fix behavior — then files were restored (git diff --stat shows only .gitreins/tasks.yaml). Gates: go build ./... exit 0, go vet ./internal/export ./internal/api exit 0, gofmt -l internal/ cmd/ pkg/ empty, LSP diagnostics empty.
POST /api/v1/export now returns 400 invalid_target_repo naming target_repo for non-bare local targets (mapped from typed ErrNonBareTarget before any mutation), docs/api-reference.md and openapi.yaml state the bare/pushable-remote requirement, and focused engine+handler regression tests pass with a reproduced RED-proof.

## Summary

Judge Result: DF-OFF-BY-ONE-35

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ POST /api/v1/export to a non-bare target returns a client-facing 4xx naming target_repo rather than a raw git push 500, and the export documentation states target_repo must be a bare repo or pushable remote; add focused regression coverage.: All three clauses verified in commit 460d304e. (1) 4xx naming target_repo: internal/export/git.go:168 defines ErrNonBareTarget ("export: target_repo must be a bare repository or a remote that accepts pushes"); validatePushTarget (git.go:510) is invoked at git.go:190 BEFORE any clone/write/commit and detects a local non-bare repo whose checked-out branch equals the push branch; internal/api/handlers.go:987-989 maps errors.Is(err, export.ErrNonBareTarget) to writeError(w, http.StatusBadRequest, "invalid_target_repo", err.Error()). (2) Docs: docs/api-reference.md:459 states target_repo "Must be a bare repository (git init --bare) or a remote that accepts pushes", with a dedicated "Non-bare target (400)" section at line 476 including the 400 invalid_target_repo JSON example (lines 480-481) and status-code list (line 485); pkg/api/openapi.yaml:352 and :713 updated identically. (3) Regression coverage: internal/api/handlers_test.go:2871 TestExportNonBareTarget400 asserts 400 + error=="invalid_target_repo" + message names target_repo + does NOT leak "refusing to update checked out branch" + no mutation before refusal; internal/export/git_test.go:313 TestExport_NonBareTargetRefused, :369 TestExport_NonBareTargetOtherBranchAllowed, and TestLocalRepoPath cover the engine. TEST EVIDENCE: `go test ./internal/api/ -run TestExportNonBareTarget400 -count=1 -v` -> `--- PASS: TestExportNonBareTarget400 (0.02s)` / `ok ... internal/api 0.023s`; `go test ./internal/export/ -run 'NonBare|PushTarget|LocalRepoPath|NonCheckedOut' -count=1 -v` -> all PASS; `go test ./... -short -count=1 -p 1 -timeout 180s` -> every package ok (internal/api ok, internal/export ok). RED-PROOF reproduced by me: neutralizing validatePushTarget (early `return nil`) made TestExportNonBareTarget400 FAIL with `status = 500, want 400; body: {"error":"export_failed",...refusing to update checked out branch...}` — the exact pre-fix behavior — then files were restored (git diff --stat shows only .gitreins/tasks.yaml). Gates: go build ./... exit 0, go vet ./internal/export ./internal/api exit 0, gofmt -l internal/ cmd/ pkg/ empty, LSP diagnostics empty.
POST /api/v1/export now returns 400 invalid_target_repo naming target_repo for non-bare local targets (mapped from typed ErrNonBareTarget before any mutation), docs/api-reference.md and openapi.yaml state the bare/pushable-remote requirement, and focused engine+handler regression tests pass with a reproduced RED-proof.

Overall: PASS ✓
