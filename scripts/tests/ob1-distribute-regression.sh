#!/bin/bash
# Regression test for OB-DIST-001 — ob1-distribute.sh PART 1 must never report a
# corpus sync it did not make.
#
# 2026-10-19 17:00 local: the old block ran `git commit ... || true`. The commit
# was rejected, the script still printed "Answer corpus synced to GitHub" over the
# STALE head, the push answered "Everything up-to-date" and the cron run reported
# success — the corpus never reached GitHub. This test pins both halves of that
# failure shut.
#
# The PART 1 add/commit/push block is sliced VERBATIM out of the script under
# test (no re-typing, so it cannot drift from production). The two export/site
# legs before it are stubbed (upstream of the block, and they need a live SQLite
# DB) and the PART 2 publish leg is excluded — it ssh's to the public catalog
# host and is not under test.
#
# Usage: scripts/tests/ob1-distribute-regression.sh [path-to-script]
#   exit 0 = new script fails loud / lands correctly, old script demonstrably
#            silent (i.e. the fix is still in place)
#   exit 1 = a case regressed
set -uo pipefail

SCRIPT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/ob1-distribute.sh}"
REPO_ROOT="$(cd "$(dirname "$SCRIPT")/.." && pwd)"
ROOT="$(mktemp -d /tmp/ob1-dist-regress.XXXXXX)"
W="$ROOT/work"
FAILS=0

cleanup() { rm -rf "$ROOT"; }

ok()   { echo "  ✔ $1"; }
bad()  { echo "  ✘ $1"; FAILS=$((FAILS + 1)); }

# The script cd's to ${OB1_REPO:-$HOME/off-by-one}. Pin it to the scratch repo so
# a broken run can never touch the real checkout (learned the hard way: the first
# version of this harness leaked into the live repo and committed there).
export OB1_REPO="$W"

git init -q --bare "$ROOT/origin.git"
git init -q --bare "$ROOT/origin2.git"
mkdir -p "$ROOT/hooks" "$W"
cd "$W" || { echo "cannot enter scratch repo"; exit 1; }
git init -q -b master .
git config user.email regression@example.com
git config user.name  "OB-DIST-001 regression"
git config core.hooksPath "$ROOT/hooks"
mkdir -p data scripts site
echo '{"v":1}' > data/answers.jsonl
echo 'seed readme' > README.md
echo 'seed site'   > site/index.html
git add -A
git commit -q -m "seed: prior corpus state (the stale head in the incident)"
git remote add origin "$ROOT/origin.git"
git push -q origin master
SEED="$(git rev-parse HEAD)"

stubs() {
  mkdir -p scripts site
  cat > scripts/export-answers.py <<'STUB'
import json, pathlib
p = pathlib.Path('data/answers.jsonl')
d = json.loads(p.read_text())
d['v'] = d.get('v', 0) + 1
p.write_text(json.dumps(d) + "\n")
STUB
  cat > scripts/generate-static-site.py <<'STUB'
import pathlib
pathlib.Path('site').mkdir(exist_ok=True)
pathlib.Path('site/index.html').write_text('regen\n')
STUB
}
stubs

# slice `set -euo pipefail` .. (exclusive) the PART 2 marker: the block under test
slice() { awk '/^set -euo pipefail$/{p=1} p && /PART 2/{exit} p{print}' "$1" > "$2"; }
gameover_hook() {
  printf '#!/bin/sh\necho "simulated guard rejection: hook exits 1" >&2\nexit 1\n' > "$ROOT/hooks/pre-commit"
  chmod +x "$ROOT/hooks/pre-commit"
}
clean_hook() { rm -f "$ROOT/hooks/pre-commit"; }

# the pre-fix text, recovered from git history, so the RED case always tests the
# real old block rather than a paraphrase
OLD="$ROOT/old.sh"
if git -C "$REPO_ROOT" log --oneline -S'files changed' -- scripts/ob1-distribute.sh >/dev/null 2>&1; then
  OLD_REV="$(git -C "$REPO_ROOT" log --format=%H -S'|| true' -- scripts/ob1-distribute.sh | head -1)"
