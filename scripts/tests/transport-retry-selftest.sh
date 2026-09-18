#!/usr/bin/env bash
# scripts/tests/transport-retry-selftest.sh — deterministic regression self-test
# for scripts/lib/transport-retry.sh and scripts/publish-catalog.sh.
#
# No network, no credentials, no real host: every ssh/scp call is a PATH shim
# that replays a scripted sequence of exit statuses and records its argv, its
# stdin, and its call count. Temp directories only.
#
# What it proves (acceptance for OB-GAP-065):
#   1. one transient reset recovers (rc 0, second attempt);
#   2. a persistent reset returns nonzero after the configured bound, with the
#      transport class + attempt accounting in the verdict;
#   3. a non-transport failure (rc 1 / rc 6) is attempted exactly once and its
#      rc is preserved;
#   4. a clean run is one attempt;
#   5. the retry path is CONDITIONAL — the NEUTER arm flips the single-line
#      lever in a copy of the library and the retry stops happening;
#   6. the publish flow never activates a partial/stale artifact pair: a
#      persistent transfer failure performs zero activation calls, the staged
#      names are fixed, activation runs exactly once, and the remote activation
#      body itself refuses an incomplete staged pair;
#   7. static mode performs no ssh/scp at all and names what is NOT published;
#   8. non-transport activation/health failures are not retried.
#
# Usage: bash scripts/tests/transport-retry-selftest.sh   (or: make transport-retry-selftest)

set -uo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SELF_DIR/.." && pwd)"
LIB="$SCRIPTS_DIR/lib/transport-retry.sh"
PUBLISH="$SCRIPTS_DIR/publish-catalog.sh"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/ob1-transport-selftest-XXXXXX")"
cleanup() { rm -rf "$TMP"; return 0; }
trap cleanup EXIT

SQLITE_BIN="$(command -v sqlite3 || true)"

pass=0
fail=0
arm=""

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
    [ -f "$3" ] && sed -n '1,15p' "$3" | sed 's/^/       | /'
  fi
}

check_not_contains() { # <desc> <needle> <file>
  if [ -f "$3" ] && grep -qF -- "$2" "$3"; then
    fail=$((fail + 1)); printf '  FAIL %s (unexpectedly present: %s)\n' "$1" "$2"
  else
    pass=$((pass + 1)); printf '  ok   %s\n' "$1"
  fi
}

field() { # <key>  — reads KEY=VALUE from $ARM/out
  sed -n "s/^$1=//p" "$ARM/out" | head -1
}

calls() { # <tool>
  grep -c "^CALL $1#" "$SHIM_LOG" 2>/dev/null || true
}

# ── one shim, installed as scp and ssh ───────────────────────────────────────
mkdir -p "$TMP/shim"
cat > "$TMP/shim/transport-shim" <<'SHIM'
#!/usr/bin/env bash
# Deterministic ssh/scp stand-in: replays a per-tool rule file and records every
# invocation (argv, stdin body, call number). Never touches the network.
set -uo pipefail
name="$(basename "$0")"
log="${SHIM_LOG:?SHIM_LOG unset}"
seq_dir="${SHIM_SEQ_DIR:?SHIM_SEQ_DIR unset}"
rules_dir="${SHIM_RULES_DIR:?SHIM_RULES_DIR unset}"

n=0
seq_file="$seq_dir/$name"
[ -s "$seq_file" ] && n="$(cat "$seq_file")"
n=$(( n + 1 ))
printf '%s\n' "$n" > "$seq_file"

{
  printf 'CALL %s#%s' "$name" "$n"
  for a in "$@"; do printf ' [%s]' "$a"; done
  printf '\n'
} >> "$log"

