# Verdict: DF-OFF-BY-ONE-29-R2

**Task:** enforce running unit flag contract
**Evaluated:** 2026-10-21T01:39:54.232964
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
- ✓ **tier2**
  - COMPLETE
  ✓ Running off-by-one.service ExecStart flags match deploy/off-by-one.service; drift fails with expected/observed values; hermetic match and drift tests pass: Contract source is deploy/off-by-one.service ExecStart ('/home/kara/off-by-one/off-by-one --port 8766 -load-threshold 4 -cron-interval 60s -pi-agent /home/kara/.local/bin/pi-agent -bwrap /usr/bin/bwrap -solve-mem-mb 6144'). scripts/check-deploy implements it: contract_flags() ~L117, running_flags() ~L127 (parses both systemctl structured form '{ path=... ; argv[]=... ; }' and plain argv, strips binary path), normalize_flags() ~L146, report_flag_drift() ~L155 (prints 'expected X, observed Y' / 'observed <absent>' / 'not in contract'), check_flag_contract() ~L199 (fail-closed on missing contract), and Leg 5 wired into main() ~L275 via `systemctl show $UNIT -p ExecStart --value` -> check_flag_contract $REPO_ROOT/deploy/off-by-one.service, exiting 1 with a remedy. Live manual runs: MATCH -> 'PASS: running unit's ExecStart flags match deploy/off-by-one.service', exit 0; DRIFT (stale argv with -load-threshold -1 and no -solve-mem-mb) -> exit 1 printing '-load-threshold: expected 4, observed -1' and '-solve-mem-mb: expected 6144, observed <absent>'. Hermetic tests: `bash scripts/check-deploy-test.sh` -> REAL_EXIT=0, 22 PASS / 0 FAIL, including 'PASS: flag contract: exact match exits 0', 'PASS: flag contract: systemd structured form with a re-spelled binary path exits 0', 'PASS: flag contract: stale drift (-load-threshold -1, no -solve-mem-mb) exits 1 naming both drifts', 'PASS: flag contract: missing vendored unit exits 1 (fail-closed)', and 'PASS: gate RED on pre-fix unit flags despite a fresh artifact (exit 1, drift named)'. `make check-deploy-test` (Makefile:121-122) -> MAKE_EXIT=0, 22 PASS / 0 FAIL. Tests are hermetic (stub systemctl on PATH, throwaway scratch clones, no live-service contact). Committed in HEAD 8981ebdb touching scripts/check-deploy (+165), scripts/check-deploy-test.sh (+101), docs/dogfood/diagnostics.md (+36).
The running-unit ExecStart flag contract is enforced in scripts/check-deploy leg 5 against deploy/off-by-one.service, drift fails with per-flag expected/observed values, and the hermetic match/drift test suite passes 22/22 with exit 0.

## Summary

Judge Result: DF-OFF-BY-ONE-29-R2

Stage tier1: PASS
    ✓ tests: scanners: nice=nice -n 10
  ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)

Stage tier2: PASS
  COMPLETE
  ✓ Running off-by-one.service ExecStart flags match deploy/off-by-one.service; drift fails with expected/observed values; hermetic match and drift tests pass: Contract source is deploy/off-by-one.service ExecStart ('/home/kara/off-by-one/off-by-one --port 8766 -load-threshold 4 -cron-interval 60s -pi-agent /home/kara/.local/bin/pi-agent -bwrap /usr/bin/bwrap -solve-mem-mb 6144'). scripts/check-deploy implements it: contract_flags() ~L117, running_flags() ~L127 (parses both systemctl structured form '{ path=... ; argv[]=... ; }' and plain argv, strips binary path), normalize_flags() ~L146, report_flag_drift() ~L155 (prints 'expected X, observed Y' / 'observed <absent>' / 'not in contract'), check_flag_contract() ~L199 (fail-closed on missing contract), and Leg 5 wired into main() ~L275 via `systemctl show $UNIT -p ExecStart --value` -> check_flag_contract $REPO_ROOT/deploy/off-by-one.service, exiting 1 with a remedy. Live manual runs: MATCH -> 'PASS: running unit's ExecStart flags match deploy/off-by-one.service', exit 0; DRIFT (stale argv with -load-threshold -1 and no -solve-mem-mb) -> exit 1 printing '-load-threshold: expected 4, observed -1' and '-solve-mem-mb: expected 6144, observed <absent>'. Hermetic tests: `bash scripts/check-deploy-test.sh` -> REAL_EXIT=0, 22 PASS / 0 FAIL, including 'PASS: flag contract: exact match exits 0', 'PASS: flag contract: systemd structured form with a re-spelled binary path exits 0', 'PASS: flag contract: stale drift (-load-threshold -1, no -solve-mem-mb) exits 1 naming both drifts', 'PASS: flag contract: missing vendored unit exits 1 (fail-closed)', and 'PASS: gate RED on pre-fix unit flags despite a fresh artifact (exit 1, drift named)'. `make check-deploy-test` (Makefile:121-122) -> MAKE_EXIT=0, 22 PASS / 0 FAIL. Tests are hermetic (stub systemctl on PATH, throwaway scratch clones, no live-service contact). Committed in HEAD 8981ebdb touching scripts/check-deploy (+165), scripts/check-deploy-test.sh (+101), docs/dogfood/diagnostics.md (+36).
The running-unit ExecStart flag contract is enforced in scripts/check-deploy leg 5 against deploy/off-by-one.service, drift fails with per-flag expected/observed values, and the hermetic match/drift test suite passes 22/22 with exit 0.

Overall: PASS ✓
