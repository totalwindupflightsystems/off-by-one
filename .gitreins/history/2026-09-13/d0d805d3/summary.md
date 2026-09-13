# Verdict: OB-GAP-059

**Task:** Document configurable bwrap solve timeout
**Evaluated:** 2026-09-13T17:06:06.704354
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m12:05PM[0m [32mINF[0m [1mscanned ~10162389 bytes (10.16 MB) in 2.68s[0m
[90m12:05PM[0m 
- ✓ **tier2**
  - COMPLETE
  ✓ All four living docs (README.md, docs/integration.md, docs/api-reference.md, skills/off-by-one-usage/SKILL.md) contain OB1_BWRAP_TIMEOUT with the 300-second default and positive-integer seconds semantics: README.md:174 table row (default `300`, 'positive integer; unset or invalid values fall back to the 300s default with a warning') and README.md:237 prose; docs/integration.md:374 table row (default `300`, 'positive integer') and :380 'Two timeouts, two layers' prose; docs/api-reference.md:597 table (default `300` seconds) and :600 'takes a positive integer number of seconds ... falls back to the 300-second default'; skills/off-by-one-usage/SKILL.md:79 and :113 ('positive integer seconds, default `300`'). Code matches: cmd/off-by-one/main.go:298-308 sandboxTimeout() requires secs>0 else warns and returns sandbox.DefaultBwrapTimeout = 5*time.Minute (internal/sandbox/bwrap.go:60).
  ✓ README.md and docs/integration.md environment-variable tables document OB1_BWRAP_TIMEOUT; docs/api-reference.md documents the operator-facing timeout configuration: README.md:160 '### Environment Variables' table contains OB1_BWRAP_TIMEOUT at line 174; docs/integration.md:360 '### Environment variables' table contains OB1_BWRAP_TIMEOUT at line 374. docs/api-reference.md has a dedicated '## Solve timeouts' section (~line 592) with a two-timeout table (OB1_BWRAP_TIMEOUT default 300s vs OFF_BY_ONE_SOLVE_TIMEOUT 30m), semantics, and the operator example `OB1_BWRAP_TIMEOUT=900 ./off-by-one`.
  ✓ The usage skill no longer tells agents that exact-300s kills are simply normal/do-not-chase; it explains the configurable cap and recommends increasing it for legitimate long solves while investigating unexpected repeats: git diff HEAD~1 on skills/off-by-one-usage/SKILL.md removes '~25-45% of fleet solves fail at the exact-300s bwrap cap (signal: killed) — normal, retry or discover later' and old pitfall 4 'is normal fleet behavior, not a regression — don't chase it'. Replaced with: raise OB1_BWRAP_TIMEOUT (positive integer seconds, e.g. 900 for 15m) for legitimate long solves, and 'Do NOT just re-submit when cap kills are unexpected or repeat — check host resource pressure, a malformed tool flow inside the sandbox, and whether the configured cap is still too low'. grep for "don't chase|normal fleet|not a regression" returns no matches (exit=1). Test evidence: `go test ./... -short -count=1 -p 1 -timeout 180s` exit_code 0, all 13 packages ok, including cmd/off-by-one (TestSandboxTimeout at main_test.go:116).
All three documentation criteria are met with matching code semantics (300s default, positive-integer seconds) and the full test suite passes (exit 0, 13 packages ok).

## Summary

Judge Result: OB-GAP-059

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m12:05PM[0m [32mINF[0m [1mscanned ~10162389 bytes (10.16 MB) in 2.68s[0m
[90m12:05PM[0m 

Stage tier2: PASS
  COMPLETE
  ✓ All four living docs (README.md, docs/integration.md, docs/api-reference.md, skills/off-by-one-usage/SKILL.md) contain OB1_BWRAP_TIMEOUT with the 300-second default and positive-integer seconds semantics: README.md:174 table row (default `300`, 'positive integer; unset or invalid values fall back to the 300s default with a warning') and README.md:237 prose; docs/integration.md:374 table row (default `300`, 'positive integer') and :380 'Two timeouts, two layers' prose; docs/api-reference.md:597 table (default `300` seconds) and :600 'takes a positive integer number of seconds ... falls back to the 300-second default'; skills/off-by-one-usage/SKILL.md:79 and :113 ('positive integer seconds, default `300`'). Code matches: cmd/off-by-one/main.go:298-308 sandboxTimeout() requires secs>0 else warns and returns sandbox.DefaultBwrapTimeout = 5*time.Minute (internal/sandbox/bwrap.go:60).
  ✓ README.md and docs/integration.md environment-variable tables document OB1_BWRAP_TIMEOUT; docs/api-reference.md documents the operator-facing timeout configuration: README.md:160 '### Environment Variables' table contains OB1_BWRAP_TIMEOUT at line 174; docs/integration.md:360 '### Environment variables' table contains OB1_BWRAP_TIMEOUT at line 374. docs/api-reference.md has a dedicated '## Solve timeouts' section (~line 592) with a two-timeout table (OB1_BWRAP_TIMEOUT default 300s vs OFF_BY_ONE_SOLVE_TIMEOUT 30m), semantics, and the operator example `OB1_BWRAP_TIMEOUT=900 ./off-by-one`.
  ✓ The usage skill no longer tells agents that exact-300s kills are simply normal/do-not-chase; it explains the configurable cap and recommends increasing it for legitimate long solves while investigating unexpected repeats: git diff HEAD~1 on skills/off-by-one-usage/SKILL.md removes '~25-45% of fleet solves fail at the exact-300s bwrap cap (signal: killed) — normal, retry or discover later' and old pitfall 4 'is normal fleet behavior, not a regression — don't chase it'. Replaced with: raise OB1_BWRAP_TIMEOUT (positive integer seconds, e.g. 900 for 15m) for legitimate long solves, and 'Do NOT just re-submit when cap kills are unexpected or repeat — check host resource pressure, a malformed tool flow inside the sandbox, and whether the configured cap is still too low'. grep for "don't chase|normal fleet|not a regression" returns no matches (exit=1). Test evidence: `go test ./... -short -count=1 -p 1 -timeout 180s` exit_code 0, all 13 packages ok, including cmd/off-by-one (TestSandboxTimeout at main_test.go:116).
All three documentation criteria are met with matching code semantics (300s default, positive-integer seconds) and the full test suite passes (exit 0, 13 packages ok).

Overall: PASS ✓
