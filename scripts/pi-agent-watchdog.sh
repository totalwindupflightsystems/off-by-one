#!/usr/bin/env bash
# pi-agent health watchdog (OB-OPS-004, tick 358; OB-GAP-078, tick 389).
#
# Stage 1 — PRESENCE: the 2026-08-25 hollow-wipe kept /tmp/pi/.git but stripped
# packages/coding-agent/dist/cli.js, root package.json, and emptied
# node_modules/.bin -> 27.5h solver outage (59 instant-fails) invisible to the
# old `ls -d /tmp/pi/.git` probe.
#
# Stage 2 — RESOLUTION (OB-GAP-078): on 2026-09-18 all four presence checks were
# GREEN (git dir intact, package.json present, node_modules/.bin non-empty,
# wrapper executable) while 4 of the 11 npm workspace links under
# node_modules/@earendil-works (pi-tui, pi-agent-core, pi-telemetry, chord) had
# vanished: every solve died in <=1s with
#   ERR_MODULE_NOT_FOUND: Cannot find package '@earendil-works/pi-tui' imported
#   from /tmp/pi/packages/coding-agent/dist/main.js
# The probe therefore asks NODE itself to resolve the workspace packages the
# solve path needs, with cwd=$PI_DIR (the layout the solver runs in), and NAMES
# every package that fails.
#
# Candidate set — deliberately NOT "the symlink entries in
# node_modules/@earendil-works" alone: a vanished link is ABSENT from that
# listing, which is exactly how the 2026-09-18 outage stayed invisible
# (enumerating the surviving 8 links would have probed healthy ones and exited
# 0). The set is derived from two dynamic sources that survive a vanished link:
#   (a) DECLARATIONS — packages/*/package.json and
#       packages/session-backends/*/package.json -> `name`, keeping only those
#       that declare a runtime entry point (`main`/`exports`). Their directory
#       outlives the node_modules link, so the name is still known when the link
#       is gone. packages/evals is private with neither entry field, cannot be
#       imported by contract, and would false-alarm on every run if included.
#   (b) INSTALLED SCOPE LINKS — every entry of node_modules/<scope> that IS a
#       symlink, kept when its target is unreadable (dangling: the package
#       directory itself was deleted, so no declaration remains) or when the
#       target declares a runtime entry point. Keyed by the LINK name, which is
#       the name node resolves.
# Neither source hardcodes a package name, so a 12th workspace package is
# covered automatically. Residual blind spot, stated rather than papered over:
# a package whose directory AND whose scope link were both deleted, which is
# imported by no surviving built code, cannot be named by either source.
#
# THREE OUTCOME STATES (OB-GAP-079) — the resolution stage must tell an
# UNVERIFIED solve path apart from a BROKEN one:
#   ok           — node resolved every enumerated solve-path package. Silent, rc 0.
#   unresolved   — node WAS ASKED and answered with a failure (ERR /
#                  ERR_MODULE_NOT_FOUND — the 2026-09-18 vanished-link class).
#                  Full BROKEN alert + re-link remedy, rc 1.
#   unverifiable — the per-package probe did not complete: a probe hit
#                  `timeout $RESOLVE_TIMEOUT` (node rc=124) because the host was
#                  CPU-starved. That is NOT a wipe and NOT health: one WARN
#                  naming the unverified packages, no rebuild recipe, rc 3.
# Why rc=124 must never land in the resolved-and-failed class: on 2026-09-18
# 15:50:22 (loadavg 159) the probe TIMED OUT on packages whose symlinks were ON
# DISK and the verdict read "pi-agent solve path BROKEN — workspace dep(s)
# unresolved … node TIMEOUT", i.e. the hollow-wipe alert for a merely
# unverifiable outcome. Twelve minutes later, at loadavg 9.33, the same
# packages resolved in 0.10-0.70s. That alert trains its readers to ignore the
# alert that matters. A timeout is therefore kept in its own bucket, the WARN
# names the unverified packages (the gap stays visible), and the operator
# re-runs when load subsides.
#
# Exit codes: 0 = healthy, 1 = broken (presence class, unresolved deps, or an
# unusable probe environment), 3 = unverifiable (the resolution probe timed out).
#
# Silent when healthy (cron no_agent watchdog pattern). Prints one message per
# incident (state-based stamp dedup) when the pi binary is missing, hollowed,
# the wrapper is gone, a solve-path workspace dep no longer resolves, or the
# resolution probe could not complete. Alerts carry the remedy; the
# unverifiable WARN carries the re-run instruction instead.
set -u

PI_DIR="${PI_DIR:-/tmp/pi}"
WRAPPER="${PI_AGENT_WRAPPER:-/home/kara/.local/bin/pi-agent}"
STAMP="${PI_AGENT_WATCHDOG_STAMP:-/tmp/pi-agent-watchdog.stamp}"
# npm workspace packages land in node_modules/<scope>/<pkg>.
WORKSPACE_SCOPE="@earendil-works"
# Per-package resolution budget: a wedged node must not wedge the 15-min cron.
RESOLVE_TIMEOUT="${PI_AGENT_RESOLVE_TIMEOUT:-30}"

