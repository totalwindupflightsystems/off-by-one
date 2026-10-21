# Verdict: DF-OFF-BY-ONE-29-R2

**Task:** enforce running unit flag contract
**Evaluated:** 2026-10-21T01:36:10.277653
**Result:** ✗ FAIL

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10
- ✗ **tier2**
  - INCOMPLETE
  ✗ Running off-by-one.service ExecStart flags match deploy/off-by-one.service; drift fails with expected/observed values; hermetic match and drift tests pass: No flag-contract enforcement exists. deploy/off-by-one.service:62 defines ExecStart with flags (--port 8766 -load-threshold 4 -cron-interval 60s -pi-agent ... -bwrap /usr/bin/bwrap -solve-mem-mb 6144), but every checker extracts ONLY the binary path and discards the flags: scripts/gate-deploy:111 uses sed 's/^ExecStart=.*path=\([^ ;]*\).*/\1/p' and scripts/check-deploy-test.sh:180-184 compares only against DOCUMENTED_EXEC=/home/kara/off-by-one/off-by-one. grep for load-threshold|solve-mem-mb|cron-interval across *.sh/*.go/*.py/Makefile/*.service returns only the unit file itself plus unrelated Go flag definitions — no comparison logic. grep for 'expected'/'observed' in scripts/check-deploy, scripts/gate-deploy, scripts/check-deploy-test.sh returns zero matches, so no expected/observed drift reporting. Drift simulation: I copied the unit and applied the exact drift the task describes (-load-threshold 4 -> -load-threshold -1, -solve-mem-mb 6144 removed); the existing extraction still yields '/home/kara/off-by-one/off-by-one' and the drift passes silently. Test evidence: `bash scripts/check-deploy-test.sh` -> exit 0, 'check-deploy-test: all assertions passed', but its assertions cover only ExecStart path, Restart=always, WorkingDirectory, and doc pointers — no flag match test and no flag drift test. git diff shows only .gitreins/tasks.yaml changed (8 lines, task record); git log for deploy/off-by-one.service, scripts/check-deploy-test.sh, scripts/gate-deploy, scripts/check-deploy shows the last commit is fc8b9364 (DF-OFF-BY-ONE-29), not this task. docs/dogfood/2026-10-20-deploy-truth-integration.md:40 documents the still-unfixed defect (running unit ships -load-threshold -1 and no -solve-mem-mb).
The task added only a board record; no code or test enforces the ExecStart flag contract, and a simulated drift of -load-threshold/-solve-mem-mb passes every existing check silently.

## Summary

Judge Result: DF-OFF-BY-ONE-29-R2

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: scanners: nice=nice -n 10

Stage tier2: FAIL
  INCOMPLETE
  ✗ Running off-by-one.service ExecStart flags match deploy/off-by-one.service; drift fails with expected/observed values; hermetic match and drift tests pass: No flag-contract enforcement exists. deploy/off-by-one.service:62 defines ExecStart with flags (--port 8766 -load-threshold 4 -cron-interval 60s -pi-agent ... -bwrap /usr/bin/bwrap -solve-mem-mb 6144), but every checker extracts ONLY the binary path and discards the flags: scripts/gate-deploy:111 uses sed 's/^ExecStart=.*path=\([^ ;]*\).*/\1/p' and scripts/check-deploy-test.sh:180-184 compares only against DOCUMENTED_EXEC=/home/kara/off-by-one/off-by-one. grep for load-threshold|solve-mem-mb|cron-interval across *.sh/*.go/*.py/Makefile/*.service returns only the unit file itself plus unrelated Go flag definitions — no comparison logic. grep for 'expected'/'observed' in scripts/check-deploy, scripts/gate-deploy, scripts/check-deploy-test.sh returns zero matches, so no expected/observed drift reporting. Drift simulation: I copied the unit and applied the exact drift the task describes (-load-threshold 4 -> -load-threshold -1, -solve-mem-mb 6144 removed); the existing extraction still yields '/home/kara/off-by-one/off-by-one' and the drift passes silently. Test evidence: `bash scripts/check-deploy-test.sh` -> exit 0, 'check-deploy-test: all assertions passed', but its assertions cover only ExecStart path, Restart=always, WorkingDirectory, and doc pointers — no flag match test and no flag drift test. git diff shows only .gitreins/tasks.yaml changed (8 lines, task record); git log for deploy/off-by-one.service, scripts/check-deploy-test.sh, scripts/gate-deploy, scripts/check-deploy shows the last commit is fc8b9364 (DF-OFF-BY-ONE-29), not this task. docs/dogfood/2026-10-20-deploy-truth-integration.md:40 documents the still-unfixed defect (running unit ships -load-threshold -1 and no -solve-mem-mb).
The task added only a board record; no code or test enforces the ExecStart flag contract, and a simulated drift of -load-threshold/-solve-mem-mb passes every existing check silently.

Overall: FAIL ✗
