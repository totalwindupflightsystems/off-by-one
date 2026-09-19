# Verdict: OB-GAP-078

**Task:** Harden pi-agent watchdog: probe solve-path workspace dep resolution
**Evaluated:** 2026-09-18T19:46:36.657369
**Result:** ✓ PASS

## Pipeline Stages

- ✓ **tier1**
  -   ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m2:45PM[0m [32mINF[0m [1mscanned ~7358952 bytes (7.36 MB) in 1.75s[0m
[90m2:45PM[0m [32m
- ✓ **tier2**
  - COMPLETE
  ✓ scripts/pi-agent-watchdog.sh exercises WORKSPACE DEP RESOLUTION from $PI_DIR (node resolving the @earendil-works/* workspace packages declared by /tmp/pi/package.json workspaces), not filesystem presence: a missing package/symlink makes the probe exit non-zero and NAME the missing package(s).: scripts/pi-agent-watchdog.sh:68-160 adds a stage-2 resolution probe. Candidates are derived dynamically (not hardcoded): (a) declarations from packages/*/package.json and packages/session-backends/*/package.json keeping names with main/exports (lines 88-100), (b) installed symlinks under node_modules/@earendil-works keyed by link name (lines 102-113). Each candidate is then resolved by NODE itself: `cd "$PI_DIR" && timeout $RESOLVE_TIMEOUT node -e "import('$name').then(()=>{}).catch(e=>{console.error(e.code||'ERR', ...); process.exit(1)})"` (line 132). Independent reproduction: healthy fixture -> rc=0, silent; `rm node_modules/@earendil-works/pi-tui` while packages/pi-tui still exists -> rc=1 with 'ALERT: pi-agent solve path BROKEN — workspace dep(s) unresolved: @earendil-works/pi-tui (node ERR_MODULE_NOT_FOUND). Re-link the workspace packages: cd ... && npm install --ignore-scripts'; removing chord as well names both packages. Real /tmp/pi (whose package.json declares workspaces ["packages/*","packages/session-backends/*",...]) -> rc=0. Verdict gate at line 163 requires resolve_state=ok for exit 0, so a missing package forces non-zero.
  ✓ A self-test ships in-repo (scripts/tests/pi-agent-watchdog-selftest.sh, wired as a make target) that runs the probe against a temp fixture workspace: healthy -> exit 0, one symlink removed -> non-zero naming the package, plus a NEUTER arm proving the test depends on the new resolution probe.: scripts/tests/pi-agent-watchdog-selftest.sh exists (12601 bytes, executable) and is wired as Makefile target `pi-agent-watchdog-selftest` (Makefile:103-104, listed in .PHONY at Makefile:1). Ran `make pi-agent-watchdog-selftest` -> MAKE_EXIT=0, output 'pi-agent-watchdog self-test: 40/40 checks passed — ALL GREEN'. ARM 1 (selftest:110-117) healthy temp fixture -> exit 0, no alert, no stamp. ARM 2 (selftest:120-130) `rm $FIX/node_modules/@earendil-works/pi-tui` -> exit 1, contains '@earendil-works/pi-tui' and 'ERR_MODULE_NOT_FOUND'. ARM 3 NEUTER (selftest:158-174) seds `resolution_stage=1` -> `resolution_stage=0` in a COPY of the probe (lever at pi-agent-watchdog.sh:59, guard at :71); independently reproduced: neutered copy on the broken fixture exits 0 and prints nothing, while the tracked probe on the same fixture exits 1 and alerts — proving the broken-fixture arm depends on the new resolution stage. Fixture package.json declares workspaces ["packages/*","packages/session-backends/*"] (selftest:78), matching the real /tmp/pi/package.json shape. Both scripts pass `bash -n` syntax check.
Both criteria verified: the watchdog now resolves @earendil-works workspace packages via node from $PI_DIR and names unresolved ones (independently reproduced), and the in-repo make-wired self-test passes 40/40 including a working NEUTER negative control.

## Summary

Judge Result: OB-GAP-078

Stage tier1: PASS
    ✓ tests: ok  	github.com/totalwindupflightsystems/off-by-one/cmd/off-by-one	(cached)
ok  	github.com/totalwin
  ✓ secrets: [90m2:45PM[0m [32mINF[0m [1mscanned ~7358952 bytes (7.36 MB) in 1.75s[0m
[90m2:45PM[0m [32m

Stage tier2: PASS
  COMPLETE
  ✓ scripts/pi-agent-watchdog.sh exercises WORKSPACE DEP RESOLUTION from $PI_DIR (node resolving the @earendil-works/* workspace packages declared by /tmp/pi/package.json workspaces), not filesystem presence: a missing package/symlink makes the probe exit non-zero and NAME the missing package(s).: scripts/pi-agent-watchdog.sh:68-160 adds a stage-2 resolution probe. Candidates are derived dynamically (not hardcoded): (a) declarations from packages/*/package.json and packages/session-backends/*/package.json keeping names with main/exports (lines 88-100), (b) installed symlinks under node_modules/@earendil-works keyed by link name (lines 102-113). Each candidate is then resolved by NODE itself: `cd "$PI_DIR" && timeout $RESOLVE_TIMEOUT node -e "import('$name').then(()=>{}).catch(e=>{console.error(e.code||'ERR', ...); process.exit(1)})"` (line 132). Independent reproduction: healthy fixture -> rc=0, silent; `rm node_modules/@earendil-works/pi-tui` while packages/pi-tui still exists -> rc=1 with 'ALERT: pi-agent solve path BROKEN — workspace dep(s) unresolved: @earendil-works/pi-tui (node ERR_MODULE_NOT_FOUND). Re-link the workspace packages: cd ... && npm install --ignore-scripts'; removing chord as well names both packages. Real /tmp/pi (whose package.json declares workspaces ["packages/*","packages/session-backends/*",...]) -> rc=0. Verdict gate at line 163 requires resolve_state=ok for exit 0, so a missing package forces non-zero.
  ✓ A self-test ships in-repo (scripts/tests/pi-agent-watchdog-selftest.sh, wired as a make target) that runs the probe against a temp fixture workspace: healthy -> exit 0, one symlink removed -> non-zero naming the package, plus a NEUTER arm proving the test depends on the new resolution probe.: scripts/tests/pi-agent-watchdog-selftest.sh exists (12601 bytes, executable) and is wired as Makefile target `pi-agent-watchdog-selftest` (Makefile:103-104, listed in .PHONY at Makefile:1). Ran `make pi-agent-watchdog-selftest` -> MAKE_EXIT=0, output 'pi-agent-watchdog self-test: 40/40 checks passed — ALL GREEN'. ARM 1 (selftest:110-117) healthy temp fixture -> exit 0, no alert, no stamp. ARM 2 (selftest:120-130) `rm $FIX/node_modules/@earendil-works/pi-tui` -> exit 1, contains '@earendil-works/pi-tui' and 'ERR_MODULE_NOT_FOUND'. ARM 3 NEUTER (selftest:158-174) seds `resolution_stage=1` -> `resolution_stage=0` in a COPY of the probe (lever at pi-agent-watchdog.sh:59, guard at :71); independently reproduced: neutered copy on the broken fixture exits 0 and prints nothing, while the tracked probe on the same fixture exits 1 and alerts — proving the broken-fixture arm depends on the new resolution stage. Fixture package.json declares workspaces ["packages/*","packages/session-backends/*"] (selftest:78), matching the real /tmp/pi/package.json shape. Both scripts pass `bash -n` syntax check.
Both criteria verified: the watchdog now resolves @earendil-works workspace packages via node from $PI_DIR and names unresolved ones (independently reproduced), and the in-repo make-wired self-test passes 40/40 including a working NEUTER negative control.

Overall: PASS ✓
