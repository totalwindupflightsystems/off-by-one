#!/usr/bin/env bash
# scripts/tests/pi-agent-watchdog-selftest.sh — deterministic regression
# self-test for scripts/pi-agent-watchdog.sh (acceptance for OB-GAP-078).
#
# No network, no credentials, no real install: every arm runs the probe against
# a temp FIXTURE workspace (mktemp dir) through the probe's PI_DIR /
# PI_AGENT_WATCHDOG_STAMP / PI_AGENT_WRAPPER overrides. The live /tmp/pi is
# never read and never written.
#
# What it proves:
#   1. a healthy fixture passes SILENTLY (exit 0, nothing on the alert channel);
#   2. a vanished workspace link fails the probe, names the package and carries
#      the node error code + the re-link remedy — the 2026-09-18 incident;
#   2b. a package whose DIRECTORY was deleted (declaration gone, link left
#      dangling) is still named, via the installed-scope-link source;
#   2c. EVERY unresolved package is reported, across both declaration roots;
#   3. the NEUTER arm: with the resolution lever flipped in a COPY of the probe,
#      the same broken fixture exits 0 and prints nothing — the arm above is
#      conditional on the new resolution stage, not on something else;
#   4. a repeated run against the same broken fixture emits exactly ONE alert
#      (state dedup) and a healthy run afterwards clears the stamp;
#   5. a missing 'node' probe tool and an empty workspace enumeration FAIL LOUD
#      (never a silent healthy verdict);
#   6. the pre-existing presence class (hollow-wipe) still alerts unchanged.
#
# Usage: bash scripts/tests/pi-agent-watchdog-selftest.sh   (or: make pi-agent-watchdog-selftest)

set -uo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SELF_DIR/.." && pwd)"
WATCHDOG="$SCRIPTS_DIR/pi-agent-watchdog.sh"
BASH_BIN="$(command -v bash)"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/ob1-pi-watchdog-selftest-XXXXXX")"
cleanup() { rm -rf "$TMP"; return 0; }
trap cleanup EXIT

pass=0
fail=0

check() { # <desc> <expected> <actual>
  if [ "$2" = "$3" ]; then
    pass=$((pass + 1)); printf '  ok   %s\n' "$1"
  else
    fail=$((fail + 1)); printf '  FAIL %s\n       expected: %s\n       actual:   %s\n' "$1" "$2" "$3"
  fi
}

check_contains() { # <desc> <needle> <file>
  if [ -f "$3" ] && grep -qF -- "$2" "$3"; then
    pass=$((pass + 1)); printf '  ok   %s\n' "$1"
  else
    fail=$((fail + 1)); printf '  FAIL %s\n       missing: %s\n       in:      %s\n' "$1" "$2" "$3"
    [ -f "$3" ] && sed 's/^/       | /' "$3"
  fi
}

check_not_contains() { # <desc> <needle> <file>
  if [ -f "$3" ] && grep -qF -- "$2" "$3"; then
    fail=$((fail + 1)); printf '  FAIL %s (unexpectedly present: %s)\n' "$1" "$2"
  else
    pass=$((pass + 1)); printf '  ok   %s\n' "$1"
  fi
}

# ── fixture: a temp workspace shaped like /tmp/pi ────────────────────────────
# <dir> <rel>...  where <rel> is a workspace package path under packages/
# (e.g. pi-tui, session-backends/sqlite-node). Each gets a package.json that
# DECLARES a runtime entry point, an index.js, and the relative scope link that
# npm install would create. Presence artifacts (cli.js, node_modules/.bin, an
# executable wrapper) are always laid down so the presence stage passes and the
# arm can only be decided by the resolution stage.
make_fixture() {
  local dir="$1"; shift
  mkdir -p "$dir/packages/coding-agent/dist" "$dir/node_modules/.bin" \
           "$dir/node_modules/@earendil-works" "$dir/bin"
  printf '{"name":"pi-fixture-root","workspaces":["packages/*","packages/session-backends/*"]}\n' > "$dir/package.json"
  printf '// presence artifact\n' > "$dir/packages/coding-agent/dist/cli.js"
  printf '#!/bin/sh\nexit 0\n' > "$dir/node_modules/.bin/pi-agent"
  printf '#!/bin/sh\nexit 0\n' > "$dir/bin/pi-agent"
  chmod +x "$dir/node_modules/.bin/pi-agent" "$dir/bin/pi-agent"
  local rel name
  for rel in "$@"; do
    name="${rel##*/}"
    mkdir -p "$dir/packages/$rel"
    printf '{"name":"@earendil-works/%s","version":"0.0.0","main":"index.js"}\n' "$name" \
      > "$dir/packages/$rel/package.json"
    printf 'module.exports = {};\n' > "$dir/packages/$rel/index.js"
    ln -s "../../packages/$rel" "$dir/node_modules/@earendil-works/$name"
  done
}

