#!/usr/bin/env bash
# scripts/check-readme-release-pin.sh — README-11 guard: the README's
# "Install from a release" first-run path must point at the LATEST release tag.
#
# The block below that heading hard-codes the release tag in a shell literal:
#
#     V=v0.3.0
#     base=https://github.com/.../releases/download/$V
#
# That literal rotted twice (v0.1.1 -> v0.2.0 -> v0.3.0), and each time the
# docs shipped a first run pointing at a tag that no longer matched the latest
# release — the worst kind of docs bug, because it only fails for a user who
# has no toolchain and therefore no fallback. This gate makes the next rot a
# loud failure instead of a manual bump someone has to remember.
#
# Source of truth: the repo's own tags (highest v<semver> by version sort).
# LOCAL GIT ONLY — no network calls, ever: `git tag` reads .git, nothing more.
#
# Outcomes:
#   0 PASS  — the README pin equals the latest tag (named).
#   0 SKIP  — no tags in this checkout (shallow clone / tagless export). The
#             skip is printed LOUDLY with its reason and is never silent: a
#             quiet skip is indistinguishable from a pass, which is the whole
#             failure mode this guard exists to remove.
#   1 VIOLATION — pin != latest tag: both values are printed to stderr with the
#             one-line remedy.
#   2 ERROR — README missing, section/fence/literal missing, or git itself is
#             broken. Never exit 0 on a git error (an unrecognised error is not
#             evidence of agreement).
#
# Read-only: no writes, no network, safe to run anywhere.
#
# Env overrides (used by scripts/tests/check-readme-release-pin-selftest.sh so
# the fixtures are temp dirs and never touch this repo):
#   REPO_DIR     repo to resolve tags from (default: this script's parent dir)
#   README_PATH  README to read the pin from (default: $REPO_DIR/README.md)
#
# Usage: bash scripts/check-readme-release-pin.sh
#        (or: make check-readme-release-pin)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="${REPO_DIR:-$(cd "$SCRIPT_DIR/.." && pwd)}"
README_PATH="${README_PATH:-$REPO_DIR/README.md}"

# A release tag is a v-prefixed semver. Sorted with version sort so v0.10.0
# outranks v0.9.9 (the lexicographic trap that would make this guard point at
# an older release than the newest one).
RELEASE_TAG_RE='^v[0-9]+\.[0-9]+\.[0-9]+([-+.][0-9A-Za-z.+-]+)?$'

err() { printf 'check-readme-release-pin: %s\n' "$*" >&2; }

if [ ! -f "$README_PATH" ]; then
  err "README not found: $README_PATH"
  err "  nothing to verify — refusing to report a pass without the file"
  exit 2
fi

if ! git -C "$REPO_DIR" rev-parse --git-dir >/dev/null 2>&1; then
  err "not a git repository: $REPO_DIR"
  err "  cannot resolve the latest release tag; a git failure is never a pass"
  exit 2
fi

# ── extract the pin ────────────────────────────────────────────────────────
# Scoped to the FIRST fenced block under the "Install from a release" heading,
# so an unrelated `V=` elsewhere in the README (or in any other code block)
# can never satisfy — or falsely fail — the comparison.
pin_probe="$(
  awk '
    BEGIN { section = 0; fence = 0; saw_block = 0; pin = ""; pin_nr = 0 }
    /^#{1,6}[[:space:]]+[Ii]nstall from a release/ { section = 1; next }
    section && /^```/ {
      if (!saw_block) { saw_block = 1; fence = 1; next }
      if (fence) { fence = 0; exit }
    }
    section && fence && pin == "" && /^V=v[0-9]+[.][0-9]+[.][0-9]+/ {
      pin = $0; pin_nr = NR; exit
    }
    END {
      if (!section) { print "SECTION_MISSING"; exit }
      if (!saw_block) { print "BLOCK_MISSING"; exit }
      if (pin == "") { print "PIN_MISSING"; exit }
      print "PIN " pin_nr " " pin
    }
  ' "$README_PATH"
)"

case "$pin_probe" in
  "PIN "*)
    pin_nr="${pin_probe#PIN }"          # "<line> V=v0.3.0"
    pin_nr="${pin_nr%% *}"
    pin_line="${pin_probe##* }"         # "V=v0.3.0"
    ;;
  SECTION_MISSING)
    err "no 'Install from a release' heading in $README_PATH"
    err "  the first-run path this gate guards is gone — restore the section (or re-point README-11's guard)"
    exit 2
    ;;
  BLOCK_MISSING)
    err "no fenced code block under 'Install from a release' in $README_PATH"
    err "  the release install snippet is gone — restore it"
    exit 2
    ;;
  PIN_MISSING)
    err "no 'V=v<semver>' literal in the 'Install from a release' block of $README_PATH"
    err "  the snippet must pin the release tag in a V= literal (V=vX.Y.Z)"
    exit 2
    ;;
  *)
    err "unexpected parser output: $pin_probe"
    exit 2
    ;;
esac

readme_tag="${pin_line#V=}"
if ! printf '%s\n' "$readme_tag" | grep -Eq "$RELEASE_TAG_RE"; then
  err "README pin is not a v<semver> release tag: $pin_line ($README_PATH:$pin_nr)"
  exit 2
fi

# ── latest tag (local git only) ────────────────────────────────────────────
if ! tag_dump="$(git -C "$REPO_DIR" tag --sort=-v:refname 2>&1)"; then
  err "git tag failed in $REPO_DIR: $tag_dump"
  err "  cannot resolve the latest release tag; a git failure is never a pass"
  exit 2
fi

latest_tag="$(printf '%s\n' "$tag_dump" | grep -E "$RELEASE_TAG_RE" | head -n 1 || true)"

if [ -z "$latest_tag" ]; then
  shallow="$(git -C "$REPO_DIR" rev-parse --is-shallow-repository 2>/dev/null || echo unknown)"
  if [ "$shallow" = "true" ]; then
    reason="shallow clone — tags were not fetched (fetch with depth 0 or run 'git fetch --tags')"
  else
    reason="no v<semver> tags in this checkout ($REPO_DIR)"
  fi
  printf 'SKIP: check-readme-release-pin — %s; README pin %s not compared against any tag (LOUD skip, NOT a pass)\n' \
    "$reason" "$readme_tag"
  exit 0
fi

# ── compare ────────────────────────────────────────────────────────────────
if [ "$readme_tag" = "$latest_tag" ]; then
  printf 'PASS: README release pin %s matches the latest tag %s (%s:%s)\n' \
    "$readme_tag" "$latest_tag" "$README_PATH" "$pin_nr"
  exit 0
fi

printf 'check-readme-release-pin: VIOLATION — the README first-run path is STALE\n' >&2
printf '  README pin: %s   (%s:%s)\n' "$readme_tag" "$README_PATH" "$pin_nr" >&2
printf '  latest tag: %s   (the latest release this repo publishes)\n' "$latest_tag" >&2
printf '  A user with no toolchain cannot fall back to a source build, so this\n' >&2
printf '  snippet pointing at a superseded tag is a dead first run.\n' >&2
printf '  remedy: sed -i "s/^V=.*/V=%s/" README.md   (or cut the release first: make release TAG=%s)\n' \
  "$latest_tag" "$latest_tag" >&2
exit 1