# Single-line NEUTER lever for scripts/tests/pi-agent-watchdog-selftest.sh: the
# self-test copies this file and flips THIS line to 0 to prove that its
# broken-fixture arm fails only because of the resolution stage below. The
# tracked value stays 1.
resolution_stage=1

# ── stage 1: presence (OB-OPS-004) ───────────────────────────────────────────
cli_ok=0; pkg_ok=0; bin_ok=0; wrapper_ok=0
[ -s "$PI_DIR/packages/coding-agent/dist/cli.js" ] && cli_ok=1
[ -s "$PI_DIR/package.json" ] && pkg_ok=1
[ -n "$(ls -A "$PI_DIR/node_modules/.bin" 2>/dev/null)" ] && bin_ok=1
[ -x "$WRAPPER" ] && wrapper_ok=1

# ── stage 2: solve-path workspace dep RESOLUTION (OB-GAP-078) ────────────────
resolve_state="ok"; resolve_sig="ok"; resolve_msg=""

if [ "$resolution_stage" = "1" ]; then
  probe_err=""
  for tool in node timeout; do
    if ! command -v "$tool" >/dev/null 2>&1; then
      probe_err="probe tool '$tool' is not on PATH"
      break
    fi
  done

  candidates=""
  if [ -z "$probe_err" ]; then
    candidates="$( cd "$PI_DIR" 2>/dev/null && PI_AGENT_WATCHDOG_SCOPE="$WORKSPACE_SCOPE" timeout "$RESOLVE_TIMEOUT" node -e '
const fs = require("fs"), path = require("path");
const scope = process.env.PI_AGENT_WATCHDOG_SCOPE;
const names = new Set();
let declared = 0, entryLinks = 0, dangling = 0;

// (a) declared workspace packages carrying a runtime entry point.
for (const root of ["packages", "packages/session-backends"]) {
  let entries = [];
  try { entries = fs.readdirSync(root, { withFileTypes: true }); } catch (e) { continue; }
  for (const e of entries) {
    if (!e.isDirectory()) continue;
    let j;
    try { j = JSON.parse(fs.readFileSync(path.join(root, e.name, "package.json"), "utf8")); } catch (e2) { continue; }
    if (!j.name || (!j.main && !j.exports)) continue;
    declared += 1;
    names.add(j.name);
  }
}

// (b) installed scope links; keyed by link name (what node resolves).
let linked = [];
try { linked = fs.readdirSync(path.join("node_modules", scope), { withFileTypes: true }); } catch (e) { linked = []; }
for (const e of linked) {
  let st = null;
  try { st = fs.lstatSync(path.join("node_modules", scope, e.name)); } catch (e2) { continue; }
  if (!st.isSymbolicLink()) continue;
  let j = null;
  try { j = JSON.parse(fs.readFileSync(path.join("node_modules", scope, e.name, "package.json"), "utf8")); } catch (e2) { j = null; }
  if (!j) { dangling += 1; names.add(scope + "/" + e.name); continue; }
  if (j.main || j.exports) { entryLinks += 1; names.add(scope + "/" + e.name); }
}

