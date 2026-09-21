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
#   6. the pre-existing presence class (hollow-wipe) still alerts unchanged;
#   7. a TIMEOUT in the per-package resolution probe (host CPU starvation, node
#      rc=124) is UNVERIFIABLE: exit 3, one WARN naming the unverified packages
#      and the budget, NO BROKEN/hollow-wipe verdict and NO rebuild recipe;
#   8. a hard resolution failure (ERR / ERR_MODULE_NOT_FOUND) still produces the
#      full BROKEN alert with the re-link remedy — and when a package times out
#      beside it, the alert says its inventory is incomplete instead of
#      reporting the wedged package as broken.
#
# Exit codes under test (documented in the probe's header): 0 healthy,
# 1 broken, 3 unverifiable.
#
# Fixture base precondition: the deleted-link arms depend on node FAILING to
# resolve a package the fixture does not have, but node walks every ANCESTOR
# node_modules too (/tmp/pi -> /tmp/node_modules -> /node_modules). A host that
# carries /tmp/node_modules/@earendil-works/<pkg> (this box does) silently
# resolves the vanished link and the arms go vacuously green, so the base is
# chosen from candidates whose ancestors are clean and that precondition is
# asserted, never assumed.
#
# Usage: bash scripts/tests/pi-agent-watchdog-selftest.sh   (or: make pi-agent-watchdog-selftest)

set -uo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SELF_DIR/.." && pwd)"
WATCHDOG="$SCRIPTS_DIR/pi-agent-watchdog.sh"
BASH_BIN="$(command -v bash)"
# Mirrors the probe's WORKSPACE_SCOPE (the npm scope the solve path resolves).
WORKSPACE_SCOPE="@earendil-works"

# Fixture base. node resolves a bare specifier by walking node_modules in every
# ancestor of the importer's directory, so a fixture under a base that carries a
# node_modules/@earendil-works/<pkg> resolves packages the fixture does not have
# and the deleted-link arms go vacuously green (live: this host has
# /tmp/node_modules/@earendil-works/pi-tui, installed 2026-09-20).
# Pick the first candidate whose ancestors are clean; never fix the base.
node_modules_ancestor_hit() { # <base> -> prints the offending dir + shim, if any
  local dir="$1" pkg="$2"
  while :; do
    if [ -e "$dir/node_modules/$WORKSPACE_SCOPE/$pkg" ]; then
      printf '%s\n' "$dir/node_modules/$WORKSPACE_SCOPE/$pkg"
      return 0
    fi
    [ "$dir" = "/" ] && return 1
    dir="$(dirname "$dir")"
  done
}

TMP=""
TMP_CANDIDATES=()
[ -n "${TMPDIR:-}" ] && TMP_CANDIDATES+=("$TMPDIR")
TMP_CANDIDATES+=(/var/tmp)
# Deliberately NOT /tmp: /tmp/node_modules is a real ancestor of the fixture and
# of the live /tmp/pi install on this host.
for base in "${TMP_CANDIDATES[@]}"; do
  [ -d "$base" ] && [ -w "$base" ] || continue
  cand="$(mktemp -d "$base/ob1-pi-watchdog-selftest-XXXXXX")" || continue
  hit="$(node_modules_ancestor_hit "$cand" pi-tui || true)"
  if [ -z "$hit" ]; then TMP="$cand"; break; fi
  printf 'skipping fixture base %s: ancestor module shim present (%s)\n' "$base" "$hit"
  rm -rf "$cand"
done

if [ -z "$TMP" ]; then
  printf 'FATAL: no clean fixture base found (every candidate has an ancestor\n' >&2
  printf '       node_modules/%s/<pkg> shim; the deleted-link arms would pass\n' "$WORKSPACE_SCOPE" >&2
  printf '       vacuously). Set TMPDIR to a clean base and re-run.\n' >&2
  exit 1
fi
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
# <script> <fixture-dir> [resolve-timeout]
# The callers of this helper all depend on node actually FAILING to resolve a
# package the fixture does not have; that holds only because $TMP was chosen
# with a clean ancestor chain above (see the base selection) — not because the
# host happens to lack /tmp/node_modules.
run_probe() { # <script> <fixture-dir> [resolve-timeout]
  OUT="$TMP/last-out.txt"
  ( PI_DIR="$2" PI_AGENT_WATCHDOG_STAMP="$2/watchdog.stamp" \
    PI_AGENT_RESOLVE_TIMEOUT="${3:-30}" \
    PI_AGENT_WRAPPER="$2/bin/pi-agent" bash "$1" ) > "$OUT" 2>&1
  OUT_RC=$?
}

