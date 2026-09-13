# Verdict: DF-OFF-BY-ONE-1

**Task:** Document port 8766 collision in Quick Start
**Evaluated:** 2026-09-13T10:50:37.155115
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m5:50AM[0m [32mINF[0m [1mscanned ~10145542 bytes (10.15 MB) in 3.12s[0m
[90m5:50AM[0m [3
- ✓ **tier2**
  - COMPLETE
  ✓ README.md Quick Start section (between '### Quick Start' and '### Build, Test, Lint') mentions the port collision scenario ('address already in use' or equivalent) AND names the escape hatch '--port' or 'OFF_BY_ONE_PORT': README.md:273 sits inside the Quick Start section (starts line 247 '### Quick Start', ends line 279 '### Build, Test, Lint') and reads: '> **Port 8766 already in use?** `./off-by-one` binds `:8766` by default; if another instance (or any other service) already holds it, startup fails with `listen tcp :8766: bind: address already in use`. Run on an alternate port instead: `./off-by-one --port 18766` (or set `OFF_BY_ONE_PORT=18766` in `.env`) — see [Configuration](#configuration).' Both required elements are present: the collision scenario ('address already in use') and BOTH escape hatches ('--port' and 'OFF_BY_ONE_PORT'). The documented flags are real, not invented: cmd/off-by-one/main.go:76 declares `port := flag.Int("port", envInt("OFF_BY_ONE_PORT", 8766), "HTTP listen port")`, confirming the default 8766 and both override mechanisms. No test suite applies to this docs-only criterion.
README.md Quick Start documents the port 8766 'address already in use' collision and names both escape hatches (--port / OFF_BY_ONE_PORT), which are verified to exist in cmd/off-by-one/main.go:76.

## Summary

Judge Result: DF-OFF-BY-ONE-1

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m5:50AM[0m [32mINF[0m [1mscanned ~10145542 bytes (10.15 MB) in 3.12s[0m
[90m5:50AM[0m [3

Stage tier2: PASS
  COMPLETE
  ✓ README.md Quick Start section (between '### Quick Start' and '### Build, Test, Lint') mentions the port collision scenario ('address already in use' or equivalent) AND names the escape hatch '--port' or 'OFF_BY_ONE_PORT': README.md:273 sits inside the Quick Start section (starts line 247 '### Quick Start', ends line 279 '### Build, Test, Lint') and reads: '> **Port 8766 already in use?** `./off-by-one` binds `:8766` by default; if another instance (or any other service) already holds it, startup fails with `listen tcp :8766: bind: address already in use`. Run on an alternate port instead: `./off-by-one --port 18766` (or set `OFF_BY_ONE_PORT=18766` in `.env`) — see [Configuration](#configuration).' Both required elements are present: the collision scenario ('address already in use') and BOTH escape hatches ('--port' and 'OFF_BY_ONE_PORT'). The documented flags are real, not invented: cmd/off-by-one/main.go:76 declares `port := flag.Int("port", envInt("OFF_BY_ONE_PORT", 8766), "HTTP listen port")`, confirming the default 8766 and both override mechanisms. No test suite applies to this docs-only criterion.
README.md Quick Start documents the port 8766 'address already in use' collision and names both escape hatches (--port / OFF_BY_ONE_PORT), which are verified to exist in cmd/off-by-one/main.go:76.

Overall: PASS ✓
