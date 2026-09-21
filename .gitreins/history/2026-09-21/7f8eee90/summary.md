# Verdict: OB-GAP-086

**Task:** Watchdog presence gate false-alarms on ELF pi layout
**Evaluated:** 2026-09-21T01:19:19.277269
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	1.454s
- ✓ **tier2**
  - COMPLETE
  ✓ scripts/pi-agent-watchdog.sh exits 0 silently on a host whose solves complete (ELF /tmp/pi/pi, wrapper executable), while still failing rc 1 on a fixture whose dist/cli.js is deleted; self-test all green with new ELF-fixture arms: Live-verified. (a) ELF host silent rc 0: real host /tmp/pi is an ELF release install (`file /tmp/pi/pi` = 'ELF 64-bit LSB executable, x86-64'; no packages/, no .git, node_modules/.bin has 0 entries); `bash scripts/pi-agent-watchdog.sh` -> REAL_HOST_RC=0 with no output; independent ELF fixture (regular-file pi + executable wrapper, no npm tree) -> ELF_FIXTURE_RC=0, OUT_BYTES=0. Pre-fix script (git show 373a839^:scripts/pi-agent-watchdog.sh) on the same host -> OLD_SCRIPT_RC=1 with 'ALERT: pi-agent binary UNHEALTHY (hollow-wipe class) — cli.js=0 pkg.json=1 node_modules/.bin=0 wrapper=1', the exact false-alarm, so the fix is real. (b) cli.js-deleted fixture still rc 1: npm fixture with dist/cli.js removed -> NPM_DELETED_RC=1 + hollow-wipe alert; selftest ARM 7d (selftest:421-427) asserts exit 1 + 'pi-agent binary UNHEALTHY (hollow-wipe class)' + 'cli.js=0'; ARM 7c (pi missing AND no cli.js) and ARM 7 also rc 1. (c) Self-test green with new ELF arms: `bash scripts/tests/pi-agent-watchdog-selftest.sh` -> SELFTEST_EXIT=0, 'pi-agent-watchdog self-test: 93/93 checks passed — ALL GREEN'; `make pi-agent-watchdog-selftest` -> MAKE_RC=0, 93/93. New ELF-fixture arms all pass: ARM 7b arm7b-elf-layout-healthy (rc 0, no alert, no stamp), 7c arm7c-neither-layout-alerts (rc 1), 7d arm7d-npm-clijs-deleted-still-alerts (rc 1), 7e arm7e-elf-skips-stage2 (rc 0, no 'UNVERIFIABLE', no 'workspace-package enumeration failed'), 7f arm7f-demoted-legs-non-gating (rc 0). make_elf_fixture() at selftest:370 builds the ELF layout; watchdog stage-1 resolution (pi-agent-watchdog.sh:118-127) mirrors wrapper findPiBin (scripts/pi-agent:132-149: PI_BIN -> __dirname/pi -> ../lib/pi/pi -> /tmp/pi/pi, statSync().isFile() == test -f). bash -n syntax OK on both scripts.
Watchdog now exits 0 silently on the real ELF /tmp/pi layout (pre-fix script false-alarmed rc 1 on the same host), still exits rc 1 on a dist/cli.js-deleted fixture, and the self-test is 93/93 green including the new ELF-fixture arms 7b-7f.

## Summary

Judge Result: OB-GAP-086

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	1.454s

Stage tier2: PASS
  COMPLETE
  ✓ scripts/pi-agent-watchdog.sh exits 0 silently on a host whose solves complete (ELF /tmp/pi/pi, wrapper executable), while still failing rc 1 on a fixture whose dist/cli.js is deleted; self-test all green with new ELF-fixture arms: Live-verified. (a) ELF host silent rc 0: real host /tmp/pi is an ELF release install (`file /tmp/pi/pi` = 'ELF 64-bit LSB executable, x86-64'; no packages/, no .git, node_modules/.bin has 0 entries); `bash scripts/pi-agent-watchdog.sh` -> REAL_HOST_RC=0 with no output; independent ELF fixture (regular-file pi + executable wrapper, no npm tree) -> ELF_FIXTURE_RC=0, OUT_BYTES=0. Pre-fix script (git show 373a839^:scripts/pi-agent-watchdog.sh) on the same host -> OLD_SCRIPT_RC=1 with 'ALERT: pi-agent binary UNHEALTHY (hollow-wipe class) — cli.js=0 pkg.json=1 node_modules/.bin=0 wrapper=1', the exact false-alarm, so the fix is real. (b) cli.js-deleted fixture still rc 1: npm fixture with dist/cli.js removed -> NPM_DELETED_RC=1 + hollow-wipe alert; selftest ARM 7d (selftest:421-427) asserts exit 1 + 'pi-agent binary UNHEALTHY (hollow-wipe class)' + 'cli.js=0'; ARM 7c (pi missing AND no cli.js) and ARM 7 also rc 1. (c) Self-test green with new ELF arms: `bash scripts/tests/pi-agent-watchdog-selftest.sh` -> SELFTEST_EXIT=0, 'pi-agent-watchdog self-test: 93/93 checks passed — ALL GREEN'; `make pi-agent-watchdog-selftest` -> MAKE_RC=0, 93/93. New ELF-fixture arms all pass: ARM 7b arm7b-elf-layout-healthy (rc 0, no alert, no stamp), 7c arm7c-neither-layout-alerts (rc 1), 7d arm7d-npm-clijs-deleted-still-alerts (rc 1), 7e arm7e-elf-skips-stage2 (rc 0, no 'UNVERIFIABLE', no 'workspace-package enumeration failed'), 7f arm7f-demoted-legs-non-gating (rc 0). make_elf_fixture() at selftest:370 builds the ELF layout; watchdog stage-1 resolution (pi-agent-watchdog.sh:118-127) mirrors wrapper findPiBin (scripts/pi-agent:132-149: PI_BIN -> __dirname/pi -> ../lib/pi/pi -> /tmp/pi/pi, statSync().isFile() == test -f). bash -n syntax OK on both scripts.
Watchdog now exits 0 silently on the real ELF /tmp/pi layout (pre-fix script false-alarmed rc 1 on the same host), still exits rc 1 on a dist/cli.js-deleted fixture, and the self-test is 93/93 green including the new ELF-fixture arms 7b-7f.

Overall: PASS ✓