# `ssh host bash -s` ships a remote script body on stdin — record it.
if [ ! -t 0 ]; then
  stdin_file="$log.stdin.$name.$n"
  if timeout 2 cat > "$stdin_file" 2>/dev/null; then
    bytes=$(wc -c < "$stdin_file")
    if [ "$bytes" -gt 0 ]; then
      printf 'STDIN %s#%s bytes=%s\n' "$name" "$n" "$bytes" >> "$log"
    fi
  fi
fi

rule="$(sed -n "${n}p" "$rules_dir/$name" 2>/dev/null)"
[ -n "$rule" ] || rule="$(tail -n 1 "$rules_dir/$name" 2>/dev/null)"

case "$rule" in
  ok)          printf '%s#%s: ok\n' "$name" "$n"; exit 0 ;;
  http200)     printf '200\n'; exit 0 ;;
  http503)     printf '503\n'; exit 0 ;;
  reset)       printf 'scp: Connection closed\n' >&2; exit 255 ;;
  reset-peer)  printf 'Read from remote host catalog.example: Connection reset by peer\n' >&2; exit 255 ;;
  refused)     printf 'ssh: connect to host catalog.example port 22: Connection refused\n' >&2; exit 255 ;;
  perm)        printf 'Permission denied (publickey,password).\n' >&2; exit 255 ;;
  hostkey)     printf 'Host key verification failed.\n' >&2; exit 255 ;;
  bare-255)    exit 255 ;;
  rc6-refuse)  printf 'REFUSING to activate: staged artifact pair incomplete or empty\n' >&2; exit 6 ;;
  rc1-nospace) printf 'scp: /opt/off-by-one/off-by-one.new: No space left on device\n' >&2; exit 1 ;;
  *)           printf 'transport-shim: unknown rule [%s] for %s\n' "$rule" "$name" >&2; exit 99 ;;
esac
SHIM
chmod +x "$TMP/shim/transport-shim"

# ── per-arm scaffolding ──────────────────────────────────────────────────────
new_arm() { # <name>
  arm="$1"
  ARM="$TMP/$arm"
  mkdir -p "$ARM/bin" "$ARM/rules" "$ARM/seq"
  ln -sf "$TMP/shim/transport-shim" "$ARM/bin/scp"
  ln -sf "$TMP/shim/transport-shim" "$ARM/bin/ssh"
  : > "$ARM/rules/scp"
  : > "$ARM/rules/ssh"
  : > "$ARM/shim.log"
  : > "$ARM/out"
  SHIM_LOG="$ARM/shim.log"
  SHIM_SEQ_DIR="$ARM/seq"
  SHIM_RULES_DIR="$ARM/rules"
  export SHIM_LOG SHIM_SEQ_DIR SHIM_RULES_DIR
  # arm-default publish configuration (overridden per arm where needed)
  MODE="host"; RETRIES="3"; DRY_RUN="0"
  BINARY_OVERRIDE=""; DB_OVERRIDE=""; BOX_OVERRIDE="root@catalog.example"
  REMOTE_DIR_OVERRIDE=""; HEALTH_SETTLE_OVERRIDE="0"; BOX_EXTRA=""
}

set_rules() { # <tool> <rule...>  (one per line; the last rule repeats)
  local tool="$1"; shift
  : > "$ARM/rules/$tool"
  local r
  for r in "$@"; do printf '%s\n' "$r" >> "$ARM/rules/$tool"; done
}

prepare_artifacts() { # a real (tiny) sqlite db + a stand-in binary, in $ARM
  printf 'fake-binary-%s\n' "$arm" > "$ARM/off-by-one"
  "$SQLITE_BIN" "$ARM/off-by-one.db" 'CREATE TABLE t(a INTEGER); INSERT INTO t VALUES (1);' >/dev/null
}

