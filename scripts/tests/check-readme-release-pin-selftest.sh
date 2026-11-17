#!/usr/bin/env bash
# scripts/tests/check-readme-release-pin-selftest.sh — non-vacuity proof for
# scripts/check-readme-release-pin.sh (README-11).
#
# A guard that never fires is worthless, and a guard that always fires is
# worse. This test drives the checker against TEMP fixture repos + READMEs via
# REPO_DIR/README_PATH and proves every outcome the checker claims:
#
#   1. RED   — pin v0.0.1 vs tag v9.9.9: exit 1, BOTH values named.
#   2. GREEN — pin == latest tag: exit 0, PASS names the tag, and unrelated
#              `V=` literals outside the install block are NOT picked up.
#   3. SKIP  — repo with no tags: exit 0 AND a literal `SKIP` line printed
#              (a silent skip would be indistinguishable from a pass).
#   4. MISSING-PIN — install block without a `V=` literal: non-zero.
#   5. VERSION SORT — tags v0.9.0 + v0.10.0: pin v0.10.0 PASSES (lexicographic
#              ordering would pick v0.9.0 and fail a correct README).
#   6. NO SECTION — README without the "Install from a release" heading:
#              non-zero (never a pass).
#   7. NOT A REPO — git itself broken: non-zero (a git error is never a pass).
#
# Hermetic: mktemp -d fixtures only, removed on exit. It never reads this
# repo's tags or its real README (the real-tree PASS is proven separately by
# `make check-readme-release-pin`), and it makes no network calls.
#
# Usage: bash scripts/tests/check-readme-release-pin-selftest.sh
#        (or: make check-readme-release-pin-selftest)

set -uo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GUARD="$(cd "$SELF_DIR/.." && pwd)/check-readme-release-pin.sh"

if [ ! -f "$GUARD" ]; then
  printf 'selftest: checker not found at %s\n' "$GUARD" >&2
  exit 1
fi

TMP="$(mktemp -d "${TMPDIR:-/tmp}/ob1-readme-release-pin-selftest-XXXXXX")"
# shellcheck disable=SC2329  # invoked by the EXIT trap below
cleanup() { rm -rf "$TMP"; return 0; }
trap cleanup EXIT

arms_pass=0
arms_fail=0
arms_total=0

# ── fixture helpers ────────────────────────────────────────────────────────

# fixture_git <dir> <args...> — git with the ambient host config neutralised so
# a machine-global core.hooksPath / commit gate cannot poison a scratch repo.
fixture_git() {
  local dir="$1"
  shift
  git -C "$dir" \
    -c core.hooksPath=/dev/null \
    -c commit.gpgsign=false \
    -c user.name=selftest \
    -c user.email=selftest@example.invalid \
    "$@"
}

init_repo() { # <dir>
  mkdir -p "$1"
  git -c init.defaultBranch=main init -q "$1"
  fixture_git "$1" commit -q --allow-empty -m "fixture root"
}

# fixture_readme <path> <mode> [tag]
#   pin       — install section + fenced block containing V=<tag>
#   nopin     — install section + fenced block WITHOUT a V= literal
#   nosection — no "Install from a release" heading at all
# pin/nopin fixtures also carry decoy `V=` literals BEFORE the section: the
# parser must be scoped to the install block, and arm 2 proves it.
fixture_readme() {
  local path="$1" mode="$2" tag="${3:-}"
  # Fixture text is shell LITERAL by design: $V / $base inside the fenced blocks
  # must reach the checker as `$V`, exactly as in the real README snippet.
  # shellcheck disable=SC2016
  {
    printf '# fixture readme\n\n'
    printf 'Prose decoy: V=v6.6.6 must never be read as the release pin.\n\n'
    printf '```bash\nV=v7.7.7\n```\n\n'
    case "$mode" in
      pin)
        printf '### Install from a release (no toolchain needed)\n\n'
        printf '```bash\nV=%s\nbase=https://example.invalid/download/$V\ncurl -fsSLO $base/off-by-one-$V\n```\n\n' "$tag"
        ;;
      nopin)
        printf '### Install from a release (no toolchain needed)\n\n'
        printf '```bash\nbase=https://example.invalid/download/off-by-one\ncurl -fsSLO $base/off-by-one\n```\n\n'
        ;;
      nosection)
        printf '## Quick Start\n\nNothing to see here.\n\n'
        ;;
      *)
        printf 'fixture_readme: unknown mode %s\n' "$mode" >&2
        return 1
        ;;
    esac
    printf '## Quick Start\n\nBuild from source instead.\n'
  } > "$path"
}