if (declared === 0 && entryLinks === 0 && dangling === 0) process.exit(3);
console.log([...names].sort().join("\n"));
' 2>/dev/null )"
    enum_rc=$?
    if [ "$enum_rc" -ne 0 ] || [ -z "$candidates" ]; then
      probe_err="workspace-package enumeration failed under $PI_DIR (node rc=$enum_rc) — the solve-path dep set is UNKNOWN"
    fi
  fi

  if [ -n "$probe_err" ]; then
    # UNCHANGED probe-error class (OB-GAP-078): the probe environment itself is
    # unusable, which is actionable — it keeps its ALERT wording and exit 1.
    # The TIMEOUT class below is the one that must NOT be read as actionable.
    resolve_state="probe-error"
    resolve_sig="probe-error"
    resolve_msg="ALERT: pi-agent solve path UNVERIFIABLE (probe error, NOT a healthy pass) — $probe_err. The workspace-dep resolution probe did not run, so solve health is UNKNOWN; fix the probe environment (node + timeout on PATH, $PI_DIR intact) and re-run."
  else
    # Three buckets, not two (OB-GAP-079): a per-package probe that never
    # COMPLETED (node rc=124 = `timeout $RESOLVE_TIMEOUT` killed it under host
    # load) is unverifiable — it must not be counted as a resolved-and-failed
    # package, because that is the hollow-wipe verdict.
    missing=(); codes=(); timed_out=()
    while IFS= read -r name; do
      [ -n "$name" ] || continue
      out="$( cd "$PI_DIR" 2>/dev/null && timeout "$RESOLVE_TIMEOUT" node -e "import('$name').then(()=>{}).catch(e=>{console.error(e.code||'ERR', e.message.split('\n')[0]); process.exit(1)})" 2>&1 )"
      rc=$?
      if [ "$rc" -eq 124 ]; then
        # TIMEOUT: no verdict was produced for this package. UNVERIFIED.
        timed_out+=("$name")
      elif [ "$rc" -ne 0 ]; then
        first="${out%%$'\n'*}"
        code="${first%%[[:space:]]*}"
        [ -n "$code" ] || code="ERR"
        missing+=("$name")
        codes+=("$code")
      fi
    done <<< "$candidates"

    if [ "${#missing[@]}" -gt 0 ]; then
      # Real resolution failure: node answered, and the answer was a failure.
      # Full BROKEN alert exactly as before OB-GAP-079.
      missing_list="$(printf '%s, ' "${missing[@]}")"; missing_list="${missing_list%, }"
      codes_list=""
      for c in "${codes[@]}"; do
        case ", $codes_list, " in *", $c, "*) ;; *) codes_list="${codes_list:+$codes_list, }$c" ;; esac
      done
      resolve_state="unresolved"
      # Dedup signature carries the STATE (which packages are unresolved),
      # never the message text.
      resolve_sig="unresolved:$missing_list"
      resolve_msg="ALERT: pi-agent solve path BROKEN — workspace dep(s) unresolved: $missing_list (node $codes_list). Re-link the workspace packages: cd $PI_DIR && npm install --ignore-scripts (or restore node_modules/$WORKSPACE_SCOPE/<pkg> -> ../../packages/<dir>)."
      if [ "${#timed_out[@]}" -gt 0 ]; then
        # A mixed run cannot claim the broken list is complete: say so rather
        # than let the BROKEN alert read as a full inventory.
        unverified_list="$(printf '%s, ' "${timed_out[@]}")"; unverified_list="${unverified_list%, }"
        resolve_msg="$resolve_msg NOT the full inventory — TIMED OUT under host load, unverified: $unverified_list."
      fi
    elif [ "${#timed_out[@]}" -gt 0 ]; then
      # Every failing package timed out. Nothing was resolved-and-failed, so
      # nothing is known to be broken: the probe did not complete.
      unverified_list="$(printf '%s, ' "${timed_out[@]}")"; unverified_list="${unverified_list%, }"
      resolve_state="unverifiable"
      # Dedup signature carries the STATE (which packages are unverified AND
      # the timeout in force), never the message text — a differently-budgeted
      # probe is a different claim.
      resolve_sig="unverifiable:$unverified_list:timeout=$RESOLVE_TIMEOUT"
      resolve_msg="WARN (not alert): pi-agent resolve probe TIMED OUT under host load (PI_AGENT_RESOLVE_TIMEOUT=$RESOLVE_TIMEOUT) — packages unverified: $unverified_list. Solve health UNKNOWN; re-run when load subsides."
    else
      resolve_state="ok"; resolve_sig="ok"
    fi
  fi
fi

# ── verdict ──────────────────────────────────────────────────────────────────
presence_ok=0
if [ "$cli_ok" = "1" ] && [ "$pkg_ok" = "1" ] && [ "$bin_ok" = "1" ] && [ "$wrapper_ok" = "1" ]; then
  presence_ok=1
fi

# Presence green + resolution not decided in either direction = unverifiable.
# Exit 3, documented in the header: a WARN, not the BROKEN alert, and not rc 0
# either — nothing here may be read as health.
if [ "$presence_ok" = "1" ] && [ "$resolve_state" = "unverifiable" ]; then
  resolve_rc=3
else
  resolve_rc=1
fi

if [ "$presence_ok" = "1" ] && [ "$resolve_state" = "ok" ]; then
  rm -f "$STAMP"
  exit 0
fi

if [ "$presence_ok" = "1" ]; then
  msg="$resolve_msg"
else
  msg="ALERT: pi-agent binary UNHEALTHY (hollow-wipe class) — cli.js=$cli_ok pkg.json=$pkg_ok node_modules/.bin=$bin_ok wrapper=$wrapper_ok. Rebuild: mv $PI_DIR $PI_DIR.bak-$(date +%s) && git clone --depth 1 https://github.com/earendil-works/pi.git $PI_DIR && cd $PI_DIR && npm install --ignore-scripts && npm run build (verify $PI_DIR/packages/coding-agent/dist/cli.js). No server restart needed (bwrap ro-mounts per solve)."
fi

# Dedup signature is the probe STATE, never the message: the rebuild recipe
# embeds `date +%s`, so comparing messages deduped only within the same wall
# second — every 15-min run re-alerted (2026-09-15: 13 alerts / 3h12m for one
# incident). A healthy run clears the stamp, so a recurring incident re-alerts.
# The resolution state is part of the signature, so a TIMEOUT (unverifiable)
# does not re-spam while it persists, and a later real failure still alerts.
sig="cli=$cli_ok pkg=$pkg_ok bin=$bin_ok wrapper=$wrapper_ok resolve=$resolve_sig dir=$PI_DIR wrapper_path=$WRAPPER"
last=""; [ -f "$STAMP" ] && last="$(cat "$STAMP")"
if [ "$last" != "$sig" ]; then
  echo "$sig" > "$STAMP"
  echo "[$(date '+%Y-%m-%d %H:%M:%S %Z')] $msg"
fi
exit "$resolve_rc"