fi
if [ -n "${OLD_REV:-}" ] && git -C "$REPO_ROOT" show "$OLD_REV:scripts/ob1-distribute.sh" > "$OLD" 2>/dev/null; then
  slice "$OLD" "$ROOT/old-part1.sh"
else
  echo "note: pre-fix revision not found in history — RED case skipped"
fi
slice "$SCRIPT" "$ROOT/new-part1.sh"

echo "script under test: $SCRIPT"

echo "[1] pre-fix block: rejected commit must be caught as a SILENT FALSE GREEN"
if [ -s "$ROOT/old-part1.sh" ]; then
  git reset -q --hard "$SEED"; stubs; gameover_hook
  bash "$ROOT/old-part1.sh" > "$ROOT/c1.out" 2>&1; rc=$?
  if [ "$rc" -eq 0 ] && grep -q "Answer corpus synced to GitHub" "$ROOT/c1.out" \
     && grep -q "seed: prior corpus state" "$ROOT/c1.out" \
     && [ "$(git rev-parse HEAD)" = "$SEED" ] && [ -n "$(git status --porcelain)" ]; then
    ok "old block exits 0 claiming success over the stale commit (RED shape reproduced)"
  else
    bad "old block no longer reproduces the silent no-op (rc=$rc)"
  fi
fi

echo "[2] rejected commit must fail loudly and never claim a sync"
git reset -q --hard "$SEED"; stubs; gameover_hook
bash "$ROOT/new-part1.sh" > "$ROOT/c2.out" 2>&1; rc=$?
if [ "$rc" -ne 0 ] && grep -q "corpus commit FAILED" "$ROOT/c2.out" \
   && ! grep -q "Answer corpus synced" "$ROOT/c2.out" && [ "$(git rev-parse HEAD)" = "$SEED" ]; then
  ok "non-zero exit, failure named, no success claim, HEAD untouched"
else
  bad "rejected commit was not reported loudly (rc=$rc)"
fi

echo "[3] healthy run must commit and land on the remote"
git reset -q --hard "$SEED"; stubs; clean_hook
bash "$ROOT/new-part1.sh" > "$ROOT/c3.out" 2>&1; rc=$?
NEW="$(git rev-parse HEAD)"; REM="$(git ls-remote origin refs/heads/master | awk '{print $1}')"
if [ "$rc" -eq 0 ] && [ "$NEW" != "$SEED" ] && [ "$REM" = "$NEW" ] \
   && grep -q "data: sync answer corpus" "$ROOT/c3.out"; then
  ok "head advanced and the remote carries ${NEW:0:7}"
else
  bad "healthy run did not land (rc=$rc head=$NEW remote=$REM)"
fi

echo "[4] a push that returns 0 but lands elsewhere must not be taken as distribution"
git reset -q --hard "$SEED"; stubs; clean_hook
git -C "$ROOT/origin.git" update-ref refs/heads/master "$SEED"
git remote set-url --push origin "$ROOT/origin2.git"
bash "$ROOT/new-part1.sh" > "$ROOT/c4.out" 2>&1; rc=$?
git remote set-url --push origin "$ROOT/origin.git"
if [ "$rc" -ne 0 ] && ! grep -q "Answer corpus synced" "$ROOT/c4.out"; then
  ok "no-op push caught: $(grep -o 'push did not land.*' "$ROOT/c4.out" | head -1 | cut -c1-60)"
else
  bad "push return code alone was accepted as distribution (rc=$rc)"
fi

REAL_HEAD="$(git -C "$REPO_ROOT" rev-parse HEAD)"
cleanup
echo
if [ "$FAILS" -eq 0 ]; then
  echo "PASS — all OB-DIST-001 cases hold (real checkout at ${REAL_HEAD:0:7} untouched)"
  exit 0
fi
echo "FAIL — $FAILS case(s) regressed"
exit 1
