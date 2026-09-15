# Verdict: QA-OFF-BY-ONE-9

**Task:** Battery harness capacity preflight
**Evaluated:** 2026-09-15T17:16:26.083525
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m12:13PM[0m [32mINF[0m [1mscanned ~6186121 bytes (6.19 MB) in 1.01s[0m
[90m12:13PM[0m [3
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
- ✓ **tier2**
  - COMPLETE
  ✓ bunker-qa.sh run_battery refuses to spawn when the target server's agent pool is full (parsed from 'bunker status' Agents: N/MAX), records exactly one FAIL evidence row, and exits rc=2 before any spawn attempt; a simulated full pool proves rc=2 with the PREFLIGHT FAIL line and no spawn log lines; a server with free capacity proceeds unchanged: /home/kara/.hermes/scripts/bunker-qa.sh: parse_bunker_capacity() (~L68-90) parses 'Agents: N/MAX' scoped to the target server; bunker_capacity() (~L95-115) probes `bunker status --server` with `bunker list` fallback; run() capacity preflight (~L336-364) at L355 `elif [ "$cap_used" -ge "$cap_max" ]` writes ONE FAIL evidence row, prints 'PREFLIGHT FAIL: bunker capacity exhausted on <srv> (N/M agents) — refusing to spawn (deterministic capacity error)' to stderr (L358), and `exit 2` (L359) before spawn_agent() is called. LIVE SIMULATION (fake bunker+ssh in PATH, temp HOME with ~/.bunker/config.yaml): (1) full pool Agents: 8/8 -> RC=2, stderr PREFLIGHT FAIL line present, evidence.jsonl contains EXACTLY ONE line {"cell":"run_battery","status":"FAIL","detail":"bunker capacity exhausted on bunker-las-02 (8/8 agents) — pool full, no spawn attempted (capacity preflight)"}, spawn.log ABSENT (no spawn attempted); (2) free capacity 2/8 -> no PREFLIGHT FAIL, spawn attempted (SPAWN-CALLED logged, agent=abcdef123456 assigned) i.e. proceeds unchanged; (3) 7/8 -> spawn attempted; (4) unparseable status -> WARN + proceeds (does not wedge); (5) parse_bunker_capacity unit checks: 8/8->'8 8', 2/8->'2 8', multi-section scopes to target (ours full->'8 8', ours free->'3 8'), unparseable/empty->''. `bash -n` reports SYNTAX OK; no LSP diagnostics.
bunker-qa.sh run_battery capacity preflight is fully implemented and proven by live simulation: full pool yields rc=2 with the PREFLIGHT FAIL line, exactly one FAIL evidence row, and zero spawn attempts, while free-capacity servers proceed unchanged.

## Summary

Judge Result: QA-OFF-BY-ONE-9

Stage tier1: PASS
    ✓ secrets: [90m12:13PM[0m [32mINF[0m [1mscanned ~6186121 bytes (6.19 MB) in 1.01s[0m
[90m12:13PM[0m [3
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin

Stage tier2: PASS
  COMPLETE
  ✓ bunker-qa.sh run_battery refuses to spawn when the target server's agent pool is full (parsed from 'bunker status' Agents: N/MAX), records exactly one FAIL evidence row, and exits rc=2 before any spawn attempt; a simulated full pool proves rc=2 with the PREFLIGHT FAIL line and no spawn log lines; a server with free capacity proceeds unchanged: /home/kara/.hermes/scripts/bunker-qa.sh: parse_bunker_capacity() (~L68-90) parses 'Agents: N/MAX' scoped to the target server; bunker_capacity() (~L95-115) probes `bunker status --server` with `bunker list` fallback; run() capacity preflight (~L336-364) at L355 `elif [ "$cap_used" -ge "$cap_max" ]` writes ONE FAIL evidence row, prints 'PREFLIGHT FAIL: bunker capacity exhausted on <srv> (N/M agents) — refusing to spawn (deterministic capacity error)' to stderr (L358), and `exit 2` (L359) before spawn_agent() is called. LIVE SIMULATION (fake bunker+ssh in PATH, temp HOME with ~/.bunker/config.yaml): (1) full pool Agents: 8/8 -> RC=2, stderr PREFLIGHT FAIL line present, evidence.jsonl contains EXACTLY ONE line {"cell":"run_battery","status":"FAIL","detail":"bunker capacity exhausted on bunker-las-02 (8/8 agents) — pool full, no spawn attempted (capacity preflight)"}, spawn.log ABSENT (no spawn attempted); (2) free capacity 2/8 -> no PREFLIGHT FAIL, spawn attempted (SPAWN-CALLED logged, agent=abcdef123456 assigned) i.e. proceeds unchanged; (3) 7/8 -> spawn attempted; (4) unparseable status -> WARN + proceeds (does not wedge); (5) parse_bunker_capacity unit checks: 8/8->'8 8', 2/8->'2 8', multi-section scopes to target (ours full->'8 8', ours free->'3 8'), unparseable/empty->''. `bash -n` reports SYNTAX OK; no LSP diagnostics.
bunker-qa.sh run_battery capacity preflight is fully implemented and proven by live simulation: full pool yields rc=2 with the PREFLIGHT FAIL line, exactly one FAIL evidence row, and zero spawn attempts, while free-capacity servers proceed unchanged.

Overall: PASS ✓
