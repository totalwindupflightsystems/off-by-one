#!/usr/bin/env bash
# scripts/lib/transport-retry.sh — classify ssh/scp failures and retry ONLY the
# transient transport class, with a bounded budget.
#
# Why this exists: the public-catalog publish leg (scripts/publish-catalog.sh)
# is a sequence of ssh/scp hops. A mid-flight hop reset (`Connection reset by
# peer` / `scp: Connection closed`, ssh/scp rc 255) is transient and must be
# retried; a remote command failure (rc 1: "no space left on device", a failed
# `systemctl restart`, a bad health probe) is NOT transport and must never be
# retried — retrying everything hides real errors behind delays, and retrying
# nothing truncates a deploy on a flaky hop. Classify first, then retry only
# the transport class, and keep the last rc for attribution.
#
# Sourced, never executed. No side effects at source time.
#
# API
#   transport_classify <rc> <text>
#       Prints and sets TRANSPORT_CLASS to one of:
#         OK                 — rc 0
#         TRANSPORT_RESET    — retryable: rc 255 + transient/closed/reset text,
#                              or rc 255 with no diagnostic text at all
#         TRANSPORT_PERMANENT— rc 255 + a known fatal text (auth denied, host key
#                              verification failed, key changed): an operator
#                              action, NOT a transient hop. Never retried.
#         NON_TRANSPORT      — any rc other than 255 (an ordinary remote command
#                              failure). Never retried.
#   retry_transport <label> -- <cmd> [args...]
#       Runs <cmd> … and retries the whole command only while the failure
#       classifies as TRANSPORT_RESET. Relays the command's output to stderr,
#       returns the LAST rc, and sets:
#         TRANSPORT_RETRY_ATTEMPTS  number of attempts made
#         TRANSPORT_RETRY_VERDICT   human-readable class + attempt accounting
#
# Environment knobs
#   TRANSPORT_RETRIES       retries AFTER the first attempt (default 3)
#   TRANSPORT_BACKOFF_BASE  first backoff seconds (default 2; 0 = no sleeping)
#   TRANSPORT_BACKOFF_MAX   backoff ceiling seconds (default 8)
#
# The wrapped command MUST be idempotent: a transport reset can happen after
# the remote side executed the command, so the retry re-runs it. Retry the
# whole idempotent body (e.g. the full staged transfer pair), never a
# non-idempotent activation step.

# Failure classes (also the values of TRANSPORT_CLASS).
TRANSPORT_CLASS=""
TRANSPORT_RETRY_VERDICT=""
TRANSPORT_RETRY_ATTEMPTS=0

# ssh/scp diagnostics that mean "the hop died mid-flight" — retryable.
TRANSPORT_TRANSIENT_RE='connection reset by peer|connection closed|connection to [^ ]* closed by remote host|read from remote host|connection timed out|operation timed out|connection refused|broken pipe|software caused connection abort|kex_exchange_identification|no route to host|network is unreachable|temporary failure in name resolution|timeout, server not responding|mux_client_read_packet'

# ssh/scp diagnostics that are FATAL and reproducible — an operator action, not
# a flaky hop. Checked BEFORE the transient list so "Permission denied" is not
# mistaken for a reset (the live legacy-box signature, OB-GAP-065).
TRANSPORT_PERMANENT_RE='permission denied|authentication failed|no supported authentication methods|host key verification failed|remote host identification has changed|offending .* key|bad permissions|too many authentication failures'

# transport_classify <rc> <text> — prints the class, sets TRANSPORT_CLASS.
transport_classify() {
  local rc="$1" text="${2:-}"
  local lower
  lower="$(printf '%s' "$text" | tr '[:upper:]' '[:lower:]')"

  if [ "$rc" -eq 0 ]; then
    TRANSPORT_CLASS="OK"
  elif [ "$rc" -ne 255 ]; then
    # 255 is the ssh/scp transport-failure code. Anything else (1, 2, 6, …) is
    # the remote command's own exit status: never retried.
    TRANSPORT_CLASS="NON_TRANSPORT"
  elif printf '%s' "$lower" | grep -Eq "$TRANSPORT_PERMANENT_RE"; then
    TRANSPORT_CLASS="TRANSPORT_PERMANENT"
  elif printf '%s' "$lower" | grep -Eq "$TRANSPORT_TRANSIENT_RE"; then
    TRANSPORT_CLASS="TRANSPORT_RESET"
  else
    # Bare rc 255 with no recognized diagnostic is still a transport death.
    TRANSPORT_CLASS="TRANSPORT_RESET"
  fi

  printf '%s\n' "$TRANSPORT_CLASS"
}

