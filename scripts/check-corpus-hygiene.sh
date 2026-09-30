#!/usr/bin/env bash
# scripts/check-corpus-hygiene.sh — REVIEW-OB-006 + REVIEW-OB-010 guard.
#
# The answer corpus (data/) and the generated static site (site/) are PUBLIC
# (github.com/totalwindupflightsystems/off-by-one + the ob1.it.com catalog).
#
# Two leak classes are rejected:
#
# 1. REVIEW-OB-006 — operator host paths: any /home/<user> prefix (kara,
#    bunker, bunker-*, runner, user — generically [A-Za-z0-9_-]+ account
#    names). The export pipeline rewrites these to `~` at write time.
#
# 2. REVIEW-OB-010 — prose in the environment/version discovery-filter
#    fields: these are exact-match tuple filters (empty = wildcard), so a
#    value carrying whitespace, ';' or '~' is unreachable via tuple-scoped
#    discovery and leaks host context into the catalog. Checked as the
#    "environment"/"version" JSON fields under data/ plus the rendered
#    env badge (class='badge v') under site/.
#
# 3. REVIEW-OB-009 — internal project/tool names: hermes-dagger,
#    chimera-v2, warpfs, crier and any .local/bin/gitreins install path
#    are lab-internal identifiers. The export pipeline rewrites them to
#    <project>/<tool> placeholders at write time.
#
# Exit 1 (listing the offending files) when a match is found under data/ or
# site/; exit 0 when clean. Ops files legitimately carry deploy paths and are
# deliberately out of scope — only data/ and site/ are scanned.
#
# CORPUS_HYGIENE_ROOT overrides the scan root so the self-test can point the
# guard at a temp fixture tree (scripts/tests/check-corpus-hygiene-selftest.sh).
#
# Usage: bash scripts/check-corpus-hygiene.sh   (or: make check-corpus-hygiene)

set -uo pipefail

ROOT="${CORPUS_HYGIENE_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}"
GUARD_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HOST_PATH_PATTERN='/home/[A-Za-z0-9_-]+'
# The rendered env badge on the static site carries the same value.
BADGE_PATTERN="class='badge v'>[^<]*[ ;~]"
# REVIEW-OB-009: internal project/tool names must never be published.
NAME_PATTERN='hermes-dagger|chimera-v2|warpfs|crier|\.local/bin/gitreins'

rc=0

host_offenders="$(grep -rEl "$HOST_PATH_PATTERN" "$ROOT/data" "$ROOT/site" 2>/dev/null)"
if [ -n "$host_offenders" ]; then
  printf 'corpus hygiene VIOLATION: operator host paths (/home/<user>) under data/ or site/:\n' >&2
  printf '%s\n' "$host_offenders" | sed 's/^/  /' >&2
  rc=1
fi

name_offenders="$(grep -rEl "$NAME_PATTERN" "$ROOT/data" "$ROOT/site" 2>/dev/null)"
if [ -n "$name_offenders" ]; then
  printf 'corpus hygiene VIOLATION: internal project/tool names (hermes-dagger, chimera-v2, warpfs, crier, .local/bin/gitreins) under data/ or site/:\n' >&2
  printf '%s\n' "$name_offenders" | sed 's/^/  /' >&2
  rc=1
fi

# REVIEW-OB-010: a JSON env/version filter-field value containing
# whitespace, ';' or '~' is prose, not a canonical token. Field-precise
# check (answer records only — nested signatures provenance is out of
# scope) lives in the python helper so it cannot false-positive on
# embedded metadata.
if ! python3 "$GUARD_DIR/check-corpus-tokens.py" "$ROOT"; then
  rc=1
fi

badge_offenders="$(grep -rEl "$BADGE_PATTERN" "$ROOT/site" 2>/dev/null)"
if [ -n "$badge_offenders" ]; then
  printf 'corpus hygiene VIOLATION: prose environment badge (whitespace, ; or ~) under site/:\n' >&2
  printf '%s\n' "$badge_offenders" | sed 's/^/  /' >&2
  rc=1
fi

if [ "$rc" -ne 0 ]; then
  printf 'remedy: fix the source, regenerate via scripts/export-answers.py +\n' >&2
  printf '        scripts/generate-static-site.py — never hand-edit data/answers/*.json\n' >&2
  exit 1
fi

printf 'corpus hygiene OK: no /home/<user> paths, no internal names and no prose env/version tokens under %s/{data,site}\n' "$ROOT"
exit 0