# Run the publish script under the shimmed PATH. Sets OUT_RC; output -> $ARM/out.
run_publish() {
  local mode="${MODE:-host}" retries="${RETRIES:-3}" dry="${DRY_RUN:-0}"
  local binary="${BINARY_OVERRIDE:-$ARM/off-by-one}"
  local db="${DB_OVERRIDE:-$ARM/off-by-one.db}"
  local box="${BOX_OVERRIDE:-root@catalog.example}${BOX_EXTRA:-}"
  local remote_dir="${REMOTE_DIR_OVERRIDE:-/opt/off-by-one}"
  local settle="${HEALTH_SETTLE_OVERRIDE:-0}"
  ( env -i \
      PATH="$ARM/bin:/usr/bin:/bin" \
      HOME="$TMP" \
      TMPDIR="$TMP" \
      SHIM_LOG="$SHIM_LOG" SHIM_SEQ_DIR="$SHIM_SEQ_DIR" SHIM_RULES_DIR="$SHIM_RULES_DIR" \
      OB1_BINARY="$binary" OB1_DB="$db" OB1_SQLITE="$SQLITE_BIN" \
      OB1_PUBLISH_MODE="$mode" OB1_PUBLISH_BOX="$box" \
      OB1_REMOTE_DIR="$remote_dir" OB1_HEALTH_SETTLE_SECONDS="$settle" \
      OB1_PUBLISH_DRY_RUN="$dry" \
      TRANSPORT_BACKOFF_BASE=0 TRANSPORT_RETRIES="$retries" \
      bash "$PUBLISH" ) > "$ARM/out" 2>&1
  OUT_RC=$?
}

# Run the library directly (also proves it is safe under set -euo pipefail).
run_retry() { # <retries> <label>
  local retries="$1" label="$2"
  ( set -euo pipefail
    TRANSPORT_BACKOFF_BASE=0 TRANSPORT_RETRIES="$retries"
    # shellcheck source=/dev/null
    . "$LIB"
    rc=0
    retry_transport "$label" -- "$ARM/bin/scp" src.bin "root@catalog.example:/opt/off-by-one/off-by-one.new" || rc=$?
    printf 'RETRY_RC=%s\n' "$rc"
    printf 'VERDICT=%s\n' "$TRANSPORT_RETRY_VERDICT"
    printf 'ATTEMPTS=%s\n' "$TRANSPORT_RETRY_ATTEMPTS"
    printf 'CLASS=%s\n' "$TRANSPORT_CLASS"
    exit "$rc"
  ) > "$ARM/out" 2>&1
  OUT_RC=$?
}

classify_one() { # <rc> <text> -> prints the class
  ( set -euo pipefail
    # shellcheck source=/dev/null
    . "$LIB"
    transport_classify "$1" "$2"
  ) 2>/dev/null
}

printf 'transport-retry self-test (no network; PATH shims + temp dirs)\n'
printf 'lib=%s\npublish=%s\n' "$LIB" "$PUBLISH"

# ══ ARM 1 — one transient reset recovers ═════════════════════════════════════
new_arm arm1-one-reset-recovers
set_rules scp reset ok
run_retry 3 "stage artifacts"
printf '\nARM 1 — one transient reset recovers\n'
check "exit code 0" "0" "$OUT_RC"
check "scp attempts == 2" "2" "$(calls scp)"
check_contains "verdict reports recovery" "OK (attempt 2/4)" "$ARM/out"
check "attempt counter == 2" "2" "$(field ATTEMPTS)"
check_contains "first failure classified as transport reset" "TRANSPORT_RESET on attempt 1/4" "$ARM/out"

# ══ ARM 2 — persistent reset exhausts the bound ══════════════════════════════
new_arm arm2-persistent-reset-bounded
set_rules scp reset
run_retry 3 "stage artifacts"
printf '\nARM 2 — persistent reset exhausts the bound (default budget 3)\n'
check "exit code preserved (255)" "255" "$OUT_RC"
check "scp attempts == budget + 1 (4)" "4" "$(calls scp)"
check "attempt counter == 4" "4" "$(field ATTEMPTS)"
check_contains "verdict names class + attempts" \
  "TRANSPORT_RESET after 3 retry attempt(s) (4 total, budget 3)" "$ARM/out"