# _transport_is_retryable — the single retry decision.
#
# The first line is the NEUTER LEVER: the regression self-test copies this
# file, seds TRANSPORT_TRANSIENT_VERDICT=1 to =0, and asserts that the retry
# stops happening — proving the retry is conditional, not decorative.
_transport_is_retryable() {
  local TRANSPORT_TRANSIENT_VERDICT=1
  [ "$TRANSPORT_TRANSIENT_VERDICT" = "1" ] || return 1
  [ "$TRANSPORT_CLASS" = "TRANSPORT_RESET" ] || return 1
  return 0
}

# _transport_first_line <text> — first non-empty line, for the verdict.
_transport_first_line() {
  printf '%s' "${1:-}" | sed -n '/[^[:space:]]/{p;q;}' | cut -c1-200
}

# _transport_sleep <seconds> — no-op when seconds is 0 or non-numeric.
_transport_sleep() {
  local secs="${1:-0}"
  case "$secs" in
    ''|*[!0-9]*) return 0 ;;
    0) return 0 ;;
  esac
  sleep "$secs"
}

# retry_transport <label> -- <cmd> [args...]
retry_transport() {
  local label="$1"
  shift
  if [ "${1:-}" = "--" ]; then shift; fi
  if [ "$#" -eq 0 ]; then
    TRANSPORT_RETRY_VERDICT="CONFIG: retry_transport needs a command"
    printf '%s\n' "$TRANSPORT_RETRY_VERDICT" >&2
    return 5
  fi

  local max="${TRANSPORT_RETRIES:-3}"
  case "$max" in ''|*[!0-9]*) max=3 ;; esac
  local base="${TRANSPORT_BACKOFF_BASE:-2}"
  case "$base" in ''|*[!0-9]*) base=2 ;; esac
  local cap="${TRANSPORT_BACKOFF_MAX:-8}"
  case "$cap" in ''|*[!0-9]*) cap=8 ;; esac

  local attempt=0 rc=0 out="" delay="$base" total=$((max + 1))
  TRANSPORT_RETRY_ATTEMPTS=0
  TRANSPORT_RETRY_VERDICT=""
  TRANSPORT_CLASS=""

  while :; do
    attempt=$((attempt + 1))
    rc=0
    out="$( "$@" 2>&1 )" || rc=$?
    if [ -n "$out" ]; then
      printf '%s\n' "$out" >&2
    fi

    if [ "$rc" -eq 0 ]; then
      TRANSPORT_RETRY_ATTEMPTS=$attempt
      TRANSPORT_RETRY_VERDICT="OK (attempt ${attempt}/${total}): ${label}"
      return 0
    fi

    # Direct call (NOT class="$(transport_classify …)"): a command
    # substitution would run in a subshell and discard TRANSPORT_CLASS, which
    # would make the retry decision unconditional.
    transport_classify "$rc" "$out" >/dev/null

    if ! _transport_is_retryable; then
      TRANSPORT_RETRY_ATTEMPTS=$attempt
      TRANSPORT_RETRY_VERDICT="${TRANSPORT_CLASS} (no retry): ${label} failed rc=${rc} on attempt ${attempt} of ${total} — $(_transport_first_line "$out")"
      printf 'transport-retry: %s\n' "$TRANSPORT_RETRY_VERDICT" >&2
      return "$rc"
    fi

    if [ "$attempt" -ge "$total" ]; then
      TRANSPORT_RETRY_ATTEMPTS=$attempt
      TRANSPORT_RETRY_VERDICT="TRANSPORT_RESET after ${max} retry attempt(s) (${attempt} total, budget ${max}): ${label} failed rc=${rc} — $(_transport_first_line "$out")"
      printf 'transport-retry: %s\n' "$TRANSPORT_RETRY_VERDICT" >&2
      return "$rc"
    fi

    printf 'transport-retry: TRANSPORT_RESET on attempt %s/%s (%s) — retrying in %ss: %s\n' \
      "$attempt" "$total" "$label" "$delay" "$(_transport_first_line "$out")" >&2
    _transport_sleep "$delay"
    delay=$(( base == 0 ? 0 : delay * 2 ))
    if [ "$delay" -gt "$cap" ]; then delay="$cap"; fi
  done
}
