#!/bin/bash
# Regression test for OB-DIST-003 — the deployed ob1-distribute.sh cron
# entrypoint must never drift from the tracked canonical silently.
#
# 2026-10-13..19: the untracked deployed copy at
# ~/.hermes/scripts/ob1-distribute.sh drifted ~6 days behind the tracked
# scripts/ob1-distribute.sh (missing the generate-static-site.py leg; site/
# frozen). Discovered only by a manual md5 compare. This test pins the two
# defenses shut: scripts/check-distribute-deploy.sh (the freshness gate) and
# the in-script drift guard in ob1-distribute.sh (a stale copy refuses to run).
#
# Deterministic: temp dirs only, no network, no contact with the real
# ~/.hermes/scripts/ob1-distribute.sh beyond a read-only sha256 leak check,
# no modification of the real checkout.
#
# Usage: scripts/tests/ob1-distribute-deploy-check-test.sh
#   exit 0 = all cases hold
#   exit 1 = a case regressed
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CHECKER="$REPO_ROOT/scripts/check-distribute-deploy.sh"
CANONICAL_SRC="$REPO_ROOT/scripts/ob1-distribute.sh"
REAL_DEPLOYED="$HOME/.hermes/scripts/ob1-distribute.sh"

ROOT="$(mktemp -d /tmp/ob1-dist-deploy-check.XXXXXX)"
FAILS=0

cleanup() { rm -rf "$ROOT"; }

ok()   { echo "  ✔ $1"; }
bad()  { echo "  ✘ $1"; FAILS=$((FAILS + 1)); }

# Leak-guard baseline: record the real deployed copy's sha256 BEFORE anything
# runs, so the end-of-test check proves this harness never touched it.
REAL_SHA_BEFORE=""
if [ -f "$REAL_DEPLOYED" ]; then
  REAL_SHA_BEFORE="$(sha256sum "$REAL_DEPLOYED" | awk '{print $1}')"
fi

echo "checker under test: $CHECKER"
echo "script under test:  $CANONICAL_SRC"

# ── Case A: in-sync canonical/deployed pair -> checker exit 0 ──
echo "[A] in-sync pair must pass"
A="$ROOT/a"
mkdir -p "$A/repo/scripts" "$A/deploy"
cp "$CANONICAL_SRC" "$A/repo/scripts/ob1-distribute.sh"
cp "$CANONICAL_SRC" "$A/deploy/ob1-distribute.sh"
chmod +x "$A/deploy/ob1-distribute.sh"
OB1_REPO="$A/repo" OB1_DEPLOYED="$A/deploy/ob1-distribute.sh" \
  bash "$CHECKER" > "$ROOT/a.out" 2>&1; rc=$?
if [ "$rc" -eq 0 ] && grep -q "distribute deploy OK" "$ROOT/a.out"; then
  ok "in-sync pair passes (exit 0)"
else
  bad "in-sync pair did not pass (rc=$rc): $(tail -1 "$ROOT/a.out")"
fi

# ── Case B: drifted pair (one byte appended) -> exit 1 naming both paths ──
echo "[B] drifted pair must fail loudly, naming both paths"
B="$ROOT/b"
mkdir -p "$B/repo/scripts" "$B/deploy"
cp "$CANONICAL_SRC" "$B/repo/scripts/ob1-distribute.sh"
cp "$CANONICAL_SRC" "$B/deploy/ob1-distribute.sh"
printf '# one-byte drift\n' >> "$B/deploy/ob1-distribute.sh"
chmod +x "$B/deploy/ob1-distribute.sh"
OB1_REPO="$B/repo" OB1_DEPLOYED="$B/deploy/ob1-distribute.sh" \
  bash "$CHECKER" > "$ROOT/b.out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && grep -qF "$B/repo/scripts/ob1-distribute.sh" "$ROOT/b.out" \
   && grep -qF "$B/deploy/ob1-distribute.sh" "$ROOT/b.out" \
   && grep -q "make deploy-distribute" "$ROOT/b.out"; then
  ok "drift fails (exit 1), both paths + remedy named"
else
  bad "drifted pair was not reported loudly (rc=$rc): $(tail -2 "$ROOT/b.out")"
fi

# ── Case C: missing deployed copy -> exit 1 with its distinct message ──
echo "[C] missing deployed copy must fail with the distinct absent-entrypoint message"
C="$ROOT/c"
mkdir -p "$C/repo/scripts"
cp "$CANONICAL_SRC" "$C/repo/scripts/ob1-distribute.sh"
OB1_REPO="$C/repo" OB1_DEPLOYED="$C/deploy/ob1-distribute.sh" \
  bash "$CHECKER" > "$ROOT/c.out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && grep -q "cron entrypoint is ABSENT" "$ROOT/c.out" \
   && grep -q "make deploy-distribute" "$ROOT/c.out"; then
  ok "missing deployed copy fails with the distinct message (exit 1)"
else
  bad "missing deployed copy not reported distinctly (rc=$rc): $(tail -1 "$ROOT/c.out")"
fi