# ══ ARM 2b — the budget is operator-configurable ═════════════════════════════
new_arm arm2b-configurable-budget
set_rules scp reset
run_retry 1 "stage artifacts"
printf '\nARM 2b — bounded budget is configurable (TRANSPORT_RETRIES=1)\n'
check "exit code preserved (255)" "255" "$OUT_RC"
check "scp attempts == 2 (1 + 1 retry)" "2" "$(calls scp)"
check_contains "verdict reports the configured budget" "(2 total, budget 1)" "$ARM/out"

# ══ ARM 3 — non-transport failure: exactly one attempt, rc preserved ═════════
new_arm arm3-nontransport-rc1
set_rules scp rc1-nospace
run_retry 3 "stage artifacts"
printf '\nARM 3 — non-transport failure (rc 1) attempted exactly once\n'
check "exit code preserved (1)" "1" "$OUT_RC"
check "scp attempts == 1" "1" "$(calls scp)"
check "class is NON_TRANSPORT" "NON_TRANSPORT" "$(field CLASS)"
check_contains "verdict marks it non-retryable" "NON_TRANSPORT (no retry)" "$ARM/out"

# ══ ARM 3b — non-transport rc 6 is not retried either ════════════════════════
new_arm arm3b-nontransport-rc6
set_rules scp rc6-refuse
run_retry 3 "stage artifacts"
printf '\nARM 3b — non-transport rc 6 (refusal) is not retried\n'
check "exit code preserved (6)" "6" "$OUT_RC"
check "scp attempts == 1" "1" "$(calls scp)"
check "class is NON_TRANSPORT" "NON_TRANSPORT" "$(field CLASS)"

# ══ ARM 4 — clean success is one attempt ═════════════════════════════════════
new_arm arm4-clean-success
set_rules scp ok
run_retry 3 "stage artifacts"
printf '\nARM 4 — clean success is exactly one attempt\n'
check "exit code 0" "0" "$OUT_RC"
check "scp attempts == 1" "1" "$(calls scp)"
check "attempt counter == 1" "1" "$(field ATTEMPTS)"

# ══ ARM 5 — classification table ═════════════════════════════════════════════
new_arm arm5-classification-table
printf '\nARM 5 — classification table (rc + diagnostic -> class)\n'
check_class() { # <rc> <text> <expected>
  local got
  got="$(classify_one "$1" "$2")"
  check "classify(rc=$1, '$(printf '%.44s' "$2")')" "$3" "$got"
}
check_class 0 "" "OK"
check_class 255 "scp: Connection closed" "TRANSPORT_RESET"
check_class 255 "Read from remote host catalog.example: Connection reset by peer" "TRANSPORT_RESET"
check_class 255 "ssh: connect to host catalog.example port 22: Connection refused" "TRANSPORT_RESET"
check_class 255 "ssh: connect to host catalog.example port 22: Connection timed out" "TRANSPORT_RESET"
check_class 255 "kex_exchange_identification: read: Connection reset by peer" "TRANSPORT_RESET"
check_class 255 "" "TRANSPORT_RESET"
check_class 255 "Permission denied (publickey,password)." "TRANSPORT_PERMANENT"
check_class 255 "Host key verification failed." "TRANSPORT_PERMANENT"
check_class 255 "WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED!" "TRANSPORT_PERMANENT"
check_class 1 "No space left on device" "NON_TRANSPORT"
check_class 1 "Connection reset by peer" "NON_TRANSPORT"
check_class 6 "REFUSING to activate: staged artifact pair incomplete" "NON_TRANSPORT"

