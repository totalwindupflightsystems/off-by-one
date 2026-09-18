#!/usr/bin/env bash
# scripts/publish-catalog.sh — host publish leg of the off-by-one public catalog.
#
# Ships the lab binary + an SQLite snapshot of the answer DB to the catalog host
# as a STAGED PAIR and activates them only after BOTH transfers succeeded:
#
#   .db snapshot -> /opt/off-by-one/off-by-one.db.new
#   binary       -> /opt/off-by-one/off-by-one.new
#   activate     -> chmod +x, mv both, systemctl restart   (single attempt)
#   health       -> remote curl of /api/v1/stats must be 200
#
# Transport semantics (scripts/lib/transport-retry.sh):
#   * a transient ssh/scp hop reset (rc 255 + reset/closed text) retries the
#     WHOLE staged-transfer pair, bounded by TRANSPORT_RETRIES (default 3),
#     because the pair is idempotent (fixed staging names, overwrite);
#   * a non-transport failure (rc 1 — "no space left", a failed remote command)
#     is attempted exactly once and its rc is returned;
#   * a permanent failure (rc 255 + "Permission denied" / host-key errors) is
#     an operator action: no retry, distinct rc, and an explicit recipe;
#   * activation runs ONCE. It is not retried blindly (mv + systemctl restart
#     is not idempotent). Activation itself refuses to touch the live artifact
#     pair unless BOTH staged files exist and are non-empty, so a truncated or
#     stale transfer can never be activated. Only the post-activation health
#     probe (read-only) is retried on the transport class.
#
# The answer-corpus/GitHub half of the distribution lives in the cron entrypoint
# scripts/ob1-distribute.sh (PART 1) and is not touched here.
#
# Modes (OB1_PUBLISH_MODE)
#   host   (default) — publish the binary + DB to OB1_PUBLISH_BOX.
#   static           — explicitly SKIP the host publish. Nothing is silently
#                      redirected: the leg prints exactly which artifacts are
#                      NOT published and which half stays live (the git/GitHub
#                      corpus + static catalog tree, refreshed by
#                      scripts/sync-answers.sh). Operator opt-out for a retired
#                      or unreachable host; never a per-run fallback.
#
# Exit codes
#   0  published and healthy (or static mode / dry-run completed)
#   1  local preparation or non-transport remote failure (nothing activated)
#   2  transport class exhausted the retry budget (nothing activated)
#   3  permanent class — operator action required (auth/key/target)
#   4  activated, but the service did not come back healthy (non-transport)
#   5  configuration error (bad mode, unset/invalid target, missing artifact)
#   6  activation refused: staged artifact pair incomplete or empty
#
# Operator configuration — see docs/publish-transport.md
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/transport-retry.sh
. "$HERE/lib/transport-retry.sh"

EXIT_OK=0
EXIT_LOCAL=1
EXIT_TRANSPORT=2
EXIT_OPERATOR=3
EXIT_HEALTH=4
EXIT_CONFIG=5
EXIT_STAGE=6

MODE="${OB1_PUBLISH_MODE:-host}"
BOX="${OB1_PUBLISH_BOX:-root@78.46.173.180}"
REMOTE_DIR="${OB1_REMOTE_DIR:-/opt/off-by-one}"
SERVICE="${OB1_SERVICE:-off-by-one}"
HEALTH_PORT="${OB1_HEALTH_PORT:-8766}"
HEALTH_PATH="${OB1_HEALTH_PATH:-/api/v1/stats}"
HEALTH_SETTLE="${OB1_HEALTH_SETTLE_SECONDS:-2}"
DRY_RUN="${OB1_PUBLISH_DRY_RUN:-0}"
SQLITE="${OB1_SQLITE:-sqlite3}"
REPO="${OB1_REPO:-$(cd "$HERE/.." && pwd)}"
BINARY="${OB1_BINARY:-$REPO/off-by-one}"
DB="${OB1_DB:-$REPO/off-by-one.db}"

# Fixed staging names — activation only ever reads these two.
STAGE_BIN="$REMOTE_DIR/off-by-one.new"
STAGE_DB="$REMOTE_DIR/off-by-one.db.new"

TMP_DB=""
cleanup() {
  if [ -n "$TMP_DB" ]; then
    rm -f "$TMP_DB"
    TMP_DB=""
  fi
  return 0
}
trap cleanup EXIT

log()  { printf '%s\n' "$*"; }
warn() { printf '%s\n' "$*" >&2; }