new_arm() { # <name> -> FIX
  ARM="$1"; FIX="$TMP/$1"; mkdir -p "$FIX"
}

# Run a probe copy against a fixture. Output -> $OUT, exit status -> $OUT_RC.
run_probe() { # <script> <fixture-dir>
  OUT="$TMP/last-out.txt"
  ( PI_DIR="$2" PI_AGENT_WATCHDOG_STAMP="$2/watchdog.stamp" \
    PI_AGENT_WRAPPER="$2/bin/pi-agent" bash "$1" ) > "$OUT" 2>&1
  OUT_RC=$?
}

stamp_state() { # <fixture-dir>
  if [ -f "$1/watchdog.stamp" ]; then echo present; else echo absent; fi
}

printf 'pi-agent watchdog self-test (no network; temp fixtures + env overrides)\n'
printf 'probe=%s\n' "$WATCHDOG"

# ══ ARM 1 — healthy fixture is silent ════════════════════════════════════════
new_arm arm1-healthy
make_fixture "$FIX" pi-tui
run_probe "$WATCHDOG" "$FIX"
printf '\nARM 1 — healthy fixture: exit 0, nothing on the alert channel\n'
check "exit code 0" "0" "$OUT_RC"
check "no alert emitted" "" "$(cat "$OUT")"
check "no stamp written" "absent" "$(stamp_state "$FIX")"

# ══ ARM 2 — vanished workspace link (the 2026-09-18 incident) ════════════════
new_arm arm2-link-vanished
make_fixture "$FIX" pi-tui chord
rm "$FIX/node_modules/@earendil-works/pi-tui"
run_probe "$WATCHDOG" "$FIX"
printf '\nARM 2 — vanished scope link: non-zero, names the package\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "flagged as the solve path breaking" "pi-agent solve path BROKEN" "$OUT"
check_contains "names the vanished package" "@earendil-works/pi-tui" "$OUT"
check_contains "carries the node failure class" "ERR_MODULE_NOT_FOUND" "$OUT"
check_contains "carries the re-link remedy" "npm install --ignore-scripts" "$OUT"
check_not_contains "does not name the healthy sibling" "@earendil-works/chord" "$OUT"

# ══ ARM 2b — package DIRECTORY deleted (link left dangling) ══════════════════
new_arm arm2b-target-deleted
make_fixture "$FIX" pi-tui
rm -rf "$FIX/packages/pi-tui"
printf '\nARM 2b — package directory deleted (declaration gone, link dangling)\n'
check "dangling scope link is still present as an entry" "yes" \
  "$([ -L "$FIX/node_modules/@earendil-works/pi-tui" ] && echo yes || echo no)"
run_probe "$WATCHDOG" "$FIX"
check "exit code 1" "1" "$OUT_RC"
check_contains "still names the package" "@earendil-works/pi-tui" "$OUT"

# ══ ARM 2c — every unresolved package is reported ════════════════════════════
new_arm arm2c-reports-all
make_fixture "$FIX" pi-tui session-backends/sqlite-node
rm "$FIX/node_modules/@earendil-works/pi-tui"
rm -rf "$FIX/packages/session-backends/sqlite-node"
run_probe "$WATCHDOG" "$FIX"
printf '\nARM 2c — several packages broken at once: all of them named (both roots)\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "names the vanished link (root packages/*)" "@earendil-works/pi-tui" "$OUT"
check_contains "names the deleted dir (root packages/session-backends/*)" "@earendil-works/sqlite-node" "$OUT"
check_contains "both listed in one alert" "unresolved: @earendil-works/pi-tui, @earendil-works/sqlite-node" "$OUT"

# ══ ARM 3 — NEUTER negative control ══════════════════════════════════════════
printf '\nARM 3 — NEUTER negative control (flip the resolution lever in a probe COPY)\n'
mkdir -p "$TMP/neuter"
NEUTER="$TMP/neuter/pi-agent-watchdog-neutered.sh"
sed 's/^resolution_stage=1$/resolution_stage=0/' "$WATCHDOG" > "$NEUTER"
if cmp -s "$WATCHDOG" "$NEUTER"; then
  check "neuter sed matched the lever line (else this arm proves nothing)" "changed" "unchanged"
else
  check "neuter sed matched the lever line (else this arm proves nothing)" "changed" "changed"
fi
check "neutered copy differs from the tracked probe" "1" "$(grep -c '^resolution_stage=0$' "$NEUTER")"
# Same broken fixture, first the tracked probe (control), then the neutered copy.
rm -f "$FIX/watchdog.stamp"
run_probe "$WATCHDOG" "$FIX"
check "control: tracked probe fails the broken fixture" "1" "$OUT_RC"
check_contains "control: tracked probe alerts" "solve path BROKEN" "$OUT"
run_probe "$NEUTER" "$FIX"
check "neutered copy: broken fixture now exits 0 (arm 2 is conditional)" "0" "$OUT_RC"
check "neutered copy: broken fixture now prints nothing" "" "$(cat "$OUT")"