# Same, but in a fixture whose stamps are independent of the shared one, for
# arms that need several probes of one tree without the dedup masking a re-run.
run_probe_stamp() { # <script> <fixture-dir> <stamp-path> [resolve-timeout]
  OUT="$TMP/last-out.txt"
  ( PI_DIR="$2" PI_AGENT_WATCHDOG_STAMP="$3" \
    PI_AGENT_RESOLVE_TIMEOUT="${4:-30}" \
    PI_AGENT_WRAPPER="$2/bin/pi-agent" bash "$1" ) > "$OUT" 2>&1
  OUT_RC=$?
}

# A fixture tree carrying a per-run PATH stub dir: every call to the REAL binary
# is real (fixture dir in, exit status out), except the named one, which is
# intercepted. This is stubbing the INNER COMMAND (hermetic, deterministic),
# not the probe's classification logic.
run_probe_stub_node() { # <script> <fixture-dir> <stub-dir> <resolve-timeout>
  OUT="$TMP/last-out.txt"
  ( PATH="$3:$PATH" PI_DIR="$2" PI_AGENT_WATCHDOG_STAMP="$2/watchdog.stamp" \
    PI_AGENT_RESOLVE_TIMEOUT="$4" \
    PI_AGENT_WRAPPER="$2/bin/pi-agent" bash "$1" ) > "$OUT" 2>&1
  OUT_RC=$?
}

# <stub-dir> <name-substring> <rc> — a node stub that intercepts ONE call: the
# per-package import probe for the named package (the argv carrying both
# "import" and that name — the watchdog's own probe shape). Every other
# invocation, notably the workspace-package ENUMERATION, is passed through to
# real node, so only the per-package probe's outcome is synthetic and the
# candidate set stays real.
#
# The pass-through target must be a REAL node, not PATH's `node`: on this host
# PATH's node is a dispatcher (vite-plus `vp`, a symlink to an ELF named `vp`)
# that itself re-resolves `node` from PATH, so `exec "$(command -v node)"` — or
# any candidate check that only looks at the symlink target's magic bytes —
# re-enters the stub. The probe then nested until `timeout` killed it and
# reported a bogus rc=124 on the ENUMERATION (which reads as "the solve-path
# dep set is UNKNOWN"). Select the first PATH candidate whose FULLY RESOLVED
# basename is `node`, and smoke-test the generated stub so a regression here
# fails loudly instead of silently poisoning every arm.
write_node_stub() {
  local stubdir="$1" needle="$2" stubrc="$3" realnode=""
  local cand resolved
  for cand in $(type -ap node 2>/dev/null); do
    resolved="$(readlink -f "$cand" 2>/dev/null)" || continue
    [ -n "$resolved" ] && [ -x "$resolved" ] || continue
    [ "$(basename "$resolved")" = "node" ] || continue
    realnode="$resolved"; break
  done
  [ -n "$realnode" ] || realnode="$(command -v node)"
  mkdir -p "$stubdir"
  cat > "$stubdir/node" <<NODESTUB
#!/bin/sh
# OB-GAP-079 selftest stub: rc $stubrc for the import probe of '$needle'.
for a in "\$@"; do
  case "\$a" in
    *import*'$needle'*) exit $stubrc ;;
  esac
done
exec "$realnode" "\$@"
NODESTUB
  chmod +x "$stubdir/node"

  # Loud self-check: the pass-through must be a one-shot real node, not the stub
  # re-entering itself through PATH.
  local probe_out probe_rc
  probe_out="$(PATH="$stubdir:$PATH" "$stubdir/node" -e 'process.stdout.write("PASSTHROUGH_OK")' 2>&1)"
  probe_rc=$?
  if [ "$probe_rc" -ne 0 ] || [ "$probe_out" != "PASSTHROUGH_OK" ]; then
    printf 'FATAL: node stub pass-through is broken (rc=%s out=%s) — the stub\n' "$probe_rc" "$probe_out" >&2
    printf '       re-enters itself or the target is not a real node; every arm\n' >&2
    printf '       using it would report bogus probe results.\n' >&2
    return 1
  fi
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