run_guard() { # <repo-dir> <readme> -> $OUT, $OUT_RC
  OUT="$TMP/last-out.txt"
  REPO_DIR="$1" README_PATH="$2" bash "$GUARD" > "$OUT" 2>&1
  OUT_RC=$?
}

# ── arm accounting ─────────────────────────────────────────────────────────
ARM_FAILS=0
ARM_DESC=""

arm_start() {
  ARM_FAILS=0
  ARM_DESC="$1"
  arms_total=$((arms_total + 1))
}

assert_eq() { # <desc> <expected> <actual>
  if [ "$2" = "$3" ]; then
    printf '       ok   %s\n' "$1"
  else
    printf '       FAIL %s\n            expected: %s\n            actual:   %s\n' "$1" "$2" "$3"
    ARM_FAILS=$((ARM_FAILS + 1))
  fi
}

assert_contains() { # <desc> <needle>
  if grep -qF -- "$2" "$OUT" 2>/dev/null; then
    printf '       ok   %s\n' "$1"
  else
    printf '       FAIL %s\n            missing: %s\n' "$1" "$2"
    ARM_FAILS=$((ARM_FAILS + 1))
  fi
}

assert_absent() { # <desc> <needle>
  if grep -qF -- "$2" "$OUT" 2>/dev/null; then
    printf '       FAIL %s\n            unexpected: %s\n' "$1" "$2"
    ARM_FAILS=$((ARM_FAILS + 1))
  else
    printf '       ok   %s\n' "$1"
  fi
}

arm_end() {
  if [ "$ARM_FAILS" -eq 0 ]; then
    arms_pass=$((arms_pass + 1))
    printf 'ARM %s: PASS — %s\n' "$arms_total" "$ARM_DESC"
  else
    arms_fail=$((arms_fail + 1))
    printf 'ARM %s: FAIL — %s (%s failed assertion(s))\n' "$arms_total" "$ARM_DESC" "$ARM_FAILS"
    printf '       ── checker output ──\n'
    sed 's/^/       | /' "$OUT" 2>/dev/null
  fi
  printf '\n'
}

printf 'check-readme-release-pin self-test (temp fixtures via REPO_DIR/README_PATH)\n'
printf 'guard=%s\n\n' "$GUARD"

# ── ARM 1 — RED: stale pin fails and names BOTH values ─────────────────────
FIX="$TMP/arm1-stale-pin"
init_repo "$FIX"
fixture_git "$FIX" tag v9.9.9
fixture_readme "$FIX/README.md" pin v0.0.1
run_guard "$FIX" "$FIX/README.md"
arm_start "stale pin v0.0.1 vs latest tag v9.9.9 → exit 1 naming both values"
assert_eq "exit code is 1" "1" "$OUT_RC"
assert_contains "names the README pin" "v0.0.1"
assert_contains "names the latest tag" "v9.9.9"
assert_contains "states the remedy" "remedy:"
assert_absent "does not print PASS" "PASS:"
arm_end

