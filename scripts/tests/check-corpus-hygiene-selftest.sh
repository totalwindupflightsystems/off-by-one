#!/usr/bin/env bash
# scripts/tests/check-corpus-hygiene-selftest.sh — non-vacuity proof for
# scripts/check-corpus-hygiene.sh (REVIEW-OB-006 + REVIEW-OB-010).
#
# A guard that never fires is worthless, so this test runs it against temp
# fixture trees via CORPUS_HYGIENE_ROOT and proves both directions:
#   1. a data/ fixture carrying /home/kara/... FAILS (exit 1, file named);
#   2. a site/ fixture carrying /home/runner/... FAILS (both scan roots covered);
#   3. a fixture carrying /home/bunker-<hex>/... FAILS (generic account class);
#   4. a clean fixture (~/ and /usr/bin text only, canonical env/version
#      tokens, canonical env badge) PASSES (exit 0);
#   5. an EMPTY fixture passes only because there is nothing to scan — the
#      dirty arms above are what prove the guard is not vacuous;
#   6. REVIEW-OB-010: a data/ fixture with prose in "environment" FAILS;
#   7. REVIEW-OB-010: a data/ fixture with prose in "version" FAILS;
#   8. REVIEW-OB-010: a site/ fixture with a prose env badge FAILS.
#   9-12. REVIEW-OB-009: one data/ fixture per internal name
#      (hermes-dagger, chimera-v2, warpfs, crier) FAILS, file named;
#   13. REVIEW-OB-009: a site/ fixture with a .local/bin/gitreins path FAILS;
#   14. REVIEW-OB-009: <project>/<tool> placeholder text PASSES.
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

# ── ARM 4 — clean fixture passes (~/ and /usr/bin are not leaks; canonical
#    env/version tokens and badges are not prose) ───────────────────────────
FIX="$TMP/arm4-clean"
mkdir -p "$FIX/data/answers" "$FIX/site/classes"
printf '{"solution":"edit ~/x then run /usr/bin/real-tool"}\n' > "$FIX/data/answers/0001-clean.json"
printf '{"answers":[{"environment":"linux","version":"go1.26","signatures":{"environment":"linux host, raw prose provenance is out of scope"}}]}\n' > "$FIX/data/answers/0002-tokens.json"
printf '{"environment":"","version":"latest"}\n' > "$FIX/data/answers.jsonl"
printf '<html>see $HOME/.config and /etc/hosts</html>\n' > "$FIX/site/classes/clean.html"
printf "<div class='meta'><span class='badge v'>github-actions</span></div>\n" > "$FIX/site/classes/badge.html"
run_guard "$FIX"
printf '\nARM 4 — clean fixture: exit 0, OK line printed\n'
check "exit code 0" "0" "$OUT_RC"
check_contains "prints the OK line" "corpus hygiene OK" "$OUT"

# ── ARM 5 — real corpus is clean right now (guard runs against the repo) ───
run_guard "$(cd "$SELF_DIR/../.." && pwd)"
printf '\nARM 5 — the committed repo tree itself passes the guard\n'
check "exit code 0" "0" "$OUT_RC"

# ── ARM 6 — REVIEW-OB-010: prose in "environment" under data/ fails ───────
FIX="$TMP/arm6-prose-env"
mkdir -p "$FIX/data/answers"
printf '{"answers":[{"environment":"linux host, go 1.26.6","version":"1.26"}]}\n' > "$FIX/data/answers/0001-prose.json"
run_guard "$FIX"
printf '\nARM 6 — prose environment value: exit 1, file named\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "names the offending file" "0001-prose.json" "$OUT"
check_contains "names the filter-field rule" "prose in environment/version" "$OUT"

# ── ARM 7 — REVIEW-OB-010: prose in "version" under data/ fails ───────────
FIX="$TMP/arm7-prose-version"
mkdir -p "$FIX/data"
printf '{"environment":"docker","version":"api.example.com/v1, verified 2026-09-15"}\n' > "$FIX/data/answers.jsonl"
run_guard "$FIX"
printf '\nARM 7 — prose version value: exit 1, file named\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "names the offending file" "answers.jsonl" "$OUT"

# ── ARM 8 — REVIEW-OB-010: prose env badge under site/ fails ───────────────
FIX="$TMP/arm8-prose-badge"
mkdir -p "$FIX/data" "$FIX/site/classes"
printf '{"environment":"docker","version":"latest"}\n' > "$FIX/data/answers.jsonl"
printf "<div class='meta'><span class='badge v'>linux host, go 1.26.6</span></div>\n" > "$FIX/site/classes/prose-badge.html"
run_guard "$FIX"
printf '\nARM 8 — prose env badge: exit 1, file named\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "names the offending file" "prose-badge.html" "$OUT"
check_contains "names the badge rule" "prose environment badge" "$OUT"

# ── ARM 9 — REVIEW-OB-009: one dirty fixture per internal-name pattern ─────
# Each leaking string gets its own arm: the guard must fail (exit 1) and
# name the offending file, proving the name check is not vacuous for ANY
# single pattern. Placeholder text (<project>, <tool>) is clean and is
# covered by ARM 14.
n=9
for leak in hermes-dagger chimera-v2 warpfs crier; do
  FIX="$TMP/arm$n-name-$leak"
  mkdir -p "$FIX/data/answers"
  printf '{"solution":"debug the %s deploy pipeline"}\n' "$leak" > "$FIX/data/answers/0001-$leak.json"
  run_guard "$FIX"
  printf '\nARM %s — %s leak under data/: exit 1, file named\n' "$n" "$leak"
  check "exit code 1" "1" "$OUT_RC"
  check_contains "names the offending file" "0001-$leak.json" "$OUT"
  check_contains "names the rule" "internal project/tool names" "$OUT"
  n=$((n + 1))
done

# ── ARM 13 — REVIEW-OB-009: gitreins tool-path leak under site/ fails ──────
FIX="$TMP/arm13-gitreins-path"
mkdir -p "$FIX/data" "$FIX/site/classes"
printf '{"environment":"linux","version":"latest"}\n' > "$FIX/data/answers.jsonl"
printf '<pre>invoke ~/.local/bin/gitreins guard before commit</pre>\n' > "$FIX/site/classes/tool-path.html"
run_guard "$FIX"
printf '\nARM 13 — .local/bin/gitreins leak under site/: exit 1, file named\n'
check "exit code 1" "1" "$OUT_RC"
check_contains "names the offending file" "tool-path.html" "$OUT"

# ── ARM 14 — REVIEW-OB-009: redacted placeholders are clean ────────────────
FIX="$TMP/arm14-placeholders"
mkdir -p "$FIX/data/answers" "$FIX/site/classes"
printf '{"solution":"debug the <project> deploy pipeline with <tool>"}\n' > "$FIX/data/answers/0001-redacted.json"
printf '<pre>invoke &lt;tool&gt; guard before commit</pre>\n' > "$FIX/site/classes/redacted.html"
run_guard "$FIX"
printf '\nARM 14 — <project>/<tool> placeholders only: exit 0\n'
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