# ── ELF-layout fixture (OB-GAP-086): a release install, no npm tree ─────────
# /tmp/pi today is an ELF release install: `pi` is a regular file (a real ELF;
# any executable regular file stands in — the probe checks test -f, the same
# statSync().isFile() the wrapper's findPiBin uses) and there is NO package.json,
# NO node_modules/.bin, NO packages/. Only the wrapper needs to be executable.
make_elf_fixture() {
  local dir="$1"
  mkdir -p "$dir/bin"
  printf '#!/bin/sh\nexit 0\n' > "$dir/pi"
  chmod +x "$dir/pi"
  printf '#!/bin/sh\nexit 0\n' > "$dir/bin/pi-agent"
  chmod +x "$dir/bin/pi-agent"
}

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

# ══ ARM 7b — ELF-layout fixture is healthy (OB-GAP-086) ══════════════════════
# A release install: /tmp/pi/pi regular file + executable wrapper, NO
# package.json, NO node_modules/.bin, NO packages/. This is THIS host's real
# layout — the false-alarm shape ("cli=0 pkg=1 bin=0 wrapper=1
# resolve=probe-error" rc=1) that OB-GAP-086 fixes.
new_arm arm7b-elf-layout-healthy
make_elf_fixture "$FIX"
run_probe "$WATCHDOG" "$FIX"
printf '\nARM 7b — ELF release layout (no npm tree): healthy, silent, rc 0\\n'
check "exit code 0" "0" "$OUT_RC"
check "no alert emitted" "" "$(cat "$OUT")"
check "no stamp written" "absent" "$(stamp_state "$FIX")"

# ══ ARM 7c — neither layout present still alerts (hollow-wipe stays REAL) ════
# Same ELF fixture but the `pi` regular file is MISSING and there is no
# dist/cli.js either: no layout the wrapper can resolve -> the presence alert.
new_arm arm7c-neither-layout-alerts
make_elf_fixture "$FIX"
rm "$FIX/pi"
run_probe "$WATCHDOG" "$FIX"
printf '\nARM 7c — pi file missing AND no cli.js: hollow-wipe alert stays real\\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "hollow-wipe alert preserved" "pi-agent binary UNHEALTHY (hollow-wipe class)" "$OUT"
check_contains "reports the unresolvable pi binary" "pi_binary=0" "$OUT"
check_contains "carries the rebuild recipe" "npm install --ignore-scripts && npm run build" "$OUT"

# ══ ARM 7d — npm layout with dist/cli.js DELETED still fails (board PASS) ════
# The board PASS criterion's second half, verbatim: "while still failing a
# fixture whose dist/cli.js is deleted." On this fixture no ELF `pi` exists
# either (it is an npm-layout workspace), so cli.js was the only resolvable
# layout — its deletion must still alert.
new_arm arm7d-npm-clijs-deleted-still-alerts
make_fixture "$FIX" pi-tui
rm "$FIX/packages/coding-agent/dist/cli.js"
run_probe "$WATCHDOG" "$FIX"
printf '\nARM 7d — npm layout, dist/cli.js deleted (no ELF fallback): rc 1\\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "hollow-wipe alert preserved" "pi-agent binary UNHEALTHY (hollow-wipe class)" "$OUT"
check_contains "reports the failed presence check" "cli.js=0" "$OUT"

# ══ ARM 7e — ELF layout skips stage 2 (resolve state stays ok) ═══════════════
# An ELF binary has no workspace links to resolve, so the node enumeration
# (which would exit 3 / probe-error on this tree) must NEVER run. Proof: the
# probe stays silent and healthy even though this fixture has zero declared
# workspace packages with entry points — the exact shape that used to alert
# with "workspace-package enumeration failed".
new_arm arm7e-elf-skips-stage2
make_elf_fixture "$FIX"
run_probe "$WATCHDOG" "$FIX"
printf '\nARM 7e — ELF layout skips stage 2: no enumeration, resolve stays ok\\n'
check "exit code 0" "0" "$OUT_RC"
check "no alert emitted" "" "$(cat "$OUT")"
check_not_contains "no probe-error text" "UNVERIFIABLE" "$OUT"
check_not_contains "no enumeration failure named" "workspace-package enumeration failed" "$OUT"