# ── ARM 2 — GREEN: pin == latest tag, unrelated V= literals ignored ─────────
FIX="$TMP/arm2-matching-pin"
init_repo "$FIX"
fixture_git "$FIX" tag v2.5.3
fixture_readme "$FIX/README.md" pin v2.5.3
run_guard "$FIX" "$FIX/README.md"
arm_start "pin v2.5.3 == latest tag v2.5.3 → exit 0 (decoy V=v6.6.6 / V=v7.7.7 ignored)"
assert_eq "exit code is 0" "0" "$OUT_RC"
assert_contains "prints PASS" "PASS:"
assert_contains "names the matching tag" "v2.5.3"
assert_absent "never reports the decoy block literal as the pin" "V=v7.7.7"
arm_end

# ── ARM 3 — SKIP: no tags → exit 0 with a LOUD SKIP line ───────────────────
FIX="$TMP/arm3-tagless"
init_repo "$FIX"
fixture_readme "$FIX/README.md" pin v0.0.1
run_guard "$FIX" "$FIX/README.md"
arm_start "no tags at all → exit 0 with a literal SKIP line (never a silent pass)"
assert_eq "exit code is 0" "0" "$OUT_RC"
assert_contains "prints a SKIP line" "SKIP:"
assert_contains "names the reason" "no v<semver> tags"
assert_absent "does not claim PASS" "PASS:"
arm_end

# ── ARM 4 — MISSING-PIN: install block without a V= literal → non-zero ─────
FIX="$TMP/arm4-missing-pin"
init_repo "$FIX"
fixture_git "$FIX" tag v1.0.0
fixture_readme "$FIX/README.md" nopin
run_guard "$FIX" "$FIX/README.md"
arm_start "install block without a V= literal → non-zero (cannot verify what is absent)"
assert_eq "exit code is 2" "2" "$OUT_RC"
assert_contains "names the missing literal" "no 'V=v<semver>' literal"
assert_absent "does not claim PASS" "PASS:"
arm_end

# ── ARM 5 — VERSION SORT: v0.10.0 outranks v0.9.0 (not lexicographic) ──────
FIX="$TMP/arm5-version-sort"
init_repo "$FIX"
fixture_git "$FIX" tag v0.9.0
fixture_git "$FIX" tag v0.10.0
fixture_readme "$FIX/README.md" pin v0.10.0
run_guard "$FIX" "$FIX/README.md"
arm_start "tags v0.9.0 + v0.10.0, pin v0.10.0 → exit 0 (version sort, not lexicographic)"
assert_eq "exit code is 0" "0" "$OUT_RC"
assert_contains "resolves v0.10.0 as the latest tag" "v0.10.0"
arm_end

# ── ARM 6 — NO SECTION: heading gone → non-zero ────────────────────────────
FIX="$TMP/arm6-no-section"
init_repo "$FIX"
fixture_git "$FIX" tag v1.0.0
fixture_readme "$FIX/README.md" nosection
run_guard "$FIX" "$FIX/README.md"
arm_start "no 'Install from a release' heading → non-zero (the guarded surface vanished)"
assert_eq "exit code is 2" "2" "$OUT_RC"
assert_contains "names the missing heading" "no 'Install from a release' heading"
arm_end

# ── ARM 7 — NOT A REPO: git broken → non-zero, never a pass ────────────────
FIX="$TMP/arm7-not-a-repo"
mkdir -p "$FIX"
fixture_readme "$FIX/README.md" pin v1.0.0
run_guard "$FIX" "$FIX/README.md"
arm_start "REPO_DIR that is not a git repository → non-zero (a git error is never a pass)"
assert_eq "exit code is 2" "2" "$OUT_RC"
assert_contains "names the git failure" "not a git repository"
assert_absent "does not claim PASS" "PASS:"
arm_end

# ── summary ────────────────────────────────────────────────────────────────
printf '──────────────────────────────────────────────\n'
printf 'check-readme-release-pin self-test: %s/%s arms passed' "$arms_pass" "$arms_total"
if [ "$arms_fail" -eq 0 ]; then
  printf ' — ALL GREEN\n'
  exit 0
fi
printf ' — %s arm(s) FAILED\n' "$arms_fail"
exit 1
