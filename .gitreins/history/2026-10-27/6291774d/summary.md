# Verdict: QA-OFF-BY-ONE-RELENG-2

**Task:** Verify off-by-one releng QA collection availability
**Evaluated:** 2026-10-27T19:58:17.942397
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ Launch a fresh bunker QA battery for /home/kara/off-by-one and collect evidence with final COLLECTED agent output and no COLLECT-FAIL; if unavailable, preserve the exact external blocker.: Fresh battery launched and collected successfully on 2026-10-27. /tmp/bunker-qa-evidence-releng2.jsonl (mtime Oct 27 14:57, 6519 bytes, 24 rows) contains the launch row {"cell":"launch","status":"OK","detail":"agent=d645e3ef server=bunker-mvp ttl=4h script_bytes=158319 — battery started detached (qa-run.sh)"} at ts 2026-10-27T19:43:53Z and the final collect row {"cell":"collect","status":"OK","detail":"agent=d645e3ef (bunker-mvp) — pulled 21 evidence row(s), merge-deduped by ts; chaos-disconnect log captured at /tmp/bunker-qa-evidence-releng2.jsonl.chaos-disconnect.log (1386 bytes); temp agent destroyed"} at ts 2026-10-27T19:57:05Z. `grep -c 'COLLECT-FAIL' /tmp/bunker-qa-evidence-releng2.jsonl` -> 0 (exit 1, no matches), so no COLLECT-FAIL line exists. The collector's fail-closed truncation guard is satisfied: `grep -c '"cell": *"chaos-errorpath"'` -> 1 (line 23, status OK), so bunker-qa.sh:4122 'COLLECT-FAIL battery-truncated' was not taken and the script reached its only COLLECTED print at bunker-qa.sh:4157 after append_collect_row OK. Meta file /tmp/bunker-qa-evidence-releng2.jsonl.meta confirms agent=d645e3ef, server=bunker-mvp, launched=2026-10-27T19:43:53Z, project=off-by-one. Battery cells recorded: repo-head OK, launch OK, toolchain-bootstrap OK (go 1.27.3, gcc, make, jq, compose, buildx, node), fresh-install OK (rc=0), ci-pass OK (native suite PASS), ci-act UNVERIFIED (act rc=1), upgrade OK, docker-deploy N/A, ui-probe INFO, chaos-disconnect INFO, chaos-shutdown N/A, chaos-corruption INFO, chaos-resource PASS, chaos-errorpath OK. Task completed_at 2026-10-27T19:57:38Z is ~33s after the collect row, consistent with the run. No test suite applies (guards.tests=false in .gitreins/config.yaml; this is a live-ops criterion), so verification was by direct command execution on the evidence artifacts as shown above.
A fresh bunker QA battery for /home/kara/off-by-one launched on agent=d645e3ef (bunker-mvp) and collected 21 evidence rows with a final collect status=OK and zero COLLECT-FAIL lines, satisfying the criterion.

## Summary

Judge Result: QA-OFF-BY-ONE-RELENG-2

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ Launch a fresh bunker QA battery for /home/kara/off-by-one and collect evidence with final COLLECTED agent output and no COLLECT-FAIL; if unavailable, preserve the exact external blocker.: Fresh battery launched and collected successfully on 2026-10-27. /tmp/bunker-qa-evidence-releng2.jsonl (mtime Oct 27 14:57, 6519 bytes, 24 rows) contains the launch row {"cell":"launch","status":"OK","detail":"agent=d645e3ef server=bunker-mvp ttl=4h script_bytes=158319 — battery started detached (qa-run.sh)"} at ts 2026-10-27T19:43:53Z and the final collect row {"cell":"collect","status":"OK","detail":"agent=d645e3ef (bunker-mvp) — pulled 21 evidence row(s), merge-deduped by ts; chaos-disconnect log captured at /tmp/bunker-qa-evidence-releng2.jsonl.chaos-disconnect.log (1386 bytes); temp agent destroyed"} at ts 2026-10-27T19:57:05Z. `grep -c 'COLLECT-FAIL' /tmp/bunker-qa-evidence-releng2.jsonl` -> 0 (exit 1, no matches), so no COLLECT-FAIL line exists. The collector's fail-closed truncation guard is satisfied: `grep -c '"cell": *"chaos-errorpath"'` -> 1 (line 23, status OK), so bunker-qa.sh:4122 'COLLECT-FAIL battery-truncated' was not taken and the script reached its only COLLECTED print at bunker-qa.sh:4157 after append_collect_row OK. Meta file /tmp/bunker-qa-evidence-releng2.jsonl.meta confirms agent=d645e3ef, server=bunker-mvp, launched=2026-10-27T19:43:53Z, project=off-by-one. Battery cells recorded: repo-head OK, launch OK, toolchain-bootstrap OK (go 1.27.3, gcc, make, jq, compose, buildx, node), fresh-install OK (rc=0), ci-pass OK (native suite PASS), ci-act UNVERIFIED (act rc=1), upgrade OK, docker-deploy N/A, ui-probe INFO, chaos-disconnect INFO, chaos-shutdown N/A, chaos-corruption INFO, chaos-resource PASS, chaos-errorpath OK. Task completed_at 2026-10-27T19:57:38Z is ~33s after the collect row, consistent with the run. No test suite applies (guards.tests=false in .gitreins/config.yaml; this is a live-ops criterion), so verification was by direct command execution on the evidence artifacts as shown above.
A fresh bunker QA battery for /home/kara/off-by-one launched on agent=d645e3ef (bunker-mvp) and collected 21 evidence rows with a final collect status=OK and zero COLLECT-FAIL lines, satisfying the criterion.

Overall: PASS ✓