# ══ ARM 7f — pkg/bin legs never gate the verdict when the binary resolves ════
# An ELF-layout fixture that ALSO carries an empty node_modules/.bin and no
# package.json (worst-case informational legs) must still be healthy: pkg_ok
# and bin_ok are informational-only parts of the stamp, never gates.
new_arm arm7f-demoted-legs-non-gating
make_elf_fixture "$FIX"
mkdir -p "$FIX/node_modules/.bin"
run_probe "$WATCHDOG" "$FIX"
printf '\nARM 7f — demoted legs (pkg=0 bin=0) never force rc=1 when pi resolves\\n'
check "exit code 0" "0" "$OUT_RC"
check "no alert emitted" "" "$(cat "$OUT")"

# ══ ARM 8 — resolve-probe TIMEOUT is UNVERIFIABLE, not broken (OB-GAP-079) ═══
# The 2026-09-18 15:50:22 outage-class alert: the host was CPU-starved
# (loadavg 159) and `timeout $RESOLVE_TIMEOUT node -e import(...)` hit its
# budget — node rc=124 — for packages whose symlinks were ON DISK. Pre-fix that
# landed in the SAME bucket as ERR_MODULE_NOT_FOUND and the verdict read
# "solve path BROKEN ... node TIMEOUT", i.e. the hollow-wipe alert for a merely
# unverifiable outcome. The inner command is stubbed here (hermetic; the REAL
# node cannot be made to time out deterministically), everything else is the
# probe's own path.
new_arm arm8-timeout-unverifiable
make_fixture "$FIX" pi-tui chord session-backends/sqlite-node
mkdir -p "$FIX/stubbin-timeout"
write_node_stub "$FIX/stubbin-timeout" "pi-tui" 124
printf '\nARM 8 — node rc=124 (probe TIMEOUT under host load): unverifiable, not broken\n'
check "stub node is the one that gets used" "taken" "$(PATH="$FIX/stubbin-timeout:$PATH" command -v node | grep -q "stubbin-timeout" && echo taken || echo real)"
run_probe_stub_node "$WATCHDOG" "$FIX" "$FIX/stubbin-timeout" 7
check "exit code 3 (distinct from broken=1)" "3" "$OUT_RC"
check_contains "WARN, not an alert" "WARN (not alert)" "$OUT"
check_contains "names the timeout as the outcome" "resolve probe TIMED OUT under host load" "$OUT"
check_contains "names the budget in force" "PI_AGENT_RESOLVE_TIMEOUT=7" "$OUT"
check_contains "names the unverified package" "@earendil-works/pi-tui" "$OUT"
check_contains "states the health is unknown" "Solve health UNKNOWN" "$OUT"
check_contains "instructs a re-run" "re-run when load subsides" "$OUT"
check_not_contains "no BROKEN verdict" "solve path BROKEN" "$OUT"
check_not_contains "no hollow-wipe alert" "UNHEALTHY (hollow-wipe class)" "$OUT"
check_not_contains "no re-link recipe" "npm install --ignore-scripts" "$OUT"
check_not_contains "does not name the resolvable siblings" "@earendil-works/chord" "$OUT"

# ══ ARM 8b — the same outcome with NO stub: a real node killed by the budget ═
# Deterministic counterweight to ARM 8: a fixture package whose entry point
# busy-waits past a small PI_AGENT_RESOLVE_TIMEOUT, so `timeout` genuinely
# returns 124 for a REAL node doing a REAL import — no stub in the path at all.
printf '\nARM 8b — real (unstubbed) budget kill: node busy-waits past the timeout\n'
new_arm arm8b-real-timeout
make_fixture "$FIX" pi-tui chord
cat > "$FIX/packages/pi-tui/index.js" <<'SLOWENTRY'
// busy-wait well past the probe budget, then export normally.
const end = Date.now() + 12000;
while (Date.now() < end) {}
module.exports = {};
SLOWENTRY
run_probe "$WATCHDOG" "$FIX" 3
check "real rc=124 outcome exits 3" "3" "$OUT_RC"
check_contains "real timeout reported as the outcome" "resolve probe TIMED OUT under host load" "$OUT"
check_contains "real timeout names the budget" "PI_AGENT_RESOLVE_TIMEOUT=3" "$OUT"
check_contains "real timeout names the wedged package" "@earendil-works/pi-tui" "$OUT"
check_not_contains "real timeout does not read as BROKEN" "solve path BROKEN" "$OUT"
check_not_contains "real timeout carries no rebuild/re-link recipe" "npm install --ignore-scripts" "$OUT"
check_not_contains "the fast sibling is not implicated" "@earendil-works/chord" "$OUT"