# ══ ARM 6 — NEUTER negative control: the retry is conditional ════════════════
new_arm arm6-neuter-negative-control
printf '\nARM 6 — NEUTER negative control (flip the single-line lever in a lib copy)\n'
mkdir -p "$ARM/neuter"
NEUTER_LIB="$ARM/neuter/transport-retry.sh"
sed 's/^  local TRANSPORT_TRANSIENT_VERDICT=1$/  local TRANSPORT_TRANSIENT_VERDICT=0/' "$LIB" > "$NEUTER_LIB"
if cmp -s "$LIB" "$NEUTER_LIB"; then
  check "neuter sed matched the lever line (else this arm proves nothing)" "changed" "unchanged"
else
  check "neuter sed matched the lever line (else this arm proves nothing)" "changed" "changed"
fi
set_rules scp reset
( set -euo pipefail
  TRANSPORT_BACKOFF_BASE=0 TRANSPORT_RETRIES=3
  # shellcheck source=/dev/null
  . "$NEUTER_LIB"
  rc=0
  retry_transport "neutered stage" -- "$ARM/bin/scp" src.bin "root@catalog.example:/opt/off-by-one/off-by-one.new" || rc=$?
  printf 'RETRY_RC=%s\nVERDICT=%s\nATTEMPTS=%s\n' "$rc" "$TRANSPORT_RETRY_VERDICT" "$TRANSPORT_RETRY_ATTEMPTS"
  exit "$rc"
) > "$ARM/out" 2>&1
OUT_RC=$?
check "exit code still 255 (fails loudly, never falsely)" "255" "$OUT_RC"
check "scp attempts == 1 (budget unspent => retry was conditional)" "1" "$(calls scp)"
check_contains "verdict reports no retry" "no retry" "$ARM/out"

# ══ ARM 7 — publish: clean run, staged pair, single activation ═══════════════
new_arm arm7-publish-clean
prepare_artifacts
set_rules scp ok ok
set_rules ssh ok http200
run_publish
printf '\nARM 7 — publish (host mode) clean run\n'
check "exit code 0" "0" "$OUT_RC"
check "scp calls == 2 (binary + db)" "2" "$(calls scp)"
check "ssh calls == 2 (activation + health)" "2" "$(calls ssh)"
check_contains "binary staged under the fixed name" \
  "[root@catalog.example:/opt/off-by-one/off-by-one.new]" "$SHIM_LOG"
check_contains "db staged under the fixed name" \
  "[root@catalog.example:/opt/off-by-one/off-by-one.db.new]" "$SHIM_LOG"
check_contains "healthy verdict" "public catalog healthy" "$ARM/out"
check_contains "activation shipped a remote body" "STDIN ssh#1 bytes=" "$SHIM_LOG"
check_contains "remote body refuses an incomplete staged pair" "REFUSING to activate" "$SHIM_LOG.stdin.ssh.1"
check_contains "remote body guards the binary staging name" "off-by-one.new" "$SHIM_LOG.stdin.ssh.1"
check_contains "remote body guards the db staging name" "off-by-one.db.new" "$SHIM_LOG.stdin.ssh.1"

# ══ ARM 8 — publish: one reset on the first transfer -> pair retried ═════════
new_arm arm8-publish-one-reset-recovers
prepare_artifacts
set_rules scp reset ok ok
set_rules ssh ok http200
run_publish
printf '\nARM 8 — publish: one transient reset on the first transfer recovers\n'
check "exit code 0" "0" "$OUT_RC"
check "scp calls == 3 (one retry of the whole pair)" "3" "$(calls scp)"
check "ssh calls == 2 (activation still exactly once)" "2" "$(calls ssh)"
check_contains "retry logged with class" "TRANSPORT_RESET on attempt 1/4" "$ARM/out"
check_contains "healthy verdict" "public catalog healthy" "$ARM/out"

