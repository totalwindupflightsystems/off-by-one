#!/usr/bin/env bash
# scripts/check-corpus-hygiene.sh — REVIEW-OB-006 guard.
#
# The answer corpus (data/) and the generated static site (site/) are PUBLIC
# (github.com/totalwindupflightsystems/off-by-one + the ob1.it.com catalog).
# Neither may carry an operator host path: any /home/<user> prefix (kara,
# bunker, bunker-*, runner, user — generically [A-Za-z0-9_-]+ account names)
# is a leak. The export pipeline rewrites these to `~` at write time; this
# guard is the backstop that rejects any leak that still lands.
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
PATTERN='/home/[A-Za-z0-9_-]+'

offenders="$(grep -rEl "$PATTERN" "$ROOT/data" "$ROOT/site" 2>/dev/null)"

if [ -n "$offenders" ]; then
  printf 'corpus hygiene VIOLATION: operator host paths (/home/<user>) under data/ or site/:\n' >&2
  printf '%s\n' "$offenders" | sed 's/^/  /' >&2
  printf 'remedy: fix the source, regenerate via scripts/export-answers.py +\n' >&2
  printf '        scripts/generate-static-site.py — never hand-edit data/answers/*.json\n' >&2
  exit 1
fi

printf 'corpus hygiene OK: no /home/<user> paths under %s/{data,site}\n' "$ROOT"
exit 0
