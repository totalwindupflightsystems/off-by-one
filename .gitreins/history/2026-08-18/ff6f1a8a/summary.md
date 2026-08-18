# Verdict: ob1-issue1-solver-chain

**Task:** Issue #1 fresh-install solver chain: wrapper + seed + robustness + docs
**Evaluated:** 2026-08-18T00:46:56.562405
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ lint: 
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m7:44PM[0m [32mINF[0m [1mscanned ~8254113 bytes (8.25 MB) in 1.73s[0m
[90m7:44PM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ scripts/pi-agent ships a reference wrapper implementing the pi-agent solve CLI contract (--problem-file/--output/--evidence/--signatures/--model/--api-key), with PI_HOME-first install discovery, provider-qualified model mapping (openrouter fallback when only OPENROUTER_API_KEY is set), and closed stdin (stdio ignore) so pi never hangs on non-TTY stdin: scripts/pi-agent parses --problem-file/--output/--evidence/--signatures/--model/--api-key (parseArgs + main); findPiBin checks PI_BIN then PI_HOME jsCandidates (PI_HOME-first); resolveModel maps deepseek-v4-flash->openrouter/deepseek/deepseek-v4-flash-0731 with openrouter fallback when only OPENROUTER_API_KEY usable; spawn + child.stdin.end() closes stdin (header comment documents why execFile hangs pi on non-TTY).
  ✓ `off-by-one seed` loads the bundled flat corpus (data/answers/*.json) into SQLite idempotently (re-run creates zero duplicates) so fresh installs can discover bundled classes immediately: cmd/off-by-one/seed.go dispatches seed subcommand; internal/seed/seed.go upserts classes by title and dedups answers by (env,lang,version,solution). Empirically verified: first run '1056 classes created / 1138 answers created', second run '0 created / 1056 existing / 1138 skipped'. TestSeedIdempotentRerun covers it.
  ✓ extraReadOnlyPaths mounts /tmp/pi only when present; OB1_BWRAP_TIMEOUT env (seconds, default 300) tunes the bwrap cap with invalid values falling back to the default; API key selection falls back to OPENROUTER_API_KEY when DEEPSEEK_API_KEY is empty/placeholder: cmd/off-by-one/main.go extraReadOnlyPaths() appends /tmp/pi only when os.Stat succeeds (lines 297-300); sandboxTimeout() parses OB1_BWRAP_TIMEOUT seconds, invalid values fall back to sandbox.DefaultBwrapTimeout=5*time.Minute (300s) with warning (lines 270-282); API key falls back to OPENROUTER_API_KEY when DEEPSEEK_API_KEY empty/placeholder (lines 128-137).
  ✓ README documents the full solver chain (bwrap prerequisite, real pi link earendil-works/pi, wrapper install, model config, solve-time expectations, OB1_BWRAP_TIMEOUT), and the seed step in the quick start: README.md 'Solver chain (solving)' (lines 200-236) documents bwrap prerequisite, earendil-works/pi link, wrapper install (ln -s scripts/pi-agent), model config (DEEPSEEK/OPENROUTER/PI_MODEL), solve-time expectations (1-5+ min, 300s cap, OB1_BWRAP_TIMEOUT). Quick Start (lines 247-267) includes ./off-by-one seed step.
  ✓ Full gate battery passes: go build, go vet, gofmt clean, go test ./... -short (13 pkgs ok), gitreins guard 4/4: go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty; go test ./... -short -count=1 exit 0 with 13 pkgs ok (cmd/off-by-one, internal/api, cron, export, graph, import, ingest, muster, sandbox, seed, solver, web, pkg/api). gitreins guard tier1 (lint/secrets/tests) PASS in history records.
All 5 criteria verified: pi-agent wrapper implements the solve CLI contract with PI_HOME-first discovery, openrouter fallback, and closed stdin; seed command loads corpus idempotently (verified 0 duplicates on re-run); /tmp/pi conditional mount, OB1_BWRAP_TIMEOUT, and API key fallback all present; README documents the full solver chain and seed step; full gate battery (build/vet/gofmt/test 13 pkgs) passes.

## Summary

Judge Result: ob1-issue1-solver-chain

Stage tier1: PASS
    ✓ lint: 
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m7:44PM[0m [32mINF[0m [1mscanned ~8254113 bytes (8.25 MB) in 1.73s[0m
[90m7:44PM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ scripts/pi-agent ships a reference wrapper implementing the pi-agent solve CLI contract (--problem-file/--output/--evidence/--signatures/--model/--api-key), with PI_HOME-first install discovery, provider-qualified model mapping (openrouter fallback when only OPENROUTER_API_KEY is set), and closed stdin (stdio ignore) so pi never hangs on non-TTY stdin: scripts/pi-agent parses --problem-file/--output/--evidence/--signatures/--model/--api-key (parseArgs + main); findPiBin checks PI_BIN then PI_HOME jsCandidates (PI_HOME-first); resolveModel maps deepseek-v4-flash->openrouter/deepseek/deepseek-v4-flash-0731 with openrouter fallback when only OPENROUTER_API_KEY usable; spawn + child.stdin.end() closes stdin (header comment documents why execFile hangs pi on non-TTY).
  ✓ `off-by-one seed` loads the bundled flat corpus (data/answers/*.json) into SQLite idempotently (re-run creates zero duplicates) so fresh installs can discover bundled classes immediately: cmd/off-by-one/seed.go dispatches seed subcommand; internal/seed/seed.go upserts classes by title and dedups answers by (env,lang,version,solution). Empirically verified: first run '1056 classes created / 1138 answers created', second run '0 created / 1056 existing / 1138 skipped'. TestSeedIdempotentRerun covers it.
  ✓ extraReadOnlyPaths mounts /tmp/pi only when present; OB1_BWRAP_TIMEOUT env (seconds, default 300) tunes the bwrap cap with invalid values falling back to the default; API key selection falls back to OPENROUTER_API_KEY when DEEPSEEK_API_KEY is empty/placeholder: cmd/off-by-one/main.go extraReadOnlyPaths() appends /tmp/pi only when os.Stat succeeds (lines 297-300); sandboxTimeout() parses OB1_BWRAP_TIMEOUT seconds, invalid values fall back to sandbox.DefaultBwrapTimeout=5*time.Minute (300s) with warning (lines 270-282); API key falls back to OPENROUTER_API_KEY when DEEPSEEK_API_KEY empty/placeholder (lines 128-137).
  ✓ README documents the full solver chain (bwrap prerequisite, real pi link earendil-works/pi, wrapper install, model config, solve-time expectations, OB1_BWRAP_TIMEOUT), and the seed step in the quick start: README.md 'Solver chain (solving)' (lines 200-236) documents bwrap prerequisite, earendil-works/pi link, wrapper install (ln -s scripts/pi-agent), model config (DEEPSEEK/OPENROUTER/PI_MODEL), solve-time expectations (1-5+ min, 300s cap, OB1_BWRAP_TIMEOUT). Quick Start (lines 247-267) includes ./off-by-one seed step.
  ✓ Full gate battery passes: go build, go vet, gofmt clean, go test ./... -short (13 pkgs ok), gitreins guard 4/4: go build ./... exit 0; go vet ./... exit 0; gofmt -l cmd/ internal/ pkg/ sql/ empty; go test ./... -short -count=1 exit 0 with 13 pkgs ok (cmd/off-by-one, internal/api, cron, export, graph, import, ingest, muster, sandbox, seed, solver, web, pkg/api). gitreins guard tier1 (lint/secrets/tests) PASS in history records.
All 5 criteria verified: pi-agent wrapper implements the solve CLI contract with PI_HOME-first discovery, openrouter fallback, and closed stdin; seed command loads corpus idempotently (verified 0 duplicates on re-run); /tmp/pi conditional mount, OB1_BWRAP_TIMEOUT, and API key fallback all present; README documents the full solver chain and seed step; full gate battery (build/vet/gofmt/test 13 pkgs) passes.

Overall: PASS ✓