# ── configuration ────────────────────────────────────────────────────────────
# Values are interpolated into the remote script body, so they are validated
# rather than escaped: a malformed value fails loudly here.
validate_config() {
  case "$MODE" in
    host|static) ;;
    *) warn "❌ config: OB1_PUBLISH_MODE must be 'host' or 'static' (got '${MODE}')"
       return "$EXIT_CONFIG" ;;
  esac
  # The remote body is generated with single-quoted literals, so a quote or a
  # newline in any interpolated value would change its meaning.
  case "$BOX" in
    *\'*|*[[:space:]]*) warn "❌ config: OB1_PUBLISH_BOX must be user@host with no quotes/whitespace (got '${BOX}')"
       return "$EXIT_CONFIG" ;;
  esac
  case "$REMOTE_DIR" in
    /*) ;;
    *) warn "❌ config: OB1_REMOTE_DIR must be an absolute path (got '${REMOTE_DIR}')"
       return "$EXIT_CONFIG" ;;
  esac
  case "$REMOTE_DIR" in
    *\'*|*[[:space:]]*) warn "❌ config: OB1_REMOTE_DIR must not contain quotes/whitespace (got '${REMOTE_DIR}')"
       return "$EXIT_CONFIG" ;;
  esac
  case "$SERVICE" in
    *\'*|*[[:space:]]*) warn "❌ config: OB1_SERVICE must not contain quotes/whitespace (got '${SERVICE}')"
       return "$EXIT_CONFIG" ;;
  esac
  case "$HEALTH_PORT" in
    ''|*[!0-9]*) warn "❌ config: OB1_HEALTH_PORT must be numeric (got '${HEALTH_PORT}')"
       return "$EXIT_CONFIG" ;;
  esac
  return 0
}

require_artifacts() {
  if [ ! -s "$BINARY" ]; then
    warn "❌ config: binary artifact missing or empty: ${BINARY} (run 'make build')"
    return "$EXIT_CONFIG"
  fi
  if [ ! -s "$DB" ]; then
    warn "❌ config: answer database missing or empty: ${DB}"
    return "$EXIT_CONFIG"
  fi
  return 0
}

print_plan() {
  log "publish leg plan"
  log "  mode        : ${MODE}"
  log "  target      : ${BOX}:${REMOTE_DIR}"
  log "  staging     : $(basename "$STAGE_BIN") + $(basename "$STAGE_DB") (fixed names)"
  log "  activate    : chmod +x + mv both + systemctl restart ${SERVICE} (single attempt)"
  log "  health      : ssh ${BOX} curl http://127.0.0.1:${HEALTH_PORT}${HEALTH_PATH} == 200"
  log "  binary      : ${BINARY}"
  log "  database    : ${DB}"
  log "  retry budget: TRANSPORT_RETRIES=${TRANSPORT_RETRIES:-3} (retries after the first attempt)"
}

# ── static mode ──────────────────────────────────────────────────────────────
# Explicit operator opt-out. This never redirects the binary/DB publish to
# another host or domain — it declares the host publish DISABLED.
mode_static() {
  log "⏭  STATIC-ONLY: host publish DISABLED by operator config (OB1_PUBLISH_MODE=static)"
  log "   NOT published: $(basename "$BINARY") -> ${BOX}:${STAGE_BIN}"
  log "   NOT published: $(basename "$DB") -> ${BOX}:${STAGE_DB}"
  log "   live half    : git/GitHub corpus (data/) + static catalog tree (site/), refreshed by scripts/sync-answers.sh"
  log "   verdict      : STATIC_ONLY (host publish skipped by operator config; no artifact redirected)"
  return 0
}

# ── staged transfers ─────────────────────────────────────────────────────────
snapshot_db() {
  TMP_DB="$(mktemp "${TMPDIR:-/tmp}/ob1-publish-XXXXXX.db")"
  if ! "$SQLITE" "$DB" ".backup '$TMP_DB'" >/dev/null 2>&1; then
    warn "❌ sqlite backup failed (${SQLITE} ${DB} .backup)"
    cleanup
    return "$EXIT_LOCAL"
  fi
  if [ ! -s "$TMP_DB" ]; then
    warn "❌ sqlite backup produced an empty snapshot"
    cleanup
    return "$EXIT_LOCAL"
  fi
  return 0
}

# Both transfers to their fixed staging names. Idempotent: safe to re-run as a
# unit after a transport reset (each attempt overwrites both staged files).
stage_artifacts() {
  scp -q "$BINARY" "$BOX:$STAGE_BIN" || return $?
  scp -q "$TMP_DB" "$BOX:$STAGE_DB" || return $?
  return 0
}

# ── activation (single attempt, all-or-nothing) ──────────────────────────────
activate_artifacts_once() {
  ssh "$BOX" bash -s <<REMOTE
set -eu
bin_new='$STAGE_BIN'
db_new='$STAGE_DB'
if [ ! -s "\$bin_new" ] || [ ! -s "\$db_new" ]; then
  echo "REFUSING to activate: staged artifact pair incomplete or empty (\$bin_new / \$db_new)" >&2
  exit 6
fi
chmod +x "\$bin_new"
mv -f "\$bin_new" '$REMOTE_DIR/off-by-one'
mv -f "\$db_new" '$REMOTE_DIR/off-by-one.db'
systemctl restart '$SERVICE'
REMOTE
}

# ── post-activation health (read-only, safe to retry) ────────────────────────
health_probe() {
  local http="" rc=0
  http="$(ssh "$BOX" "curl -s -m 5 -o /dev/null -w '%{http_code}' http://127.0.0.1:${HEALTH_PORT}${HEALTH_PATH}")" || rc=$?
  if [ "$rc" -ne 0 ]; then
    return "$rc"
  fi
  if [ "$http" != "200" ]; then
    warn "health: ${BOX} returned HTTP '${http}' for http://127.0.0.1:${HEALTH_PORT}${HEALTH_PATH}"
    return "$EXIT_LOCAL"
  fi
  return 0
}

# Classify the last retry_transport verdict into this script's exit code.
classify_exit() {
  case "$TRANSPORT_CLASS" in
    TRANSPORT_PERMANENT) printf '%s' "$EXIT_OPERATOR" ;;
    TRANSPORT_RESET)     printf '%s' "$EXIT_TRANSPORT" ;;
    *)                   printf '%s' "$EXIT_LOCAL" ;;
  esac
}

report_failure() {
  local rc="$1" stage="$2" hint="$3"
  warn "❌ ${stage}: rc=${rc} — ${TRANSPORT_RETRY_VERDICT:-no verdict}"
  [ -n "$hint" ] && warn "   ${hint}"
  if [ "$TRANSPORT_CLASS" = "TRANSPORT_PERMANENT" ]; then
    warn "   operator action required: this target is refusing credentials (not a transient hop)."
    warn "   set OB1_PUBLISH_BOX to a reachable user@host with a working deploy key, or set"
    warn "   OB1_PUBLISH_MODE=static to declare the host publish retired. See docs/publish-transport.md."
  fi
}

# ── host mode ────────────────────────────────────────────────────────────────
mode_host() {
  require_artifacts || return $?

  snapshot_db || return $?

  local rc=0
  if retry_transport "stage $(basename "$BINARY") + db snapshot to ${BOX}" -- stage_artifacts; then
    log "📤 staged pair → ${BOX}:${REMOTE_DIR} ($(basename "$STAGE_BIN"), $(basename "$STAGE_DB"))"
  else
    rc=$?
    report_failure "$rc" "staged transfer failed" "artifacts on the host were NOT replaced (activation skipped)"
    cleanup
    return "$(classify_exit)"
  fi

  # Activation: one attempt. Non-idempotent (mv + systemctl restart) — never
  # retried blindly; the remote body itself refuses an incomplete staged pair.
  # Its output is captured only so the failure can be CLASSIFIED for attribution
  # (a reset mid-activation is reported as transport, an auth failure as
  # operator-action — never as a plain local error).
  local act_out="" rc_act=0
  act_out="$(activate_artifacts_once 2>&1)" || rc_act=$?
  if [ -n "$act_out" ]; then
    printf '%s\n' "$act_out" >&2
  fi
  if [ "$rc_act" -eq 0 ]; then
    log "🚀 activated on ${BOX} (systemctl restart ${SERVICE})"
  else
    rc="$rc_act"
    cleanup
    if [ "$rc" -eq "$EXIT_STAGE" ]; then
      warn "❌ activation refused: staged artifact pair incomplete or empty on ${BOX} — nothing activated"
      return "$EXIT_STAGE"
    fi
    transport_classify "$rc" "$act_out" >/dev/null
    TRANSPORT_RETRY_VERDICT="${TRANSPORT_CLASS} (no retry): activation on ${BOX} failed rc=${rc} — $(_transport_first_line "$act_out")"
    report_failure "$rc" "activation failed" "activation is non-idempotent and was attempted exactly once"
    return "$(classify_exit)"
  fi

  if [ -n "$HEALTH_SETTLE" ] && [ "$HEALTH_SETTLE" != "0" ]; then
    case "$HEALTH_SETTLE" in
      *[!0-9]*) warn "⚠️  OB1_HEALTH_SETTLE_SECONDS='${HEALTH_SETTLE}' is not numeric — skipping settle sleep" ;;
      *) sleep "$HEALTH_SETTLE" ;;
    esac
  fi

  if retry_transport "post-activation health probe on ${BOX}" -- health_probe; then
    log "✅ public catalog healthy on ${BOX} (HTTP 200)"
    cleanup
    return "$EXIT_OK"
  else
    rc=$?
    cleanup
    if [ "$TRANSPORT_CLASS" = "TRANSPORT_RESET" ]; then
      report_failure "$rc" "health probe transport failed" "the artifacts WERE activated; probe the host when the hop recovers"
      return "$EXIT_TRANSPORT"
    fi
    warn "❌ activated on ${BOX} but the service is not healthy — ${TRANSPORT_RETRY_VERDICT:-no verdict}"
    return "$EXIT_HEALTH"
  fi
}

# ── main ─────────────────────────────────────────────────────────────────────
main() {
  validate_config || return $?

  if [ "$DRY_RUN" = "1" ]; then
    print_plan
    log "verdict: DRY_RUN (no ssh/scp performed)"
    return 0
  fi

  case "$MODE" in
    static) mode_static ;;
    host)   mode_host ;;
  esac
}

main "$@"