# ══ ARM 9 — publish: persistent reset -> bound exhausted, ZERO activation ════
new_arm arm9-publish-persistent-reset
prepare_artifacts
set_rules scp reset
set_rules ssh ok http200
run_publish
printf '\nARM 9 — publish: persistent transfer reset exhausts the bound\n'
check "exit code 2 (transport budget exhausted)" "2" "$OUT_RC"
check "scp calls == 4 (budget + 1)" "4" "$(calls scp)"
check "ssh calls == 0 (no activation of a partial/stale pair)" "0" "$(calls ssh)"
check_contains "verdict names class + attempts" "TRANSPORT_RESET after 3 retry attempt(s)" "$ARM/out"
check_contains "artifacts explicitly not replaced" "NOT replaced" "$ARM/out"
check_not_contains "no success claim" "public catalog healthy" "$ARM/out"

# ══ ARM 9b — publish: budget honoured in the publish flow ════════════════════
new_arm arm9b-publish-tight-budget
prepare_artifacts
set_rules scp reset
set_rules ssh ok http200
RETRIES="1"
run_publish
printf '\nARM 9b — publish honours TRANSPORT_RETRIES=1\n'
check "exit code 2" "2" "$OUT_RC"
check "scp calls == 2" "2" "$(calls scp)"
check "ssh calls == 0" "0" "$(calls ssh)"

# ══ ARM 10 — publish: non-transport transfer failure ═════════════════════════
new_arm arm10-publish-nontransport
prepare_artifacts
set_rules scp rc1-nospace
set_rules ssh ok http200
run_publish
printf '\nARM 10 — publish: non-transport transfer failure (rc 1) is not retried\n'
check "exit code 1" "1" "$OUT_RC"
check "scp calls == 1" "1" "$(calls scp)"
check "ssh calls == 0 (nothing activated)" "0" "$(calls ssh)"
check_contains "verdict marks it non-retryable" "NON_TRANSPORT (no retry)" "$ARM/out"

# ══ ARM 11 — publish: permanent class (permission denied) ════════════════════
new_arm arm11-publish-permanent
prepare_artifacts
set_rules scp perm
set_rules ssh ok http200
run_publish
printf '\nARM 11 — publish: permanent class (auth denied) -> operator action, one attempt\n'
check "exit code 3 (operator action required)" "3" "$OUT_RC"
check "scp calls == 1 (no pointless retries)" "1" "$(calls scp)"
check "ssh calls == 0" "0" "$(calls ssh)"
check_contains "verdict names the permanent class" "TRANSPORT_PERMANENT" "$ARM/out"
check_contains "operator recipe printed" "operator action required" "$ARM/out"
check_contains "doc pointer printed" "docs/publish-transport.md" "$ARM/out"

# ══ ARM 12 — publish: activation refuses an incomplete staged pair ═══════════
new_arm arm12-activation-refuses-partial-pair
prepare_artifacts
set_rules scp ok ok
set_rules ssh rc6-refuse
run_publish
printf '\nARM 12 — publish: activation refuses an incomplete staged pair (rc 6)\n'
check "exit code 6 (staged pair incomplete, nothing activated)" "6" "$OUT_RC"
check "ssh calls == 1 (activation attempt only, no health probe)" "1" "$(calls ssh)"
check_contains "refusal reported" "activation refused" "$ARM/out"
check_not_contains "no success claim" "public catalog healthy" "$ARM/out"

# ══ ARM 13 — publish: activation is never retried blindly ════════════════════
new_arm arm13-activation-not-retried
prepare_artifacts
set_rules scp ok ok
set_rules ssh reset
run_publish
printf '\nARM 13 — publish: activation transport reset is attempted exactly once\n'
check "exit code 2 (transport class)" "2" "$OUT_RC"
check "ssh calls == 1 (non-idempotent activation not retried)" "1" "$(calls ssh)"
check_contains "one-attempt note printed" "attempted exactly once" "$ARM/out"

