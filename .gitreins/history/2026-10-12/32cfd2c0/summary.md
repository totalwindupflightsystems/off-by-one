# Verdict: INT-CI-005

**Task:** Fix memcap wrapper lookup under RLIMIT_AS
**Evaluated:** 2026-10-12T18:30:57.439001
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ The oby-memcap wrapper must resolve its child command before applying RLIMIT_AS so Go runtime allocations in os/exec.LookPath cannot fatal under the low cap; TestObyMemcapExec_ChildInheritsCap must still prove inherited 64 MiB soft/hard limit and real 256 MiB mmap ENOMEM; focused/full short tests, build, and vet pass.: ORDERING: cmd/oby-memcap/main.go resolves exec.LookPath (line 40), argv (line 45) and os.Environ() (line 46) BEFORE sandbox.ApplyMemLimitMB (line 53); only the exec error branch sits between setrlimit and syscall.Exec (line 58), so no post-cap Go heap allocation is required (rationale documented lines 30-39). TEST: internal/sandbox/memcap_test.go TestObyMemcapExec_ChildInheritsCap builds the real wrapper, runs it with OBY_MEM_LIMIT_MB=64, and (a) asserts /proc/self/limits shows '67108864             67108864' (soft==hard 64 MiB) and (b) adds an enforcement arm (lines 344-366) exec'ing python3 -S with inheritedCapProbePy, which asserts getrlimit==(67108864,67108864) AND a 256 MiB mmap refused with errno 12 (ENOMEM), requiring 'ALLOC-REFUSED'. python3 is on PATH so the arm RAN (no skip). COMMANDS (fresh, -count=1): 'go build ./...' exit 0; 'go vet ./...' exit 0; 'gofmt -l cmd/ internal/ pkg/ sql/' empty exit 0; 'go test ./internal/sandbox/ -run TestObyMemcapExec_ChildInheritsCap -count=1 -v' -> '--- PASS: TestObyMemcapExec_ChildInheritsCap (0.19s)' / 'ok ... 0.208s' exit 0; 'go test ./... -short -count=1 -p 1 -timeout 300s' -> EXIT=0, 18 packages ok, zero FAIL/panic. Independent repro: 'OBY_MEM_LIMIT_MB=64 /tmp/oby-memcap python3 -S -c ...' printed 'limits 67108864 67108864' and 'errno 12' (exit 0); negative control with no cap printed 'MAPPED-OK', proving the ENOMEM is cap-caused. LSP diagnostics: 0 findings.
The oby-memcap wrapper now resolves LookPath/argv/envp before applying RLIMIT_AS, and TestObyMemcapExec_ChildInheritsCap proves the inherited 64 MiB soft/hard limit plus a real 256 MiB mmap ENOMEM, with build, vet, gofmt, focused and full short tests all passing.

## Summary

Judge Result: INT-CI-005

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ The oby-memcap wrapper must resolve its child command before applying RLIMIT_AS so Go runtime allocations in os/exec.LookPath cannot fatal under the low cap; TestObyMemcapExec_ChildInheritsCap must still prove inherited 64 MiB soft/hard limit and real 256 MiB mmap ENOMEM; focused/full short tests, build, and vet pass.: ORDERING: cmd/oby-memcap/main.go resolves exec.LookPath (line 40), argv (line 45) and os.Environ() (line 46) BEFORE sandbox.ApplyMemLimitMB (line 53); only the exec error branch sits between setrlimit and syscall.Exec (line 58), so no post-cap Go heap allocation is required (rationale documented lines 30-39). TEST: internal/sandbox/memcap_test.go TestObyMemcapExec_ChildInheritsCap builds the real wrapper, runs it with OBY_MEM_LIMIT_MB=64, and (a) asserts /proc/self/limits shows '67108864             67108864' (soft==hard 64 MiB) and (b) adds an enforcement arm (lines 344-366) exec'ing python3 -S with inheritedCapProbePy, which asserts getrlimit==(67108864,67108864) AND a 256 MiB mmap refused with errno 12 (ENOMEM), requiring 'ALLOC-REFUSED'. python3 is on PATH so the arm RAN (no skip). COMMANDS (fresh, -count=1): 'go build ./...' exit 0; 'go vet ./...' exit 0; 'gofmt -l cmd/ internal/ pkg/ sql/' empty exit 0; 'go test ./internal/sandbox/ -run TestObyMemcapExec_ChildInheritsCap -count=1 -v' -> '--- PASS: TestObyMemcapExec_ChildInheritsCap (0.19s)' / 'ok ... 0.208s' exit 0; 'go test ./... -short -count=1 -p 1 -timeout 300s' -> EXIT=0, 18 packages ok, zero FAIL/panic. Independent repro: 'OBY_MEM_LIMIT_MB=64 /tmp/oby-memcap python3 -S -c ...' printed 'limits 67108864 67108864' and 'errno 12' (exit 0); negative control with no cap printed 'MAPPED-OK', proving the ENOMEM is cap-caused. LSP diagnostics: 0 findings.
The oby-memcap wrapper now resolves LookPath/argv/envp before applying RLIMIT_AS, and TestObyMemcapExec_ChildInheritsCap proves the inherited 64 MiB soft/hard limit plus a real 256 MiB mmap ENOMEM, with build, vet, gofmt, focused and full short tests all passing.

Overall: PASS ✓
