# Verdict: INT-CI-005

**Task:** Fix memcap wrapper lookup under RLIMIT_AS
**Evaluated:** 2026-10-12T18:30:26.705318
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ The oby-memcap wrapper must resolve its child command before applying RLIMIT_AS so Go runtime allocations in os/exec.LookPath cannot fatal under the low cap; TestObyMemcapExec_ChildInheritsCap must still prove inherited 64 MiB soft/hard limit and real 256 MiB mmap ENOMEM; focused/full short tests, build, and vet pass.: RESOLVE-BEFORE-CAP: cmd/oby-memcap/main.go (commit 6ea48894 'fix: resolve exec target before applying RLIMIT_AS in oby-memcap') now does exec.LookPath(os.Args[1]) at line 40, argv:=os.Args[1:] at line 45, envp:=os.Environ() at line 46 — all BEFORE sandbox.ApplyMemLimitMB(mb) at line 53; between the cap (53) and syscall.Exec(path,argv,envp) (58) there is only the exec error branch, so no post-cap Go heap allocation is required. `git diff HEAD~1 -- cmd/oby-memcap/main.go` confirms the reorder (LookPath/Environ previously ran AFTER the cap). TEST PROVES INHERITED CAP + REAL ENOMEM: `go test ./internal/sandbox/ -run TestObyMemcapExec_ChildInheritsCap -count=1 -v` => '--- PASS: TestObyMemcapExec_ChildInheritsCap (0.18s)' / 'ok github.com/totalwindupflightsystems/off-by-one/internal/sandbox 0.178s', with NO SKIP (python3 present, so the ENOMEM arm actually executed). The test builds the real wrapper (`go build -o <tmp>/oby-memcap .../cmd/oby-memcap`), execs it with OBY_MEM_LIMIT_MB=64, asserts /proc/self/limits contains '67108864             67108864' (64 MiB soft==hard inherited across syscall.Exec), then runs inheritedCapProbePy asserting getrlimit==(67108864,67108864) AND a 256 MiB mmap refused with errno 12 (prints ALLOC-REFUSED). Independently reproduced outside the test: `OBY_MEM_LIMIT_MB=64 <wrapper> /bin/sh -c 'grep "Max address space" /proc/self/limits'` => 'Max address space         67108864             67108864             bytes' (exit 0); `OBY_MEM_LIMIT_MB=64 <wrapper> python3 -S -c '<getrlimit+256MiB mmap>'` => 'LIMITS 67108864 67108864' / 'ERRNO 12' (exit 0). GATES: `go build ./...` exit 0 (no output); `go vet ./...` exit 0 (no output); `gofmt -l cmd/ internal/ pkg/ sql/` empty (exit 0); `go test ./... -short -p 1 -count=1 -timeout 300s` exit 0 with all 15 packages 'ok' (cmd/off-by-one 1.453s, internal/api 0.576s, internal/sandbox 1.341s, internal/solver 0.138s, internal/web 2.523s, ...). LSP diagnostics: 0 findings.
The oby-memcap wrapper now resolves LookPath/argv/envp before ApplyMemLimitMB, and TestObyMemcapExec_ChildInheritsCap passes (no skip) proving an inherited 64 MiB soft==hard RLIMIT_AS and a real 256 MiB mmap ENOMEM, with build, vet, gofmt, and the full short suite all green.

## Summary

Judge Result: INT-CI-005

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ The oby-memcap wrapper must resolve its child command before applying RLIMIT_AS so Go runtime allocations in os/exec.LookPath cannot fatal under the low cap; TestObyMemcapExec_ChildInheritsCap must still prove inherited 64 MiB soft/hard limit and real 256 MiB mmap ENOMEM; focused/full short tests, build, and vet pass.: RESOLVE-BEFORE-CAP: cmd/oby-memcap/main.go (commit 6ea48894 'fix: resolve exec target before applying RLIMIT_AS in oby-memcap') now does exec.LookPath(os.Args[1]) at line 40, argv:=os.Args[1:] at line 45, envp:=os.Environ() at line 46 — all BEFORE sandbox.ApplyMemLimitMB(mb) at line 53; between the cap (53) and syscall.Exec(path,argv,envp) (58) there is only the exec error branch, so no post-cap Go heap allocation is required. `git diff HEAD~1 -- cmd/oby-memcap/main.go` confirms the reorder (LookPath/Environ previously ran AFTER the cap). TEST PROVES INHERITED CAP + REAL ENOMEM: `go test ./internal/sandbox/ -run TestObyMemcapExec_ChildInheritsCap -count=1 -v` => '--- PASS: TestObyMemcapExec_ChildInheritsCap (0.18s)' / 'ok github.com/totalwindupflightsystems/off-by-one/internal/sandbox 0.178s', with NO SKIP (python3 present, so the ENOMEM arm actually executed). The test builds the real wrapper (`go build -o <tmp>/oby-memcap .../cmd/oby-memcap`), execs it with OBY_MEM_LIMIT_MB=64, asserts /proc/self/limits contains '67108864             67108864' (64 MiB soft==hard inherited across syscall.Exec), then runs inheritedCapProbePy asserting getrlimit==(67108864,67108864) AND a 256 MiB mmap refused with errno 12 (prints ALLOC-REFUSED). Independently reproduced outside the test: `OBY_MEM_LIMIT_MB=64 <wrapper> /bin/sh -c 'grep "Max address space" /proc/self/limits'` => 'Max address space         67108864             67108864             bytes' (exit 0); `OBY_MEM_LIMIT_MB=64 <wrapper> python3 -S -c '<getrlimit+256MiB mmap>'` => 'LIMITS 67108864 67108864' / 'ERRNO 12' (exit 0). GATES: `go build ./...` exit 0 (no output); `go vet ./...` exit 0 (no output); `gofmt -l cmd/ internal/ pkg/ sql/` empty (exit 0); `go test ./... -short -p 1 -count=1 -timeout 300s` exit 0 with all 15 packages 'ok' (cmd/off-by-one 1.453s, internal/api 0.576s, internal/sandbox 1.341s, internal/solver 0.138s, internal/web 2.523s, ...). LSP diagnostics: 0 findings.
The oby-memcap wrapper now resolves LookPath/argv/envp before ApplyMemLimitMB, and TestObyMemcapExec_ChildInheritsCap passes (no skip) proving an inherited 64 MiB soft==hard RLIMIT_AS and a real 256 MiB mmap ENOMEM, with build, vet, gofmt, and the full short suite all green.

Overall: PASS ✓
