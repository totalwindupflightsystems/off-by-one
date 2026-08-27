#!/usr/bin/env bash
# pi-agent binary health watchdog (OB-OPS-004, tick 358).
# Probes pi-agent WRAPPER RESOLUTION, not mere presence: the 2026-08-25
# hollow-wipe kept /tmp/pi/.git but stripped packages/coding-agent/dist/cli.js,
# root package.json, and emptied node_modules/.bin -> 27.5h solver outage
# (59 instant-fails) invisible to the old `ls -d /tmp/pi/.git` probe.
#
# Silent when healthy (cron no_agent watchdog pattern). Prints an ALERT
# (once per incident, stamp-file dedup) when the pi binary is missing,
# hollowed, or the wrapper is gone. Alert carries the rebuild recipe.
set -u

PI_DIR="${PI_DIR:-/tmp/pi}"
WRAPPER="${PI_AGENT_WRAPPER:-/home/kara/.local/bin/pi-agent}"
STAMP=/tmp/pi-agent-watchdog.stamp

# Core resolution probes (npm layout of earendil-works/pi monorepo).
cli_ok=0; pkg_ok=0; bin_ok=0; wrapper_ok=0
[ -s "$PI_DIR/packages/coding-agent/dist/cli.js" ] && cli_ok=1
[ -s "$PI_DIR/package.json" ] && pkg_ok=1
[ -n "$(ls -A "$PI_DIR/node_modules/.bin" 2>/dev/null)" ] && bin_ok=1
[ -x "$WRAPPER" ] && wrapper_ok=1

if [ "$cli_ok" = "1" ] && [ "$pkg_ok" = "1" ] && [ "$bin_ok" = "1" ] && [ "$wrapper_ok" = "1" ]; then
  rm -f "$STAMP"
  exit 0
fi

msg="ALERT: pi-agent binary UNHEALTHY (hollow-wipe class) — cli.js=$cli_ok pkg.json=$pkg_ok node_modules/.bin=$bin_ok wrapper=$wrapper_ok. Rebuild: mv $PI_DIR $PI_DIR.bak-$(date +%s) && git clone --depth 1 https://github.com/earendil-works/pi.git $PI_DIR && cd $PI_DIR && npm install --ignore-scripts && npm run build (verify $PI_DIR/packages/coding-agent/dist/cli.js). No server restart needed (bwrap ro-mounts per solve)."

last=""; [ -f "$STAMP" ] && last="$(cat "$STAMP")"
if [ "$last" != "$msg" ]; then
  echo "$msg" > "$STAMP"
  echo "[$(date '+%Y-%m-%d %H:%M:%S %Z')] $msg"
fi
exit 1