# ── Case C2: non-executable deployed copy -> its own loud failure ──
echo "[C2] non-executable deployed copy must fail as its own case (cron cannot run it)"
C2="$ROOT/c2"
mkdir -p "$C2/repo/scripts" "$C2/deploy"
cp "$CANONICAL_SRC" "$C2/repo/scripts/ob1-distribute.sh"
cp "$CANONICAL_SRC" "$C2/deploy/ob1-distribute.sh"
chmod -x "$C2/deploy/ob1-distribute.sh"
OB1_REPO="$C2/repo" OB1_DEPLOYED="$C2/deploy/ob1-distribute.sh" \
  bash "$CHECKER" > "$ROOT/c2.out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && grep -q "NOT EXECUTABLE" "$ROOT/c2.out" \
   && grep -q "make deploy-distribute" "$ROOT/c2.out"; then
  ok "non-executable deployed copy fails with its own message (exit 1)"
else
  bad "non-executable deployed copy not rejected (rc=$rc): $(tail -1 "$ROOT/c2.out")"
fi

# ── Case D: the in-script drift guard ──
# Scratch repo whose scripts/ob1-distribute.sh IS the canonical; stub the two
# python legs (they need a live SQLite DB) and the PART 2 publish leg (it ssh's
# to the public catalog host). The stubs write markers OUTSIDE the scratch repo
# so the git-diff block stays quiet and nothing is committed or pushed.
echo "[D] in-script drift guard: stale copy must refuse to run; canonical must pass"
D="$ROOT/d"
mkdir -p "$D/repo/scripts" "$D/repo/data" "$D/repo/site" "$D/deploy"
cp "$CANONICAL_SRC" "$D/repo/scripts/ob1-distribute.sh"
cat > "$D/repo/scripts/export-answers.py" <<STUB
import pathlib
pathlib.Path("$D/export-ran").write_text("ran")
STUB
cat > "$D/repo/scripts/generate-static-site.py" <<STUB
import pathlib
pathlib.Path("$D/site-ran").write_text("ran")
STUB
printf '#!/bin/sh\nexit 0\n' > "$D/repo/scripts/publish-catalog.sh"
chmod +x "$D/repo/scripts/publish-catalog.sh"
echo '{\"v\":1}' > "$D/repo/data/answers.jsonl"
echo 'seed readme' > "$D/repo/README.md"
echo 'seed site'   > "$D/repo/site/index.html"
git -C "$D/repo" init -q -b master .
git -C "$D/repo" config user.email deploy-check@example.com
git -C "$D/repo" config user.name  "OB-DIST-003 deploy-check"
git -C "$D/repo" add -A
git -C "$D/repo" commit -q -m "seed: scratch repo for the drift guard cases"

# D1: a DIFFERENT copy (canonical + appended byte) run as the "deployed" one
# must be refused by the guard BEFORE any export/site leg runs.
cp "$CANONICAL_SRC" "$D/deploy/ob1-distribute.sh"
printf '# stale deployed copy\n' >> "$D/deploy/ob1-distribute.sh"
chmod +x "$D/deploy/ob1-distribute.sh"
OB1_REPO="$D/repo" bash "$D/deploy/ob1-distribute.sh" > "$ROOT/d1.out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && grep -q "drift guard" "$ROOT/d1.out" \
   && grep -q "make deploy-distribute" "$ROOT/d1.out" \
   && [ ! -e "$D/export-ran" ] && [ ! -e "$D/site-ran" ]; then
  ok "stale copy refused (exit 1, loud, remedy named) before any distribution leg ran"
else
  bad "stale copy was not refused by the guard (rc=$rc): $(tail -2 "$ROOT/d1.out")"
fi

# D2: the canonical path itself must pass the guard and run the legs (stubs
# prove the run got past the guard; PART 2 publish is stubbed to exit 0).
OB1_REPO="$D/repo" bash "$D/repo/scripts/ob1-distribute.sh" > "$ROOT/d2.out" 2>&1; rc=$?
if [ "$rc" -eq 0 ] && [ -e "$D/export-ran" ] && [ -e "$D/site-ran" ]; then
  ok "canonical copy passes the guard and runs the (stubbed) legs"
else
  bad "canonical copy did not run clean (rc=$rc): $(tail -2 "$ROOT/d2.out")"
fi

# ── Leak guard: the real deployed copy must be byte-unchanged ──
echo "[leak] real deployed copy must be untouched"
if [ -n "$REAL_SHA_BEFORE" ]; then
  REAL_SHA_AFTER="$(sha256sum "$REAL_DEPLOYED" | awk '{print $1}')"
  if [ "$REAL_SHA_AFTER" = "$REAL_SHA_BEFORE" ]; then
    ok "real deployed copy unchanged (sha256 $REAL_SHA_AFTER)"
  else
    bad "LEAK: real deployed copy changed ($REAL_SHA_BEFORE -> $REAL_SHA_AFTER)"
  fi
else
  echo "  (real deployed copy absent — leak guard has nothing to compare)"
fi

REAL_HEAD="$(git -C "$REPO_ROOT" rev-parse HEAD)"
cleanup
echo
if [ "$FAILS" -eq 0 ]; then
  echo "PASS — all OB-DIST-003 deploy-check cases hold (real checkout at ${REAL_HEAD:0:7} untouched)"
  exit 0
fi
echo "FAIL — $FAILS case(s) regressed"
exit 1
