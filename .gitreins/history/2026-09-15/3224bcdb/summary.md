# Verdict: DF-OFF-BY-ONE-3

**Task:** PI_MODEL override is inert end-to-end and README model resolution overstates the shipped server
**Evaluated:** 2026-09-15T10:41:24.291562
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.355s
ok  	github.com/totalwindu
  ✓ secrets: [90m5:40AM[0m [32mINF[0m [1mscanned ~6161766 bytes (6.16 MB) in 1.55s[0m
[90m5:40AM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ PASS: (1) cmd/off-by-one/main.go reads PI_MODEL and passes it as solver.Config.Model (grep -n PI_MODEL cmd/off-by-one/main.go non-empty) with fallback to solver.DefaultModel when unset; (2) a Go test asserts the process env PI_MODEL value reaches the solve config AND is propagated into the sandbox env (ExtraEnv) so scripts/pi-agent sees it verbatim; (3) go build ./... + go vet ./... clean and go test ./... -short -p 1 -count=1 -timeout 120s green; (4) every README model-resolution claim verified true against the shipped code (fix or drop only claims that do not hold).: (1) grep -n PI_MODEL cmd/off-by-one/main.go is non-empty (lines 310-348): resolveSolverModel() returns strings.TrimSpace(getenv("PI_MODEL")) when non-blank else solver.DefaultModel; solverConfigFor() sets Config.Model=resolveSolverModel(getenv) and Config.ExtraEnv=solverExtraEnv(getenv); main() now calls solverConfigFor(os.Getenv,...) instead of hardcoding solver.DefaultModel. (2) cmd/off-by-one/main_test.go:255 TestSolverModelFromProcessEnv uses t.Setenv("PI_MODEL",...) + os.Getenv and asserts cfg.Model==override and cfg.ExtraEnv==["PI_MODEL="+override] (and default/nil when unset); internal/solver/piagent_test.go:799 TestExecutor_Solve_PropagatesExtraEnv asserts the PI_MODEL value reaches the runner Exec env verbatim and that args carry the same model. Chain confirmed: piagent.go:253 env=append(ExtraEnv...) -> handle.Exec -> bsandbox_runner.go:93 RunWithEnv -> bwrap.go:290-291 cmd.Env=append(os.Environ(),ExtraEnv...)+append(env...). (3) go build ./... exit 0; go vet ./... exit 0; go test ./... -short -p 1 -count=1 -timeout 120s exit 0 with all 13 packages 'ok' (cmd/off-by-one 0.308s, internal/solver 0.139s, etc.); targeted runs of TestResolveSolverModel, TestSolverExtraEnv_ForwardsPI_MODELOnly, TestSolverModelFromProcessEnv, TestExecutor_Solve_PropagatesExtraEnv, TestExecutor_Solve_NoPIModelEnvWhenUnset all PASS. (4) README.md:233/235 claims verified against shipped code: 'read by the server at startup and passed as --model and as PI_MODEL in the sandbox environment' matches main.go solverConfigFor + piagent.go:250 (--model e.cfg.Model) + ExtraEnv plumbing; 'Unset or blank falls back to deepseek-v4-flash' matches solver.DefaultModel="deepseek-v4-flash" (piagent.go:28); 'empty or whitespace-only counts as unset' matches strings.TrimSpace; 'value wins verbatim and bypasses the mapping' matches scripts/pi-agent:107-110 resolveModel; 'derives the provider from that id's prefix' matches scripts/pi-agent:279-281; 'failing fast if no usable key for that provider is present' matches scripts/pi-agent:295-302. No README model-resolution claim was found false.


## Summary

Judge Result: DF-OFF-BY-ONE-3

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.355s
ok  	github.com/totalwindu
  ✓ secrets: [90m5:40AM[0m [32mINF[0m [1mscanned ~6161766 bytes (6.16 MB) in 1.55s[0m
[90m5:40AM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ PASS: (1) cmd/off-by-one/main.go reads PI_MODEL and passes it as solver.Config.Model (grep -n PI_MODEL cmd/off-by-one/main.go non-empty) with fallback to solver.DefaultModel when unset; (2) a Go test asserts the process env PI_MODEL value reaches the solve config AND is propagated into the sandbox env (ExtraEnv) so scripts/pi-agent sees it verbatim; (3) go build ./... + go vet ./... clean and go test ./... -short -p 1 -count=1 -timeout 120s green; (4) every README model-resolution claim verified true against the shipped code (fix or drop only claims that do not hold).: (1) grep -n PI_MODEL cmd/off-by-one/main.go is non-empty (lines 310-348): resolveSolverModel() returns strings.TrimSpace(getenv("PI_MODEL")) when non-blank else solver.DefaultModel; solverConfigFor() sets Config.Model=resolveSolverModel(getenv) and Config.ExtraEnv=solverExtraEnv(getenv); main() now calls solverConfigFor(os.Getenv,...) instead of hardcoding solver.DefaultModel. (2) cmd/off-by-one/main_test.go:255 TestSolverModelFromProcessEnv uses t.Setenv("PI_MODEL",...) + os.Getenv and asserts cfg.Model==override and cfg.ExtraEnv==["PI_MODEL="+override] (and default/nil when unset); internal/solver/piagent_test.go:799 TestExecutor_Solve_PropagatesExtraEnv asserts the PI_MODEL value reaches the runner Exec env verbatim and that args carry the same model. Chain confirmed: piagent.go:253 env=append(ExtraEnv...) -> handle.Exec -> bsandbox_runner.go:93 RunWithEnv -> bwrap.go:290-291 cmd.Env=append(os.Environ(),ExtraEnv...)+append(env...). (3) go build ./... exit 0; go vet ./... exit 0; go test ./... -short -p 1 -count=1 -timeout 120s exit 0 with all 13 packages 'ok' (cmd/off-by-one 0.308s, internal/solver 0.139s, etc.); targeted runs of TestResolveSolverModel, TestSolverExtraEnv_ForwardsPI_MODELOnly, TestSolverModelFromProcessEnv, TestExecutor_Solve_PropagatesExtraEnv, TestExecutor_Solve_NoPIModelEnvWhenUnset all PASS. (4) README.md:233/235 claims verified against shipped code: 'read by the server at startup and passed as --model and as PI_MODEL in the sandbox environment' matches main.go solverConfigFor + piagent.go:250 (--model e.cfg.Model) + ExtraEnv plumbing; 'Unset or blank falls back to deepseek-v4-flash' matches solver.DefaultModel="deepseek-v4-flash" (piagent.go:28); 'empty or whitespace-only counts as unset' matches strings.TrimSpace; 'value wins verbatim and bypasses the mapping' matches scripts/pi-agent:107-110 resolveModel; 'derives the provider from that id's prefix' matches scripts/pi-agent:279-281; 'failing fast if no usable key for that provider is present' matches scripts/pi-agent:295-302. No README model-resolution claim was found false.


Overall: PASS ✓
