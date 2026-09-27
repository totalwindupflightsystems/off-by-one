#!/usr/bin/env bash
# scripts/tests/check-corpus-hygiene-selftest.sh — non-vacuity proof for
# scripts/check-corpus-hygiene.sh (REVIEW-OB-006).
#
# A guard that never fires is worthless, so this test runs it against temp
# fixture trees via CORPUS_HYGIENE_ROOT and proves both directions:
#   1. a data/ fixture carrying /home/kara/... FAILS (exit 1, file named);
#   2. a site/ fixture carrying /home/runner/... FAILS (both scan roots covered);
#   3. a fixture carrying /home/bunker-<hex>/... FAILS (generic account class);
#   4. a clean fixture (~/ and /usr/bin text only) PASSES (exit 0);
#   5. an EMPTY fixture passes only because there is nothing to scan — the
#      dirty arms above are what prove the guard is not vacuous.
#
# Fixture strings are plain paths — no key-shaped strings.
# Temp fixtures only; the repo tree is never read or written.
#
# Usage: bash scripts/tests/check-corpus-hygiene-selftest.sh
#        (or: make check-corpus-hygiene-selftest)

set -uo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GUARD="$(cd "$SELF_DIR/.." && pwd)/check-corpus-hygiene.sh"

TMP="$(mktemp -d "${TMPDIR:-/tmp}/ob1-corpus-hygiene-selftest-XXXXXX")"
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

run_guard() { # <fixture-root> -> $OUT, $OUT_RC
  OUT="$TMP/last-out.txt"
  CORPUS_HYGIENE_ROOT="$1" bash "$GUARD" > "$OUT" 2>&1
  OUT_RC=$?
}

printf 'check-corpus-hygiene self-test (temp fixtures via CORPUS_HYGIENE_ROOT)\n'
printf 'guard=%s\n' "$GUARD"

# ── ARM 1 — data/ leak fails, offending file named ─────────────────────────
FIX="$TMP/arm1-data-leak"
mkdir -p "$FIX/data/answers" "$FIX/site/classes"
printf '{"solution":"edit /home/kara/x then run it"}\n' > "$FIX/data/answers/0001-leak.json"
printf '<html>clean ~/text only</html>\n' > "$FIX/site/classes/clean.html"
run_guard "$FIX"
printf '\nARM 1 — /home/kara leak under data/: exit 1, file named\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "names the offending file" "0001-leak.json" "$OUT"
check_contains "prints the remedy" "never hand-edit" "$OUT"

# ── ARM 2 — site/ leak fails (both scan roots are covered) ─────────────────
FIX="$TMP/arm2-site-leak"
mkdir -p "$FIX/data/answers" "$FIX/site/classes"
printf '{"solution":"clean /usr/bin text"}\n' > "$FIX/data/answers/0001-clean.json"
printf '<pre>ran as /home/runner/work/shop</pre>\n' > "$FIX/site/classes/leak.html"
run_guard "$FIX"
printf '\nARM 2 — /home/runner leak under site/: exit 1, file named\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "names the offending file" "leak.html" "$OUT"

# ── ARM 3 — generic account class fails (bunker-<hex>) ─────────────────────
FIX="$TMP/arm3-bunker-leak"
mkdir -p "$FIX/data"
printf 'home dir /home/bunker-deadbeef was left behind\n' > "$FIX/data/INDEX.md"
run_guard "$FIX"
printf '\nARM 3 — /home/bunker-<hex> leak: exit 1\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "names the offending file" "INDEX.md" "$OUT"

# ── ARM 4 — clean fixture passes (~/ and /usr/bin are not leaks) ───────────
FIX="$TMP/arm4-clean"
mkdir -p "$FIX/data/answers" "$FIX/site/classes"
printf '{"solution":"edit ~/x then run /usr/bin/real-tool"}\n' > "$FIX/data/answers/0001-clean.json"
printf '<html>see $HOME/.config and /etc/hosts</html>\n' > "$FIX/site/classes/clean.html"
run_guard "$FIX"
printf '\nARM 4 — clean fixture: exit 0, OK line printed\n'
check "exit code 0" "0" "$OUT_RC"
check_contains "prints the OK line" "corpus hygiene OK" "$OUT"

# ── ARM 5 — real corpus is clean right now (guard runs against the repo) ───
run_guard "$(cd "$SELF_DIR/../.." && pwd)"
printf '\nARM 5 — the committed repo tree itself passes the guard\n'
check "exit code 0" "0" "$OUT_RC"

# ── summary ────────────────────────────────────────────────────────────────
total=$((pass + fail))
printf '\n──────────────────────────────────────────────\n'
printf 'check-corpus-hygiene self-test: %s/%s checks passed' "$pass" "$total"
if [ "$fail" -eq 0 ]; then
  printf ' — ALL GREEN\n'
  exit 0
fi
printf ' — %s FAILED\n' "$fail"
exit 1