# ══ ARM 14 — publish: unhealthy service is a non-transport failure ═══════════
new_arm arm14-health-nontransport
prepare_artifacts
set_rules scp ok ok
set_rules ssh ok http503
run_publish
printf '\nARM 14 — publish: non-200 health probe is not retried (rc 4)\n'
check "exit code 4 (activated but unhealthy)" "4" "$OUT_RC"
check "ssh calls == 2 (activation + one probe)" "2" "$(calls ssh)"
check_contains "unhealthy verdict" "not healthy" "$ARM/out"
check_not_contains "no success claim" "public catalog healthy" "$ARM/out"

# ══ ARM 14b — publish: read-only health probe does retry on transport ════════
new_arm arm14b-health-retries-transport
prepare_artifacts
set_rules scp ok ok
set_rules ssh ok reset http200
run_publish
printf '\nARM 14b — publish: read-only health probe retries a transport reset\n'
check "exit code 0" "0" "$OUT_RC"
check "ssh calls == 3 (activation + 2 probe attempts)" "3" "$(calls ssh)"
check_contains "healthy verdict" "public catalog healthy" "$ARM/out"

# ══ ARM 15 — publish: static mode is an explicit, named opt-out ══════════════
new_arm arm15-static-mode
prepare_artifacts
MODE="static"
run_publish
printf '\nARM 15 — publish: static mode performs no ssh/scp and names what is unpublished\n'
check "exit code 0" "0" "$OUT_RC"
check "scp calls == 0" "0" "$(calls scp)"
check "ssh calls == 0" "0" "$(calls ssh)"
check_contains "mode verdict" "STATIC_ONLY" "$ARM/out"
check_contains "names the unpublished artifacts" "NOT published" "$ARM/out"
check_contains "denies any silent redirect" "no artifact redirected" "$ARM/out"

# ══ ARM 16 — publish: dry-run touches nothing ════════════════════════════════
new_arm arm16-dry-run
prepare_artifacts
DRY_RUN="1"
run_publish
printf '\nARM 16 — publish: dry-run prints the plan and touches nothing\n'
check "exit code 0" "0" "$OUT_RC"
check "scp calls == 0" "0" "$(calls scp)"
check "ssh calls == 0" "0" "$(calls ssh)"
check_contains "dry-run verdict" "DRY_RUN" "$ARM/out"
check_contains "plan names the staging scheme" "fixed names" "$ARM/out"

# ══ ARM 17 — publish: configuration errors fail loudly before any ssh ════════
new_arm arm17-config-errors
prepare_artifacts
printf '\nARM 17 — publish: configuration errors fail loudly (rc 5, no ssh/scp)\n'
MODE="bogus"
run_publish
check "bad mode -> rc 5" "5" "$OUT_RC"
check_contains "bad mode message" "OB1_PUBLISH_MODE must be 'host' or 'static'" "$ARM/out"
MODE="host"
REMOTE_DIR_OVERRIDE="relative/path"
run_publish
check "relative remote dir -> rc 5" "5" "$OUT_RC"
check_contains "relative dir message" "must be an absolute path" "$ARM/out"
REMOTE_DIR_OVERRIDE=""
BOX_EXTRA="'; echo pwned #"
run_publish
check "quote/space injection in box -> rc 5" "5" "$OUT_RC"
check_contains "injection message" "no quotes/whitespace" "$ARM/out"
BOX_EXTRA=""
BINARY_OVERRIDE="$ARM/does-not-exist"
run_publish
check "missing binary -> rc 5" "5" "$OUT_RC"
check_contains "missing binary message" "binary artifact missing" "$ARM/out"
BINARY_OVERRIDE=""
check "no ssh calls across all config errors" "0" "$(calls ssh)"
check "no scp calls across all config errors" "0" "$(calls scp)"

# ══ summary ═════════════════════════════════════════════════════════════════
total=$((pass + fail))
printf '\n──────────────────────────────────────────────\n'
printf 'transport-retry self-test: %s/%s checks passed' "$pass" "$total"
if [ "$fail" -eq 0 ]; then
  printf ' — ALL GREEN\n'
  exit 0
fi
printf ' — %s FAILED\n' "$fail"
exit 1
