# Verdict: QA-OFF-BY-ONE-20-R3-R2

**Task:** QA-OFF-BY-ONE-20-R3 battery collect + acceptance grading
**Evaluated:** 2026-11-03T01:37:01.192426
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check
- ✗ **tier2**
  - INCOMPLETE
  ✗ PASS when the collected battery evidence proves: (1) ui_probe_scratch_start arm in ~/.hermes/scripts/bunker-qa.sh; (2) ui-probe cell OK 'UI serves text/html' with the SPA shell served (/, 200 text/html, byte-exact web/index.html with nav tabs home,search,submit,explore,system,export,import) while a foreign listener held :8766 (pre-run health evidence of the contending service); (3) no bind: address already in use signature in the run's ui-probe evidence: Sub-part (1) PASS: grep -n ui_probe_scratch_start ~/.hermes/scripts/bunker-qa.sh -> 2736 (comment), 2740 (function def), 2801 (pre-detected-conflict call), 2834 (recovery call); body 2740-2775 tries --port/-port/OFF_BY_ONE_PORT and sets UIPID+UI_LISTEN, scratch port picked at 2772-2787 when UI_OCCUPIED != ' '. Sub-part (3) PASS: grep -c 'address already in use' on the run evidence = 0. Sub-part (2) FAIL: the run evidence (~/.hermes/qa/evidence/off-by-one-qa-2026-11-03-010307/bunker-qa-evidence-20261103T010307Z-1194510.jsonl, 25 rows) row 17 is {"cell":"ui-probe","status":"OK","detail":"UI serves text/html: http://127.0.0.1:24700/chat 200 text/html; charset=utf-8 5260B ... http://127.0.0.1:24700/ 200 text/html; charset=utf-8 5260B ... (start: go run ./cmd/off-by-one), other paths: /index.html=[404 text/plain; charset=utf-8]"} — OK status, 'UI serves text/html', / 200 text/html, and byte-exactness all hold (web/index.html is 5260 bytes per wc -c, matching the served 5260B; nav tabs home,search,submit,explore,system,export,import confirmed at web/index.html:36-42 data-view=). However the required 'while a foreign listener held :8766 (pre-run health evidence of the contending service)' is NOT proven: grep -c '8766' on the run evidence = 0 and grep -c 'health' = 0; no pre-run occupancy/health row exists anywhere in the 25 collected rows. The scratch port 24700 implies a conflict was detected, but the evidence never names the held port or the contending service. The only :8766 foreign-listener evidence in the evidence tree comes from other runs (off-by-one-qa-2026-10-21-11-14-40/qa-evidence-collected.jsonl:16 and off-by-one-qa-2026-10-28-05-43-44/harvest/qa-evidence.jsonl:16, both 'listen tcp 127.0.0.1:8766: bind: address already in use'), not from this run.
The scratch-port arm exists and the ui-probe cell graded OK with the byte-exact 5260B SPA shell and no bind error, but the collected evidence contains zero reference to :8766 or any pre-run health/occupancy record of the contending service, so sub-part (2) is unproven.

## Summary

Judge Result: QA-OFF-BY-ONE-20-R3-R2

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: scanners=gitleaks+builtin cross-check

Stage tier2: FAIL
  INCOMPLETE
  ✗ PASS when the collected battery evidence proves: (1) ui_probe_scratch_start arm in ~/.hermes/scripts/bunker-qa.sh; (2) ui-probe cell OK 'UI serves text/html' with the SPA shell served (/, 200 text/html, byte-exact web/index.html with nav tabs home,search,submit,explore,system,export,import) while a foreign listener held :8766 (pre-run health evidence of the contending service); (3) no bind: address already in use signature in the run's ui-probe evidence: Sub-part (1) PASS: grep -n ui_probe_scratch_start ~/.hermes/scripts/bunker-qa.sh -> 2736 (comment), 2740 (function def), 2801 (pre-detected-conflict call), 2834 (recovery call); body 2740-2775 tries --port/-port/OFF_BY_ONE_PORT and sets UIPID+UI_LISTEN, scratch port picked at 2772-2787 when UI_OCCUPIED != ' '. Sub-part (3) PASS: grep -c 'address already in use' on the run evidence = 0. Sub-part (2) FAIL: the run evidence (~/.hermes/qa/evidence/off-by-one-qa-2026-11-03-010307/bunker-qa-evidence-20261103T010307Z-1194510.jsonl, 25 rows) row 17 is {"cell":"ui-probe","status":"OK","detail":"UI serves text/html: http://127.0.0.1:24700/chat 200 text/html; charset=utf-8 5260B ... http://127.0.0.1:24700/ 200 text/html; charset=utf-8 5260B ... (start: go run ./cmd/off-by-one), other paths: /index.html=[404 text/plain; charset=utf-8]"} — OK status, 'UI serves text/html', / 200 text/html, and byte-exactness all hold (web/index.html is 5260 bytes per wc -c, matching the served 5260B; nav tabs home,search,submit,explore,system,export,import confirmed at web/index.html:36-42 data-view=). However the required 'while a foreign listener held :8766 (pre-run health evidence of the contending service)' is NOT proven: grep -c '8766' on the run evidence = 0 and grep -c 'health' = 0; no pre-run occupancy/health row exists anywhere in the 25 collected rows. The scratch port 24700 implies a conflict was detected, but the evidence never names the held port or the contending service. The only :8766 foreign-listener evidence in the evidence tree comes from other runs (off-by-one-qa-2026-10-21-11-14-40/qa-evidence-collected.jsonl:16 and off-by-one-qa-2026-10-28-05-43-44/harvest/qa-evidence.jsonl:16, both 'listen tcp 127.0.0.1:8766: bind: address already in use'), not from this run.
The scratch-port arm exists and the ui-probe cell graded OK with the byte-exact 5260B SPA shell and no bind error, but the collected evidence contains zero reference to :8766 or any pre-run health/occupancy record of the contending service, so sub-part (2) is unproven.

Overall: FAIL ✗