# ══ ARM 9 — dedup carries the unverifiable state (no re-spam) ════════════════
printf '\nARM 9 — repeated TIMEOUTs: one WARN, and it is not confused with a failure\n'
# ARM 8 left a stamp behind; clear it so this arm observes the first (alerting)
# timeout run rather than a deduped repeat of someone else's incident.
rm -f "$FIX/watchdog.stamp"
: > "$TMP/arm9-combined.txt"
run_probe_stub_node "$WATCHDOG" "$FIX" "$FIX/stubbin-timeout" 7
cat "$OUT" >> "$TMP/arm9-combined.txt"
check "the timeout run wrote the stamp" "present" "$(stamp_state "$FIX")"
check "stamp records the unverifiable state" "yes" \
  "$(grep -q 'resolve=unverifiable:' "$FIX/watchdog.stamp" && echo yes || echo no)"
run_probe_stub_node "$WATCHDOG" "$FIX" "$FIX/stubbin-timeout" 7
cat "$OUT" >> "$TMP/arm9-combined.txt"
check "second identical TIMEOUT is silent (dedup)" "" "$(cat "$OUT")"
check "second identical TIMEOUT still exits 3" "3" "$OUT_RC"
# A real failure appearing afterwards must NOT be swallowed by the timeout stamp.
rm "$FIX/node_modules/@earendil-works/pi-tui"
run_probe "$WATCHDOG" "$FIX"
cat "$OUT" >> "$TMP/arm9-combined.txt"
check "a later real failure still exits 1" "1" "$OUT_RC"
check_contains "a later real failure still produces the BROKEN alert" "solve path BROKEN" "$OUT"
check "exactly one WARN across the repeated timeout runs" "1" "$(grep -c 'WARN' "$TMP/arm9-combined.txt")"

# ══ ARM 10 — ERR keeps the full BROKEN alert; mixed reports incompleteness ═══
printf '\nARM 10 — hard failure preserved; a timeout beside it is disclosed\n'
new_arm arm10-err-preserved
make_fixture "$FIX" pi-tui chord
rm "$FIX/node_modules/@earendil-works/pi-tui"
run_probe "$WATCHDOG" "$FIX"
check "ERR outcome exits 1 (not 3)" "1" "$OUT_RC"
check_contains "BROKEN alert preserved" "pi-agent solve path BROKEN" "$OUT"
check_contains "names the package" "@earendil-works/pi-tui" "$OUT"
check_contains "carries the node failure class" "ERR_MODULE_NOT_FOUND" "$OUT"
check_contains "carries the re-link remedy" "npm install --ignore-scripts" "$OUT"
check_not_contains "no WARN text on the broken path" "WARN (not alert)" "$OUT"

# Mixed: one package genuinely broken, one timed out. The BROKEN alert must
# still fire (requirement 4) and must NOT present the wedged package as broken.
new_arm arm10b-mixed
make_fixture "$FIX" pi-tui chord
rm "$FIX/node_modules/@earendil-works/pi-tui"
mkdir -p "$FIX/stubbin-mixed"
write_node_stub "$FIX/stubbin-mixed" "chord" 124
run_probe_stub_node "$WATCHDOG" "$FIX" "$FIX/stubbin-mixed" 7
check "mixed run exits 1 (a real failure is present)" "1" "$OUT_RC"
check_contains "mixed run still alerts BROKEN" "solve path BROKEN" "$OUT"
check_contains "the genuinely broken package is the one reported" \
  "unresolved: @earendil-works/pi-tui" "$OUT"
check_not_contains "the timed-out package is NOT called unresolved" \
  "unresolved: @earendil-works/pi-tui, @earendil-works/chord" "$OUT"
check_contains "the timed-out package is disclosed as unverified" \
  "unverified: @earendil-works/chord" "$OUT"

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
