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
# Silent when healthy (cron no_agent watchdog pattern). Prints an ALERT (once
# per incident, state-based stamp dedup) when the pi binary is missing,
# hollowed, the wrapper is gone, or a solve-path workspace dep no longer
# resolves. Alert carries the remedy.
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
    resolve_state="probe-error"
    resolve_sig="probe-error"
    resolve_msg="ALERT: pi-agent solve path UNVERIFIABLE (probe error, NOT a healthy pass) — $probe_err. The workspace-dep resolution probe did not run, so solve health is UNKNOWN; fix the probe environment (node + timeout on PATH, $PI_DIR intact) and re-run."
  else
    missing=(); codes=()
    while IFS= read -r name; do
      [ -n "$name" ] || continue
      out="$( cd "$PI_DIR" 2>/dev/null && timeout "$RESOLVE_TIMEOUT" node -e "import('$name').then(()=>{}).catch(e=>{console.error(e.code||'ERR', e.message.split('\n')[0]); process.exit(1)})" 2>&1 )"
      rc=$?
      if [ "$rc" -ne 0 ]; then
        first="${out%%$'\n'*}"
        code="${first%%[[:space:]]*}"
        if [ -z "$code" ]; then
          if [ "$rc" -eq 124 ]; then code="TIMEOUT"; else code="ERR"; fi
        fi
        missing+=("$name")
        codes+=("$code")
      fi
    done <<< "$candidates"

    if [ "${#missing[@]}" -eq 0 ]; then
      resolve_state="ok"; resolve_sig="ok"
    else
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
    fi
  fi
fi

# ── verdict ──────────────────────────────────────────────────────────────────
if [ "$cli_ok" = "1" ] && [ "$pkg_ok" = "1" ] && [ "$bin_ok" = "1" ] && [ "$wrapper_ok" = "1" ] && [ "$resolve_state" = "ok" ]; then
  rm -f "$STAMP"
  exit 0
fi

if [ "$cli_ok" = "1" ] && [ "$pkg_ok" = "1" ] && [ "$bin_ok" = "1" ] && [ "$wrapper_ok" = "1" ]; then
  msg="$resolve_msg"
else
  msg="ALERT: pi-agent binary UNHEALTHY (hollow-wipe class) — cli.js=$cli_ok pkg.json=$pkg_ok node_modules/.bin=$bin_ok wrapper=$wrapper_ok. Rebuild: mv $PI_DIR $PI_DIR.bak-$(date +%s) && git clone --depth 1 https://github.com/earendil-works/pi.git $PI_DIR && cd $PI_DIR && npm install --ignore-scripts && npm run build (verify $PI_DIR/packages/coding-agent/dist/cli.js). No server restart needed (bwrap ro-mounts per solve)."
fi

# Dedup signature is the probe STATE, never the message: the rebuild recipe
# embeds `date +%s`, so comparing messages deduped only within the same wall
# second — every 15-min run re-alerted (2026-09-15: 13 alerts / 3h12m for one
# incident). A healthy run clears the stamp, so a recurring incident re-alerts.
sig="cli=$cli_ok pkg=$pkg_ok bin=$bin_ok wrapper=$wrapper_ok resolve=$resolve_sig dir=$PI_DIR wrapper_path=$WRAPPER"
last=""; [ -f "$STAMP" ] && last="$(cat "$STAMP")"
if [ "$last" != "$sig" ]; then
  echo "$sig" > "$STAMP"
  echo "[$(date '+%Y-%m-%d %H:%M:%S %Z')] $msg"
fi
exit 1