# ══ ARM 4 — state dedup + healthy run clears the stamp ══════════════════════
new_arm arm4-dedup
make_fixture "$FIX" pi-tui
run_probe "$WATCHDOG" "$FIX"
check "healthy baseline leaves no stamp" "absent" "$(stamp_state "$FIX")"
rm "$FIX/node_modules/@earendil-works/pi-tui"
run_probe "$WATCHDOG" "$FIX"
cat "$OUT" > "$TMP/dedup-combined.txt"
check "first broken run alerts" "1" "$(grep -c 'ALERT' "$OUT")"
check "first broken run writes the stamp" "present" "$(stamp_state "$FIX")"
run_probe "$WATCHDOG" "$FIX"
cat "$OUT" >> "$TMP/dedup-combined.txt"
check "second broken run exits 1 too" "1" "$OUT_RC"
check "second broken run is silent (state dedup)" "" "$(cat "$OUT")"
printf '\nARM 4 — one alert per incident, cleared by a healthy run\n'
check "exactly one alert across both broken runs" "1" "$(grep -c 'ALERT' "$TMP/dedup-combined.txt")"
ln -s ../../packages/pi-tui "$FIX/node_modules/@earendil-works/pi-tui"
run_probe "$WATCHDOG" "$FIX"
check "healthy run after repair exits 0" "0" "$OUT_RC"
check "healthy run clears the stamp" "absent" "$(stamp_state "$FIX")"

# ══ ARM 5 — missing 'node' fails loud, never silently healthy ════════════════
new_arm arm5-node-missing
make_fixture "$FIX" pi-tui
mkdir -p "$FIX/stubbin"
for tool in ls rm cat date timeout; do ln -s "$(command -v "$tool")" "$FIX/stubbin/$tool"; done
OUT="$TMP/last-out.txt"
( env -i PATH="$FIX/stubbin" PI_DIR="$FIX" PI_AGENT_WATCHDOG_STAMP="$FIX/watchdog.stamp" \
    PI_AGENT_WRAPPER="$FIX/bin/pi-agent" "$BASH_BIN" "$WATCHDOG" ) > "$OUT" 2>&1
OUT_RC=$?
printf '\nARM 5 — node not on PATH: fails loud (probe error named, not a healthy pass)\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "names the missing probe tool" "probe tool 'node' is not on PATH" "$OUT"
check_contains "refuses to report health" "solve path UNVERIFIABLE" "$OUT"

# ══ ARM 6 — nothing to enumerate fails loud ══════════════════════════════════
new_arm arm6-nothing-to-enumerate
mkdir -p "$FIX/packages/coding-agent/dist" "$FIX/node_modules/.bin" "$FIX/bin"
printf '{"name":"pi-fixture-root"}\n' > "$FIX/package.json"
printf '// presence artifact\n' > "$FIX/packages/coding-agent/dist/cli.js"
printf '#!/bin/sh\nexit 0\n' > "$FIX/node_modules/.bin/pi-agent"
printf '#!/bin/sh\nexit 0\n' > "$FIX/bin/pi-agent"
chmod +x "$FIX/node_modules/.bin/pi-agent" "$FIX/bin/pi-agent"
run_probe "$WATCHDOG" "$FIX"
printf '\nARM 6 — no workspace declarations and no scope links: probe error\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "names the enumeration failure" "workspace-package enumeration failed" "$OUT"
check_contains "does not claim health" "solve path UNVERIFIABLE" "$OUT"

# ══ ARM 7 — the pre-existing presence class still alerts ═════════════════════
new_arm arm7-presence-preserved
make_fixture "$FIX" pi-tui
rm "$FIX/packages/coding-agent/dist/cli.js"
run_probe "$WATCHDOG" "$FIX"
printf '\nARM 7 — hollow-wipe presence class unchanged\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "hollow-wipe alert preserved" "pi-agent binary UNHEALTHY (hollow-wipe class)" "$OUT"
check_contains "reports the failed presence check" "cli.js=0" "$OUT"
check_contains "carries the rebuild recipe" "npm install --ignore-scripts && npm run build" "$OUT"

# ══ summary ═════════════════════════════════════════════════════════════════
total=$((pass + fail))
printf '\n──────────────────────────────────────────────\n'
printf 'pi-agent-watchdog self-test: %s/%s checks passed' "$pass" "$total"
if [ "$fail" -eq 0 ]; then
  printf ' — ALL GREEN\n'
  exit 0
fi
printf ' — %s FAILED\n' "$fail"
exit 1
