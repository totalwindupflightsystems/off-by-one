# Verdict: OB-GAP-035

**Task:** Sandbox tool resolution drops symlink-resolved tools whose realpath is unmounted (SBOX-002 never-fails)
**Evaluated:** 2026-08-14T23:34:23.641158
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ lint: 
  ✓ secrets: [90m6:33PM[0m [32mINF[0m [1mscanned ~9330079 bytes (9.33 MB) in 2.22s[0m
[90m6:33PM[0m [33m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.011s
ok  	github.com/totalwindu
- ✓ **tier2**
  - COMPLETE
  ✓ grep -n EvalSymlinks internal/sandbox/tools.go returns >= 1: grep -n EvalSymlinks internal/sandbox/tools.go returns lines 22, 111, 122, 129 (>=1); filepath.EvalSymlinks used in lookPathUsable at line 129
  ✓ internal/solver/bsandbox_runner.go ResolveTools call passes full mount set (DefaultReadOnlyPaths + exec.ExtraReadOnlyPaths): bsandbox_runner.go line 44: sandbox.ResolveTools(oc.requiredTools, mountSet) where mountSet is built (lines 38-42) from sandbox.DefaultReadOnlyPaths + r.exec.ExtraReadOnlyPaths (r.exec is *sandbox.Executor which has ExtraReadOnlyPaths at bwrap.go:56)
  ✓ New unit tests: symlink-to-outside-path tool lands in missing (not resolved); symlink-to-covered-path deduped; all resolver tests pass: tools_test.go adds TestResolveTools_SymlinkOutsideMountSet (outside-path tool -> missing, not resolved), TestResolveTools_SymlinkTargetCovered (covered symlink deduped, neither resolved nor missing), TestResolveTools_ExecutorExtrasCoverSymlink. go test ./internal/sandbox/ -run 'TestResolveTools|TestIsPathCovered' -count=1 -v -> all 12 PASS
  ✓ go test -short ./... -count=1 -p 1 -timeout 120s passes; go build ./...; go vet ./...; gofmt -l cmd/ internal/ pkg/ sql/ empty: go test -short ./... -count=1 -p 1 -timeout 120s -> all packages ok; go build ./... -> BUILD OK; go vet ./... -> VET OK; gofmt -l cmd/ internal/ pkg/ sql/ -> empty (clean)
All four OB-GAP-035 criteria verified: EvalSymlinks present in tools.go, ResolveTools passes full mount set (defaults + executor extras), new symlink regression tests pass, and full test/build/vet/gofmt suite is clean.

## Summary

Judge Result: OB-GAP-035

Stage tier1: PASS
    ✓ lint: 
  ✓ secrets: [90m6:33PM[0m [32mINF[0m [1mscanned ~9330079 bytes (9.33 MB) in 2.22s[0m
[90m6:33PM[0m [33m
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.011s
ok  	github.com/totalwindu

Stage tier2: PASS
  COMPLETE
  ✓ grep -n EvalSymlinks internal/sandbox/tools.go returns >= 1: grep -n EvalSymlinks internal/sandbox/tools.go returns lines 22, 111, 122, 129 (>=1); filepath.EvalSymlinks used in lookPathUsable at line 129
  ✓ internal/solver/bsandbox_runner.go ResolveTools call passes full mount set (DefaultReadOnlyPaths + exec.ExtraReadOnlyPaths): bsandbox_runner.go line 44: sandbox.ResolveTools(oc.requiredTools, mountSet) where mountSet is built (lines 38-42) from sandbox.DefaultReadOnlyPaths + r.exec.ExtraReadOnlyPaths (r.exec is *sandbox.Executor which has ExtraReadOnlyPaths at bwrap.go:56)
  ✓ New unit tests: symlink-to-outside-path tool lands in missing (not resolved); symlink-to-covered-path deduped; all resolver tests pass: tools_test.go adds TestResolveTools_SymlinkOutsideMountSet (outside-path tool -> missing, not resolved), TestResolveTools_SymlinkTargetCovered (covered symlink deduped, neither resolved nor missing), TestResolveTools_ExecutorExtrasCoverSymlink. go test ./internal/sandbox/ -run 'TestResolveTools|TestIsPathCovered' -count=1 -v -> all 12 PASS
  ✓ go test -short ./... -count=1 -p 1 -timeout 120s passes; go build ./...; go vet ./...; gofmt -l cmd/ internal/ pkg/ sql/ empty: go test -short ./... -count=1 -p 1 -timeout 120s -> all packages ok; go build ./... -> BUILD OK; go vet ./... -> VET OK; gofmt -l cmd/ internal/ pkg/ sql/ -> empty (clean)
All four OB-GAP-035 criteria verified: EvalSymlinks present in tools.go, ResolveTools passes full mount set (defaults + executor extras), new symlink regression tests pass, and full test/build/vet/gofmt suite is clean.

Overall: PASS ✓
