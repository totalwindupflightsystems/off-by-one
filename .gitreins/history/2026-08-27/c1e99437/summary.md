# Verdict: OB-OPS-004

**Task:** pi-agent binary health watchdog
**Evaluated:** 2026-08-27T11:44:27.045492
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: [90m6:43AM[0m [32mINF[0m [1mscanned ~10890409 bytes (10.89 MB) in 3.14s[0m
[90m6:43AM[0m [3
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	6.854s
ok  	github.com/totalwindu
- ✓ **tier2**
  - COMPLETE
  ✓ Watchdog script scripts/pi-agent-watchdog.sh exists, is executable, and prints an ALERT when /tmp/pi/packages/coding-agent/dist/cli.js or /tmp/pi/package.json is missing/empty (hollow-wipe class) while staying silent when healthy; a no_agent cron job runs it at <=15m interval; live test: temporarily moving cli.js aside makes the script print an alert, restoring it makes it silent.: Script exists (scripts/pi-agent-watchdog.sh, 1770 bytes, -rwxr-xr-x, test -x confirms executable). Uses `[ -s "$PI_DIR/packages/coding-agent/dist/cli.js" ]` and `[ -s "$PI_DIR/package.json" ]` which fail for both missing and empty files. Live tests (run_command): healthy state -> silent exit 0; moving cli.js aside -> 'ALERT: pi-agent binary UNHEALTHY (hollow-wipe class)' exit 1; empty cli.js -> ALERT exit 1; empty package.json -> ALERT exit 1; restoring -> silent exit 0. Cron job in ~/.hermes/cron/jobs.json: 'pi-agent binary watchdog (off-by-one)', script pi-agent-watchdog.sh, no_agent=True, schedule interval 15m (<=15m), enabled=True, state=scheduled. Environment fully restored (cli.js rebuilt to 710 bytes matching original, package.json restored).
Watchdog script exists, is executable, alerts on missing/empty cli.js and package.json, stays silent when healthy, has a no_agent cron job at 15m interval, and passed the live move-aside/restore test.

## Summary

Judge Result: OB-OPS-004

Stage tier1: PASS
    ✓ secrets: [90m6:43AM[0m [32mINF[0m [1mscanned ~10890409 bytes (10.89 MB) in 3.14s[0m
[90m6:43AM[0m [3
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	6.854s
ok  	github.com/totalwindu

Stage tier2: PASS
  COMPLETE
  ✓ Watchdog script scripts/pi-agent-watchdog.sh exists, is executable, and prints an ALERT when /tmp/pi/packages/coding-agent/dist/cli.js or /tmp/pi/package.json is missing/empty (hollow-wipe class) while staying silent when healthy; a no_agent cron job runs it at <=15m interval; live test: temporarily moving cli.js aside makes the script print an alert, restoring it makes it silent.: Script exists (scripts/pi-agent-watchdog.sh, 1770 bytes, -rwxr-xr-x, test -x confirms executable). Uses `[ -s "$PI_DIR/packages/coding-agent/dist/cli.js" ]` and `[ -s "$PI_DIR/package.json" ]` which fail for both missing and empty files. Live tests (run_command): healthy state -> silent exit 0; moving cli.js aside -> 'ALERT: pi-agent binary UNHEALTHY (hollow-wipe class)' exit 1; empty cli.js -> ALERT exit 1; empty package.json -> ALERT exit 1; restoring -> silent exit 0. Cron job in ~/.hermes/cron/jobs.json: 'pi-agent binary watchdog (off-by-one)', script pi-agent-watchdog.sh, no_agent=True, schedule interval 15m (<=15m), enabled=True, state=scheduled. Environment fully restored (cli.js rebuilt to 710 bytes matching original, package.json restored).
Watchdog script exists, is executable, alerts on missing/empty cli.js and package.json, stays silent when healthy, has a no_agent cron job at 15m interval, and passed the live move-aside/restore test.

Overall: PASS ✓
