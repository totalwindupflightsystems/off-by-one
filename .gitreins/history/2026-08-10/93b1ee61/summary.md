# Verdict: OB-GAP-015

**Task:** Stop leaking DEEPSEEK_API_KEY in process argv (bwrap env shim -> envp)
**Evaluated:** 2026-08-10T06:00:09.169256
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ lint: 
  ✓ secrets: [90m12:58AM[0m [32mINF[0m [1mscanned ~5144790 bytes (5.14 MB) in 1.11s[0m
[90m12:58AM[0m [3
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t
- ✓ **tier2**
  - COMPLETE
  ✓ grep -rn '/usr/bin/env' internal/ shows no env-carrying shim for Exec (code line gone; doc-comment mentions allowed): grep shows only doc-comments (internal/sandbox/bwrap.go:273, internal/solver/bsandbox_runner.go:75) and test assertions (bsandbox_runner_test.go:195-196); no env-carrying shim code line exists
  ✓ internal/sandbox/bwrap.go exposes RunWithEnv(ctx, name, args, env) delivering per-call env via exec.Cmd.Env (envp), appended after Config.ExtraEnv so per-call wins on duplicates; Run delegates to it with env=nil: bwrap.go:276 RunWithEnv(ctx,name,args,env); cmd.Env=append(os.Environ(), s.cfg.ExtraEnv...) then append(cmd.Env, env...) at line 290 so per-call wins; Run (line 262) delegates to RunWithEnv(ctx,name,args,nil) at line 263
  ✓ internal/solver/bsandbox_runner.go Exec uses RunWithEnv when env is non-empty and no longer builds a /usr/bin/env KEY=VAL argv; stderr-merge-on-error behavior preserved: bsandbox_runner.go Exec (line 76) calls RunWithEnv when len(env)>0 (line 82) else Run; no /usr/bin/env argv built; stderr-merge-on-error preserved at lines 88-96
  ✓ Regression tests (sandbox + solver) drive a fake bwrap binary that dumps its argv and its own env: argv contains NO DEEPSEEK_API_KEY/LLM_API_KEY/sk- values, env DOES contain the keys; empty-env path runs the command directly: bwrap_test.go:468 TestRunWithEnv_EnvNotInArgv and bsandbox_runner_test.go:129 TestExecEnvNotInArgv use makeRecordingBwrap dumping argv+env; assert argv has no DEEPSEEK_API_KEY/LLM_API_KEY/sk-, env has keys, and empty-env subtest runs /bin/echo directly without /usr/bin/env shim
  ✓ go build ./..., go vet ./..., gofmt -l cmd/ internal/ pkg/ sql/ (empty), and go test ./... -short -p 1 -count=1 -timeout 120s all pass; gitreins guard 4/4 PASS: go build exit 0, go vet exit 0, gofmt -l empty, go test ./... -short -p 1 -count=1 -timeout 120s exit 0 (11 packages ok, no FAIL/panic), gitreins guard exit 0 with 'Tier 1 Guards: PASS' 4/4 (secrets, go_build, go_lint, go_tests)
All 5 criteria verified: env shim removed, RunWithEnv delivers per-call env via envp after ExtraEnv, Exec uses it, regression tests confirm no argv leak, and all build/vet/gofmt/test/guard checks pass.

## Summary

Judge Result: OB-GAP-015

Stage tier1: PASS
    ✓ lint: 
  ✓ secrets: [90m12:58AM[0m [32mINF[0m [1mscanned ~5144790 bytes (5.14 MB) in 1.11s[0m
[90m12:58AM[0m [3
  ✓ tests: ?   	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	[no test files]
ok  	github.com/t

Stage tier2: PASS
  COMPLETE
  ✓ grep -rn '/usr/bin/env' internal/ shows no env-carrying shim for Exec (code line gone; doc-comment mentions allowed): grep shows only doc-comments (internal/sandbox/bwrap.go:273, internal/solver/bsandbox_runner.go:75) and test assertions (bsandbox_runner_test.go:195-196); no env-carrying shim code line exists
  ✓ internal/sandbox/bwrap.go exposes RunWithEnv(ctx, name, args, env) delivering per-call env via exec.Cmd.Env (envp), appended after Config.ExtraEnv so per-call wins on duplicates; Run delegates to it with env=nil: bwrap.go:276 RunWithEnv(ctx,name,args,env); cmd.Env=append(os.Environ(), s.cfg.ExtraEnv...) then append(cmd.Env, env...) at line 290 so per-call wins; Run (line 262) delegates to RunWithEnv(ctx,name,args,nil) at line 263
  ✓ internal/solver/bsandbox_runner.go Exec uses RunWithEnv when env is non-empty and no longer builds a /usr/bin/env KEY=VAL argv; stderr-merge-on-error behavior preserved: bsandbox_runner.go Exec (line 76) calls RunWithEnv when len(env)>0 (line 82) else Run; no /usr/bin/env argv built; stderr-merge-on-error preserved at lines 88-96
  ✓ Regression tests (sandbox + solver) drive a fake bwrap binary that dumps its argv and its own env: argv contains NO DEEPSEEK_API_KEY/LLM_API_KEY/sk- values, env DOES contain the keys; empty-env path runs the command directly: bwrap_test.go:468 TestRunWithEnv_EnvNotInArgv and bsandbox_runner_test.go:129 TestExecEnvNotInArgv use makeRecordingBwrap dumping argv+env; assert argv has no DEEPSEEK_API_KEY/LLM_API_KEY/sk-, env has keys, and empty-env subtest runs /bin/echo directly without /usr/bin/env shim
  ✓ go build ./..., go vet ./..., gofmt -l cmd/ internal/ pkg/ sql/ (empty), and go test ./... -short -p 1 -count=1 -timeout 120s all pass; gitreins guard 4/4 PASS: go build exit 0, go vet exit 0, gofmt -l empty, go test ./... -short -p 1 -count=1 -timeout 120s exit 0 (11 packages ok, no FAIL/panic), gitreins guard exit 0 with 'Tier 1 Guards: PASS' 4/4 (secrets, go_build, go_lint, go_tests)
All 5 criteria verified: env shim removed, RunWithEnv delivers per-call env via envp after ExtraEnv, Exec uses it, regression tests confirm no argv leak, and all build/vet/gofmt/test/guard checks pass.

Overall: PASS ✓
