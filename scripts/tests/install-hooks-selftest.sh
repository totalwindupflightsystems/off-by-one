#!/usr/bin/env bash
# scripts/tests/install-hooks-selftest.sh — regression self-test for
# scripts/install-hooks.sh + scripts/hooks/pre-commit (acceptance for
# DF-OFF-BY-ONE-55).
#
# What it proves, all inside a throwaway local clone of this repo (the real
# checkout is never touched):
#   1. the installer makes git resolve pre-commit to the CLONE's own
#      .git/hooks — never a machine-global core.hooksPath dir;
#   2. the installed hook is fail-closed: no boardctl_lint_mode="warn";
#   3. a commit staging a DUPLICATE board row id is REJECTED — pre-commit
#      exits non-zero, HEAD does not move, and the hook output names the
#      duplicated id;
#   4. a commit staging a valid new board row still passes (the gate is not
#      a blanket wedge: gitreins guard + board-lint both run and pass).
#
# The clone only carries COMMITTED content, so the working-tree hook sources
# under test are overlaid into the clone before the installer runs; after this
# change lands, the tracked files are byte-identical and the overlay is a no-op
# in content.
#
# Usage: bash scripts/tests/install-hooks-selftest.sh   (or: make install-hooks-selftest)

set -uo pipefail

SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SELF_DIR/../.." && pwd)"

WORK="$(mktemp -d /tmp/oby-install-hooks-selftest.XXXXXX)"
trap 'rm -rf "$WORK"' EXIT

fail() { echo "SELFTEST FAIL: $*" >&2; exit 1; }

command -v gitreins >/dev/null 2>&1 || fail "gitreins not on PATH (repo commit contract requires the guard)"
command -v boardctl >/dev/null 2>&1 || fail "boardctl not on PATH (board-lint gate under test)"

CLONE="$WORK/clone"
git clone --quiet "$REPO_ROOT" "$CLONE" || fail "git clone"
mkdir -p "$CLONE/scripts/hooks"
cp "$REPO_ROOT/scripts/hooks/pre-commit" "$CLONE/scripts/hooks/pre-commit" || fail "overlay hook source"
cp "$REPO_ROOT/scripts/install-hooks.sh" "$CLONE/scripts/install-hooks.sh" || fail "overlay installer"
cd "$CLONE" || fail "cd clone"
git config user.email selftest@example.invalid
git config user.name "install-hooks selftest"

# Arm 1+2: installer lands the repo-local hook and git consults it.
bash scripts/install-hooks.sh >"$WORK/install.log" 2>&1 || {
	cat "$WORK/install.log" >&2
	fail "install-hooks.sh exited non-zero"
}
effective="$(git rev-parse --git-path hooks/pre-commit)"
case "$effective" in
	"$CLONE"/.git/hooks/pre-commit|.git/hooks/pre-commit) : ;;
	*) fail "pre-commit resolves to '$effective', not the repo-local .git/hooks" ;;
esac
[ -x .git/hooks/pre-commit ] || fail ".git/hooks/pre-commit not installed/executable"
if grep -q 'boardctl_lint_mode="warn"' .git/hooks/pre-commit; then
	fail "installed hook still carries boardctl_lint_mode=\"warn\""
fi

# Arm 3: duplicate-id commit is rejected, HEAD unchanged, id named.
DUP_ID="DF-OFF-BY-ONE-55"
BOARD=.coding-hermes/board/tasks.jsonl
[ -f "$BOARD" ] || fail "clone carries no board"
python3 - "$BOARD" "$DUP_ID" <<'PYEOF'
import json, sys
path, dup_id = sys.argv[1], sys.argv[2]
row = None
for line in open(path):
    try:
        r = json.loads(line)
    except Exception:
        continue
    if r.get('id') == dup_id:
        row = r
        break
if row is None:
    sys.exit('fixture row %s not found' % dup_id)
open(path, 'a').write(json.dumps(row) + '\n')
PYEOF
[ $? -eq 0 ] || fail "fixture append"
head_before="$(git rev-parse HEAD)"
git add "$BOARD" || fail "git add duplicate fixture"
git commit -m "selftest: duplicate board id (must be rejected)" >"$WORK/dup.log" 2>&1
dup_rc=$?
[ "$dup_rc" -ne 0 ] || fail "duplicate-id commit was ACCEPTED (rc=0)"
[ "$(git rev-parse HEAD)" = "$head_before" ] || fail "HEAD moved on rejected commit"
grep -q "$DUP_ID" "$WORK/dup.log" || fail "hook output did not name the duplicated id $DUP_ID"
grep -q 'board-lint: commit rejected' "$WORK/dup.log" || fail "hook output missing the board-lint rejection line"

# Arm 4: a valid new board row commits cleanly (gate is not a wedge).
git restore --staged "$BOARD" || fail "unstage"
git checkout -- "$BOARD" || fail "restore board"
python3 - "$BOARD" <<'PYEOF'
import json, sys
path = sys.argv[1]
row = {"id": "SELFTEST-INSTALL-HOOKS-1", "title": "selftest row (throwaway clone)",
       "status": "pending", "priority": "P4", "complexity": 1,
       "depends_on": [], "blocks": []}
open(path, 'a').write(json.dumps(row) + '\n')
PYEOF
[ $? -eq 0 ] || fail "valid fixture append"
git add "$BOARD" || fail "git add valid fixture"
git commit -m "selftest: valid board row (must pass)" >"$WORK/ok.log" 2>&1 || {
	cat "$WORK/ok.log" >&2
	fail "valid board-row commit was rejected"
}
[ "$(git rev-parse HEAD)" != "$head_before" ] || fail "HEAD did not advance on valid commit"

echo "SELFTEST PASS: install-hooks + fail-closed board-lint (clone: $CLONE)"
