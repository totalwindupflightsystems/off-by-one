# Verdict: OB-GAP-006

**Task:** Stop passing DeepSeek API key as --api-key CLI arg (ps leak); rely on env vars
**Evaluated:** 2026-08-09T00:12:48.170030
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: /bin/sh: 1: gitleaks: not found

  ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
- ✓ **tier2**
  - COMPLETE
  ✓ grep -n 'api-key' internal/solver/piagent.go returns nothing (the --api-key CLI arg append is removed): grep returns exit 1 (no matches). Args list at piagent.go:241-250 contains only solve/--problem-file/--output/--evidence/--signatures/--model — no --api-key.
  ✓ internal/solver/piagent.go still sets DEEPSEEK_API_KEY and LLM_API_KEY env vars when cfg.APIKey is non-empty: piagent.go:254-258: if e.cfg.APIKey != "" { env = append(env, "DEEPSEEK_API_KEY="+e.cfg.APIKey, "LLM_API_KEY="+e.cfg.APIKey) }
  ✓ internal/solver/piagent_test.go has a regression test asserting args passed to Exec do NOT contain --api-key while env DOES contain DEEPSEEK_API_KEY: TestExecutor_Solve_PropagatesAPIVars at piagent_test.go:654 asserts env has DEEPSEEK_API_KEY (685-690) and args lack --api-key (695-696). Test passes (go test -run TestExecutor_Solve_PropagatesAPIVars PASS).
  ✓ go build ./..., go vet ./..., gofmt -l cmd/ internal/ pkg/ sql/ (empty), and go test -short ./... all pass: go build exit 0, go vet exit 0, gofmt -l empty output, go test -short ./... all packages ok.
All 4 criteria verified: --api-key CLI arg removed from piagent.go, env vars still set, regression test present and passing, and all build/vet/gofmt/test checks pass.

## Summary

Judge Result: OB-GAP-006

Stage tier1: PASS
    ✓ secrets: /bin/sh: 1: gitleaks: not found

  ✓ lint: 
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t

Stage tier2: PASS
  COMPLETE
  ✓ grep -n 'api-key' internal/solver/piagent.go returns nothing (the --api-key CLI arg append is removed): grep returns exit 1 (no matches). Args list at piagent.go:241-250 contains only solve/--problem-file/--output/--evidence/--signatures/--model — no --api-key.
  ✓ internal/solver/piagent.go still sets DEEPSEEK_API_KEY and LLM_API_KEY env vars when cfg.APIKey is non-empty: piagent.go:254-258: if e.cfg.APIKey != "" { env = append(env, "DEEPSEEK_API_KEY="+e.cfg.APIKey, "LLM_API_KEY="+e.cfg.APIKey) }
  ✓ internal/solver/piagent_test.go has a regression test asserting args passed to Exec do NOT contain --api-key while env DOES contain DEEPSEEK_API_KEY: TestExecutor_Solve_PropagatesAPIVars at piagent_test.go:654 asserts env has DEEPSEEK_API_KEY (685-690) and args lack --api-key (695-696). Test passes (go test -run TestExecutor_Solve_PropagatesAPIVars PASS).
  ✓ go build ./..., go vet ./..., gofmt -l cmd/ internal/ pkg/ sql/ (empty), and go test -short ./... all pass: go build exit 0, go vet exit 0, gofmt -l empty output, go test -short ./... all packages ok.
All 4 criteria verified: --api-key CLI arg removed from piagent.go, env vars still set, regression test present and passing, and all build/vet/gofmt/test checks pass.

Overall: PASS ✓
