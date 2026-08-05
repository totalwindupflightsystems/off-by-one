# Verdict: SBOX-002

**Task:** Custom sandbox provisioning — required_tools declared in submit, resolved and ro-mounted into bwrap
**Evaluated:** 2026-08-05T16:47:49.450743
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
  ✓ secrets: [90m11:46AM[0m [32mINF[0m [1mscanned ~3596014 bytes (3.60 MB) in 419ms[0m
[90m11:46AM[0m [3
- ✓ **tier2**
  - COMPLETE
  ✓ Submit API accepts required_tools field and persists it (ingest queue schema + migration for existing DBs): handlers.go submitProblemRequest has RequiredTools []string json:"required_tools,omitempty" and handleSubmitProblem plumbs it into ingest.Submission; queue.go Submission/Entry carry RequiredTools; sql/schema/queue.sql adds required_tools TEXT NOT NULL DEFAULT '[]'; INSERT/SELECT/scanEntry updated; Open() runs ALTER TABLE ADD COLUMN migration with duplicate-column guard (queue.go:118-123). Verified migration re-adds column to existing DB via manual test.
  ✓ Sandbox resolver maps tool names to host paths via exec.LookPath, dedups against DefaultReadOnlyPaths (prefix coverage): tools.go ResolveTools uses lookPathMulti (exec.LookPath), dedups via isPathCovered prefix coverage against alreadyMounted (DefaultReadOnlyPaths), sorts output; special cases for git/python3-venv. Covered by TestResolveTools_DedupeAlreadyMounted, TestIsPathCovered, TestResolveTools_GitSpecialCase.
  ✓ Runner.Create accepts WithRequiredTools option and mounts resolved paths read-only via ExtraReadOnlyPaths: piagent.go defines CreateOption variadic + WithRequiredTools + resolveCreateOptions; BSandboxRunner.Create (bsandbox_runner.go:28) resolves tools and sets cfg.ExtraReadOnlyPaths; bwrap.go buildBwrapArgs mounts extraReadOnlyPaths via --ro-bind (read-only); Solve() passes WithRequiredTools(sub.RequiredTools).
  ✓ Unresolvable tools degrade gracefully: warning logged, solve continues without them (never fails): ResolveTools returns missing tools without error (documented degrade-gracefully contract in tools.go); BSandboxRunner.Create logs slog.Warn for missing and always proceeds to r.exec.Create. TestResolveTools_UnknownTool and TestResolveTools_Mixed verify unknown tools return missing without failure.
  ✓ New unit tests cover resolver, handler, bwrap-args, piagent problem.json; all existing tests pass: New tests: tools_test.go (TestResolveTools_*, TestIsPathCovered), handlers_test.go (TestSubmit_RequiredTools, TestSubmit_RequiredTools_Empty), bwrap_test.go (TestBuildBwrapArgs_ExtraReadOnlyPaths), piagent_test.go (TestExecutor_Solve_RequiredToolsInProblemJSON). go test -count=1 ./... all pass; go vet clean; LSP diagnostics empty.
All 5 SBOX-002 criteria are fully implemented and verified: required_tools flows from submit API through queue schema+migration, resolver dedups via exec.LookPath against DefaultReadOnlyPaths, WithRequiredTools mounts read-only via ExtraReadOnlyPaths, unresolvable tools degrade gracefully, and all new+existing tests pass.

## Summary

Judge Result: SBOX-002

Stage tier1: PASS
    ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
  ✓ secrets: [90m11:46AM[0m [32mINF[0m [1mscanned ~3596014 bytes (3.60 MB) in 419ms[0m
[90m11:46AM[0m [3

Stage tier2: PASS
  COMPLETE
  ✓ Submit API accepts required_tools field and persists it (ingest queue schema + migration for existing DBs): handlers.go submitProblemRequest has RequiredTools []string json:"required_tools,omitempty" and handleSubmitProblem plumbs it into ingest.Submission; queue.go Submission/Entry carry RequiredTools; sql/schema/queue.sql adds required_tools TEXT NOT NULL DEFAULT '[]'; INSERT/SELECT/scanEntry updated; Open() runs ALTER TABLE ADD COLUMN migration with duplicate-column guard (queue.go:118-123). Verified migration re-adds column to existing DB via manual test.
  ✓ Sandbox resolver maps tool names to host paths via exec.LookPath, dedups against DefaultReadOnlyPaths (prefix coverage): tools.go ResolveTools uses lookPathMulti (exec.LookPath), dedups via isPathCovered prefix coverage against alreadyMounted (DefaultReadOnlyPaths), sorts output; special cases for git/python3-venv. Covered by TestResolveTools_DedupeAlreadyMounted, TestIsPathCovered, TestResolveTools_GitSpecialCase.
  ✓ Runner.Create accepts WithRequiredTools option and mounts resolved paths read-only via ExtraReadOnlyPaths: piagent.go defines CreateOption variadic + WithRequiredTools + resolveCreateOptions; BSandboxRunner.Create (bsandbox_runner.go:28) resolves tools and sets cfg.ExtraReadOnlyPaths; bwrap.go buildBwrapArgs mounts extraReadOnlyPaths via --ro-bind (read-only); Solve() passes WithRequiredTools(sub.RequiredTools).
  ✓ Unresolvable tools degrade gracefully: warning logged, solve continues without them (never fails): ResolveTools returns missing tools without error (documented degrade-gracefully contract in tools.go); BSandboxRunner.Create logs slog.Warn for missing and always proceeds to r.exec.Create. TestResolveTools_UnknownTool and TestResolveTools_Mixed verify unknown tools return missing without failure.
  ✓ New unit tests cover resolver, handler, bwrap-args, piagent problem.json; all existing tests pass: New tests: tools_test.go (TestResolveTools_*, TestIsPathCovered), handlers_test.go (TestSubmit_RequiredTools, TestSubmit_RequiredTools_Empty), bwrap_test.go (TestBuildBwrapArgs_ExtraReadOnlyPaths), piagent_test.go (TestExecutor_Solve_RequiredToolsInProblemJSON). go test -count=1 ./... all pass; go vet clean; LSP diagnostics empty.
All 5 SBOX-002 criteria are fully implemented and verified: required_tools flows from submit API through queue schema+migration, resolver dedups via exec.LookPath against DefaultReadOnlyPaths, WithRequiredTools mounts read-only via ExtraReadOnlyPaths, unresolvable tools degrade gracefully, and all new+existing tests pass.

Overall: PASS ✓
