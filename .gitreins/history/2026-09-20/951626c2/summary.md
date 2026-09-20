# Verdict: OB-GAP-079

**Task:** watchdog: node resolve TIMEOUT is UNVERIFIABLE, not hollow-wipe
**Evaluated:** 2026-09-20T13:58:14.181208
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	3.253s
- ✓ **tier2**
  - COMPLETE
  ✓ The watchdog must not equate a node resolve TIMEOUT with ERR_MODULE_NOT_FOUND: TIMEOUT gets its own UNVERIFIABLE outcome state (distinct exit code or status line, no hollow-wipe alert text) and a red-probe test proves a TIMEOUT no longer fires the wipe alert while a genuine resolution failure still does; selftest covers both paths: scripts/pi-agent-watchdog.sh:40-70 documents three states with exit contract 0=healthy/1=broken/3=unverifiable; the per-package probe loop (~line 186) buckets node rc=124 into timed_out[] instead of missing[], and when only timeouts occur sets resolve_state="unverifiable" with resolve_msg="WARN (not alert): pi-agent resolve probe TIMED OUT under host load ... Solve health UNKNOWN; re-run when load subsides" (no 'solve path BROKEN', no 'UNHEALTHY (hollow-wipe class)', no rebuild recipe); lines ~218-224 map presence_ok=1 && resolve_state=unverifiable -> resolve_rc=3, distinct from the unresolved path's rc=1 which still emits the full BROKEN alert + 'npm install --ignore-scripts' remedy. RED-PROBE PROVEN: running the new selftest against the pre-change watchdog (git show 594ee2d^:scripts/pi-agent-watchdog.sh) yields 'pi-agent-watchdog self-test: 58/77 checks passed - 19 FAILED', with ARM 8 reporting 'FAIL exit code 3 (distinct from broken=1) expected: 3 actual: 1' and the actual output being 'ALERT: pi-agent solve path BROKEN - workspace dep(s) unresolved: @earendil-works/pi-tui (node TIMEOUT). Re-link ... npm install --ignore-scripts' — i.e. the hollow-wipe alert for a TIMEOUT; ARM 8b (real unstubbed node busy-waiting past the budget) fails identically on the old script. On current code `bash scripts/tests/pi-agent-watchdog-selftest.sh` exits 0 with 'pi-agent-watchdog self-test: 77/77 checks passed - ALL GREEN', covering both paths: ARM 8/8b assert exit 3 + WARN text + check_not_contains 'solve path BROKEN'/'UNHEALTHY (hollow-wipe class)'/'npm install --ignore-scripts'; ARM 9 shows repeated TIMEOUTs dedup to one WARN while a later real failure still exits 1 with the BROKEN alert; ARM 10/10b show a genuine ERR_MODULE_NOT_FOUND still exits 1 with the BROKEN alert + re-link remedy and no WARN text (mixed run discloses the timed-out package as 'unverified:' not 'unresolved:'); ARM 7 confirms the hollow-wipe presence class is unchanged. Docs updated at docs/dogfood/diagnostics.md:39-47; Makefile:134-135 exposes the selftest target.
TIMEOUT now has its own UNVERIFIABLE state (WARN, exit 3, no hollow-wipe text) while genuine resolution failures still fire the BROKEN alert (exit 1), and the selftest is a proven red probe (19/77 fail on the old script, 77/77 green on the new one).

## Summary

Judge Result: OB-GAP-079

Stage tier1: PASS
    ✓ secrets: secrets: harness state excluded from gitleaks scope (.gitreins/**)
  ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	3.253s

Stage tier2: PASS
  COMPLETE
  ✓ The watchdog must not equate a node resolve TIMEOUT with ERR_MODULE_NOT_FOUND: TIMEOUT gets its own UNVERIFIABLE outcome state (distinct exit code or status line, no hollow-wipe alert text) and a red-probe test proves a TIMEOUT no longer fires the wipe alert while a genuine resolution failure still does; selftest covers both paths: scripts/pi-agent-watchdog.sh:40-70 documents three states with exit contract 0=healthy/1=broken/3=unverifiable; the per-package probe loop (~line 186) buckets node rc=124 into timed_out[] instead of missing[], and when only timeouts occur sets resolve_state="unverifiable" with resolve_msg="WARN (not alert): pi-agent resolve probe TIMED OUT under host load ... Solve health UNKNOWN; re-run when load subsides" (no 'solve path BROKEN', no 'UNHEALTHY (hollow-wipe class)', no rebuild recipe); lines ~218-224 map presence_ok=1 && resolve_state=unverifiable -> resolve_rc=3, distinct from the unresolved path's rc=1 which still emits the full BROKEN alert + 'npm install --ignore-scripts' remedy. RED-PROBE PROVEN: running the new selftest against the pre-change watchdog (git show 594ee2d^:scripts/pi-agent-watchdog.sh) yields 'pi-agent-watchdog self-test: 58/77 checks passed - 19 FAILED', with ARM 8 reporting 'FAIL exit code 3 (distinct from broken=1) expected: 3 actual: 1' and the actual output being 'ALERT: pi-agent solve path BROKEN - workspace dep(s) unresolved: @earendil-works/pi-tui (node TIMEOUT). Re-link ... npm install --ignore-scripts' — i.e. the hollow-wipe alert for a TIMEOUT; ARM 8b (real unstubbed node busy-waiting past the budget) fails identically on the old script. On current code `bash scripts/tests/pi-agent-watchdog-selftest.sh` exits 0 with 'pi-agent-watchdog self-test: 77/77 checks passed - ALL GREEN', covering both paths: ARM 8/8b assert exit 3 + WARN text + check_not_contains 'solve path BROKEN'/'UNHEALTHY (hollow-wipe class)'/'npm install --ignore-scripts'; ARM 9 shows repeated TIMEOUTs dedup to one WARN while a later real failure still exits 1 with the BROKEN alert; ARM 10/10b show a genuine ERR_MODULE_NOT_FOUND still exits 1 with the BROKEN alert + re-link remedy and no WARN text (mixed run discloses the timed-out package as 'unverified:' not 'unresolved:'); ARM 7 confirms the hollow-wipe presence class is unchanged. Docs updated at docs/dogfood/diagnostics.md:39-47; Makefile:134-135 exposes the selftest target.
TIMEOUT now has its own UNVERIFIABLE state (WARN, exit 3, no hollow-wipe text) while genuine resolution failures still fire the BROKEN alert (exit 1), and the selftest is a proven red probe (19/77 fail on the old script, 77/77 green on the new one).

Overall: PASS ✓
