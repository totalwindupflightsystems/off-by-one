# Verdict: OB-GAP-071

**Task:** Fix seed subcommand flag ordering
**Evaluated:** 2026-09-18T12:24:50.850580
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	1.563s
- ✓ **tier2**
  - COMPLETE
  ✓ When invoked as ./off-by-one --db PATH seed, the binary must dispatch seed and honor --db regardless of flag position; add regression coverage; go build and tests pass.: Fix in commit 0ff9d4c: new cmd/off-by-one/dispatch.go seedDispatch() probes argv with a FlagSet built from the live server declarations and finds the subcommand at the first positional arg; main.go:83 dispatches before flag.Parse() and forwards a pre-seed -db in seed spelling. Pre-fix code (git show 0ff9d4c^:cmd/off-by-one/main.go:67) was `os.Args[1] == "seed"`, the exact bug. Real-binary E2E: `/tmp/ob1bin --db /tmp/e2e/out.db seed` -> EXIT=0, log 'seed complete: files=1; classes=1 created ... (db=/tmp/e2e/out.db)', out.db created with problem_classes row (1,'e2e check class'); `--db out2.db --port 18777 seed` -> EXIT=0, 'seed: ignoring server flag(s) given before the subcommand: -port', grep -c 'listening on' = 0 (port never bound); historical `seed -db h.db` -> EXIT=0. Regression coverage in cmd/off-by-one/dispatch_test.go: TestSeedDispatchRecognisesSubcommandForms, TestSeedDispatchLeavesServerStartupAlone, TestSeedDispatchMirrorsServerFlagArity, TestSeedDispatchForwardsDBIntoSeedRun, plus real-binary subprocess tests TestSeedAfterLeadingServerFlagsSeedsAndNeverServes (acceptance), TestSeedSubcommandFirstStillSeeds, TestServerStartupStillParsesFlags. Fresh runs: `go build ./...` exit 0; `go test ./... -short -p 1 -count=1 -timeout 300s` TEST_EXIT=0 with 13 'ok' packages and no FAIL/panic; `go test ./cmd/off-by-one/ -run TestSeed -v -count=1` all PASS (TestSeedAfterLeadingServerFlagsSeedsAndNeverServes 0.57s); `gofmt -l cmd/ internal/ pkg/ sql/` empty; `go vet ./...` exit 0; LSP diagnostics 0.
The seed subcommand now dispatches correctly for `./off-by-one --db PATH seed` (verified with the real binary, corpus loaded into the --db path, port never bound), regression tests cover all flag positions, and go build/vet/gofmt plus the full test suite pass.

## Summary

Judge Result: OB-GAP-071

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	1.563s

Stage tier2: PASS
  COMPLETE
  ✓ When invoked as ./off-by-one --db PATH seed, the binary must dispatch seed and honor --db regardless of flag position; add regression coverage; go build and tests pass.: Fix in commit 0ff9d4c: new cmd/off-by-one/dispatch.go seedDispatch() probes argv with a FlagSet built from the live server declarations and finds the subcommand at the first positional arg; main.go:83 dispatches before flag.Parse() and forwards a pre-seed -db in seed spelling. Pre-fix code (git show 0ff9d4c^:cmd/off-by-one/main.go:67) was `os.Args[1] == "seed"`, the exact bug. Real-binary E2E: `/tmp/ob1bin --db /tmp/e2e/out.db seed` -> EXIT=0, log 'seed complete: files=1; classes=1 created ... (db=/tmp/e2e/out.db)', out.db created with problem_classes row (1,'e2e check class'); `--db out2.db --port 18777 seed` -> EXIT=0, 'seed: ignoring server flag(s) given before the subcommand: -port', grep -c 'listening on' = 0 (port never bound); historical `seed -db h.db` -> EXIT=0. Regression coverage in cmd/off-by-one/dispatch_test.go: TestSeedDispatchRecognisesSubcommandForms, TestSeedDispatchLeavesServerStartupAlone, TestSeedDispatchMirrorsServerFlagArity, TestSeedDispatchForwardsDBIntoSeedRun, plus real-binary subprocess tests TestSeedAfterLeadingServerFlagsSeedsAndNeverServes (acceptance), TestSeedSubcommandFirstStillSeeds, TestServerStartupStillParsesFlags. Fresh runs: `go build ./...` exit 0; `go test ./... -short -p 1 -count=1 -timeout 300s` TEST_EXIT=0 with 13 'ok' packages and no FAIL/panic; `go test ./cmd/off-by-one/ -run TestSeed -v -count=1` all PASS (TestSeedAfterLeadingServerFlagsSeedsAndNeverServes 0.57s); `gofmt -l cmd/ internal/ pkg/ sql/` empty; `go vet ./...` exit 0; LSP diagnostics 0.
The seed subcommand now dispatches correctly for `./off-by-one --db PATH seed` (verified with the real binary, corpus loaded into the --db path, port never bound), regression tests cover all flag positions, and go build/vet/gofmt plus the full test suite pass.

Overall: PASS ✓
