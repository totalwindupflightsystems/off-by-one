# Verdict: DF-OFF-BY-ONE-3

**Task:** Make seed corpus path independent of caller CWD
**Evaluated:** 2026-09-14T12:38:13.353805
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.301s
ok  	github.com/totalwindu
  ✓ secrets: [90m7:36AM[0m [32mINF[0m [1mscanned ~6087443 bytes (6.09 MB) in 981ms[0m
[90m7:36AM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ With no -dir, a repo-built off-by-one binary invoked from outside the repo resolves its sibling data directory and seeds successfully; explicit -dir still wins; missing corpus errors list attempted paths; focused and full Go tests pass: cmd/off-by-one/seed.go adds seedDirCandidates (cwd/data, exeDir/data, exeDir/../data; cleaned+deduped) and resolveSeedDir (explicit -dir wins verbatim with no fallback; else first candidate holding answers/; total miss lists every attempted path + advises -dir); seedRun resolves before graph.Open so no misleading empty DB. E2E: repo-built binary at repo root invoked from /tmp/ob1-eval/foreign with no -dir -> 'seed complete: files=1528; classes=1528 created / 0 existing; answers=1606 created / 0 skipped' (resolved sibling /home/kara/off-by-one/data). Explicit -dir: '-dir /home/kara/off-by-one/data' seeds 1528 files; invalid '-dir /tmp/nonexistent-corpus' -> 'explicit -dir ... has no answers/ subdirectory (looked for .../answers); pass -dir DIR ... no fallback is attempted when -dir is given', no DB created. Missing corpus from empty dir -> exit=1, 'corpus directory not found: no answers/ subdirectory in any of /tmp/ob1-eval/empty/data, /tmp/ob1-eval/data, /tmp/data; pass -dir DIR', no DB created. bin/ layout (exe in app/bin, data in app/data) also seeds 1528 files; repo-local run unchanged. Tests: 'go test ./cmd/off-by-one/ -run Seed|ResolveSeedDir -count=1 -v' -> PASS (ExplicitWins, PrefersCWD, ExecutableFallback/next_to_executable+bin_layout, ErrorListsAttempts, CleansAndDedupes, SeedRunResolvesCorpusFromExecutableDir, SeedRunCWDCorpusBeatsExecutableCorpus, SeedRunMissingCorpusCreatesNoDB, SeedRunInvalidExplicitDirDoesNotFallBack, RunSeedHonorsEnvVar, RunSeedDBFlagOverridesEnvVar) ok 0.295s. Full: 'go test ./... -short -p 1 -count=1 -timeout 180s' -> all 13 pkgs ok. go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty; gitreins guard 4/4 PASS; LSP diagnostics 0.
Seed corpus resolution is now CWD-independent: a repo-built binary run from outside the repo finds its sibling data/ and seeds 1528 files, explicit -dir wins without fallback, missing-corpus errors list all attempted paths and create no DB, and focused plus full Go tests, build, vet, gofmt and gitreins guard all pass.

## Summary

Judge Result: DF-OFF-BY-ONE-3

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	0.301s
ok  	github.com/totalwindu
  ✓ secrets: [90m7:36AM[0m [32mINF[0m [1mscanned ~6087443 bytes (6.09 MB) in 981ms[0m
[90m7:36AM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ With no -dir, a repo-built off-by-one binary invoked from outside the repo resolves its sibling data directory and seeds successfully; explicit -dir still wins; missing corpus errors list attempted paths; focused and full Go tests pass: cmd/off-by-one/seed.go adds seedDirCandidates (cwd/data, exeDir/data, exeDir/../data; cleaned+deduped) and resolveSeedDir (explicit -dir wins verbatim with no fallback; else first candidate holding answers/; total miss lists every attempted path + advises -dir); seedRun resolves before graph.Open so no misleading empty DB. E2E: repo-built binary at repo root invoked from /tmp/ob1-eval/foreign with no -dir -> 'seed complete: files=1528; classes=1528 created / 0 existing; answers=1606 created / 0 skipped' (resolved sibling /home/kara/off-by-one/data). Explicit -dir: '-dir /home/kara/off-by-one/data' seeds 1528 files; invalid '-dir /tmp/nonexistent-corpus' -> 'explicit -dir ... has no answers/ subdirectory (looked for .../answers); pass -dir DIR ... no fallback is attempted when -dir is given', no DB created. Missing corpus from empty dir -> exit=1, 'corpus directory not found: no answers/ subdirectory in any of /tmp/ob1-eval/empty/data, /tmp/ob1-eval/data, /tmp/data; pass -dir DIR', no DB created. bin/ layout (exe in app/bin, data in app/data) also seeds 1528 files; repo-local run unchanged. Tests: 'go test ./cmd/off-by-one/ -run Seed|ResolveSeedDir -count=1 -v' -> PASS (ExplicitWins, PrefersCWD, ExecutableFallback/next_to_executable+bin_layout, ErrorListsAttempts, CleansAndDedupes, SeedRunResolvesCorpusFromExecutableDir, SeedRunCWDCorpusBeatsExecutableCorpus, SeedRunMissingCorpusCreatesNoDB, SeedRunInvalidExplicitDirDoesNotFallBack, RunSeedHonorsEnvVar, RunSeedDBFlagOverridesEnvVar) ok 0.295s. Full: 'go test ./... -short -p 1 -count=1 -timeout 180s' -> all 13 pkgs ok. go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty; gitreins guard 4/4 PASS; LSP diagnostics 0.
Seed corpus resolution is now CWD-independent: a repo-built binary run from outside the repo finds its sibling data/ and seeds 1528 files, explicit -dir wins without fallback, missing-corpus errors list all attempted paths and create no DB, and focused plus full Go tests, build, vet, gofmt and gitreins guard all pass.

Overall: PASS ✓
