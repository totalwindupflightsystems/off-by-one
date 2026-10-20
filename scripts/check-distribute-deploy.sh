#!/usr/bin/env bash
# scripts/check-distribute-deploy.sh — OB-DIST-003 freshness gate for the
# ob1-distribute cron entrypoint.
#
# The distribution cron (4x/day) runs the UNTRACKED deployed copy
# ~/.hermes/scripts/ob1-distribute.sh; the tracked canonical is
# scripts/ob1-distribute.sh in this repo. Nothing enforced that the two match,
# and the deployed copy silently drifted ~6 days behind the canonical (missing
# the generate-static-site.py leg — discovered only by a manual md5 compare).
# This checker makes that drift a loud failure instead of a silent freeze.
#
# Read-only: no network, no writes, no side effects. Exit 1 when the deployed
# copy is missing, non-executable or byte-drifted; exit 0 when the pair match.
#
# OB1_REPO / OB1_DEPLOYED override the two paths so the self-test can point the
# checker at temp fixture trees
# (scripts/tests/ob1-distribute-deploy-check-test.sh).
#
# Usage: bash scripts/check-distribute-deploy.sh   (or: make check-distribute-deploy)

set -uo pipefail

CANONICAL="${OB1_REPO:-$HOME/off-by-one}/scripts/ob1-distribute.sh"
DEPLOYED="${OB1_DEPLOYED:-$HOME/.hermes/scripts/ob1-distribute.sh}"

if [ ! -f "$CANONICAL" ]; then
  printf 'distribute deploy VIOLATION: tracked canonical is missing: %s\n' "$CANONICAL" >&2
  printf '  nothing to verify the deployed copy against — restore the checkout\n' >&2
  exit 1
fi

if [ ! -f "$DEPLOYED" ]; then
  printf 'distribute deploy VIOLATION: cron entrypoint is ABSENT: %s\n' "$DEPLOYED" >&2
  printf '  the distribution cron runs this untracked copy — without it nothing is distributed\n' >&2
  printf '  remedy: make deploy-distribute\n' >&2
  exit 1
fi

if [ ! -x "$DEPLOYED" ]; then
  printf 'distribute deploy VIOLATION: deployed copy is NOT EXECUTABLE (the cron cannot run it): %s\n' "$DEPLOYED" >&2
  printf '  remedy: make deploy-distribute\n' >&2
  exit 1
fi

canonical_sha="$(sha256sum "$CANONICAL" | awk '{print $1}')"
deployed_sha="$(sha256sum "$DEPLOYED" | awk '{print $1}')"

if [ "$canonical_sha" != "$deployed_sha" ]; then
  printf 'distribute deploy VIOLATION: deployed copy has DRIFTED from the tracked canonical:\n' >&2
  printf '  canonical: %s\n    sha256 %s\n' "$CANONICAL" "$canonical_sha" >&2
  printf '  deployed:  %s\n    sha256 %s\n' "$DEPLOYED" "$deployed_sha" >&2
  printf '  remedy: make deploy-distribute\n' >&2
  exit 1
fi

printf 'distribute deploy OK: %s matches %s (sha256 %s)\n' "$DEPLOYED" "$CANONICAL" "$deployed_sha"
exit 0
