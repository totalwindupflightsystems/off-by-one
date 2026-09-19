#!/usr/bin/env bash
# Self-test for scripts/check-deploy (OB-GAP-077). Deterministic: the three
# --resolve-stamp cases run against this repo's git state only (no systemctl,
# no /proc). The LIVE section runs `make check-deploy` end-to-end when the
# off-by-one unit is visible to systemctl and asserts ONLY that the check runs
# and prints a verdict — the live server may legitimately be stale mid-tick
# (that is the failure mode check-deploy exists to catch).
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CHECK="$SCRIPT_DIR/check-deploy"
OUT="$(mktemp)"
trap 'rm -f "$OUT"' EXIT

cd "$REPO_ROOT"

failures=0
pass() {
	echo "PASS: $*"
}
fail() {
	echo "FAIL: $*"
	failures=$((failures + 1))
}

# --- FRESH: HEAD's stamp resolves with an empty code-path diff ---------------
head_stamp="$(git describe --tags --always)"
head_stamp_rc=0
"$CHECK" --resolve-stamp "$head_stamp" >"$OUT" 2>&1 || head_stamp_rc=$?
if [ "$head_stamp_rc" -eq 0 ]; then
	pass "fresh stamp '$head_stamp' resolves (exit 0)"
elif [ "$head_stamp_rc" -eq 1 ] && ! grep -q "^FAIL: stamp" "$OUT" && \
	! grep -q "$(git rev-parse --short HEAD)" "$OUT"; then
	# The resolver itself is non-deterministic only under a concurrent git
	# process: a set -e death can surface as rc=1 WITHOUT any FAIL line (or a
	# code-path verdict). Re-run once; only a second identical failure fails.
	head_stamp_rc2=0
	"$CHECK" --resolve-stamp "$head_stamp" >"$OUT" 2>&1 || head_stamp_rc2=$?
	if [ "$head_stamp_rc2" -eq 0 ]; then
		pass "fresh stamp '$head_stamp' resolves (exit 0; first run died on a transient git failure)"
	else
		fail "fresh stamp '$head_stamp' failed twice (rc=$head_stamp_rc then rc=$head_stamp_rc2); output:"
		sed 's/^/    /' "$OUT"
	fi
else
	fail "fresh stamp '$head_stamp' should exit 0; rc=$head_stamp_rc; output:"
	sed 's/^/    /' "$OUT"
fi

# --- DIVERGENCE: parent of the last code commit must fail naming the remedy --
last_code_commit="$(git log -1 --format=%h -- cmd/ internal/ web/ sql/ pkg/ go.mod go.sum)"
divergence_stamp="${last_code_commit}^"
set +e
"$CHECK" --resolve-stamp "$divergence_stamp" >"$OUT" 2>&1
divergence_rc=$?
set -e
if [ "$divergence_rc" -ne 0 ] && grep -q "make build" "$OUT"; then
	pass "divergence stamp '$divergence_stamp' exits non-zero and names the remedy"
else
	fail "divergence stamp '$divergence_stamp': rc=$divergence_rc (want non-zero + 'make build' in output); output:"
	sed 's/^/    /' "$OUT"
fi

# --- GARBAGE: an unparseable stamp must fail ---------------------------------
set +e
"$CHECK" --resolve-stamp not-a-real-stamp >"$OUT" 2>&1
garbage_rc=$?
set -e
if [ "$garbage_rc" -ne 0 ]; then
	pass "garbage stamp exits non-zero"
else
	fail "garbage stamp should exit non-zero; output:"
	sed 's/^/    /' "$OUT"
fi

# --- LIVE: end-to-end make check-deploy (verdict presence only) --------------
if systemctl show off-by-one -p MainPID --value >/dev/null 2>&1; then
	echo "--- LIVE: make check-deploy (end-to-end; asserts verdict presence only) ---"
	set +e
	make check-deploy >"$OUT" 2>&1
	live_rc=$?
	set -e
	sed 's/^/    /' "$OUT"
	echo "    exit code: $live_rc"
	if grep -q "^check-deploy: " "$OUT"; then
		pass "live make check-deploy ran and printed a check-deploy verdict (rc=$live_rc)"
	else
		fail "live make check-deploy printed no 'check-deploy:' verdict line (rc=$live_rc)"
	fi
else
	echo "SKIP: live section — 'systemctl show off-by-one' failed on this host"
fi

echo "---"
if [ "$failures" -gt 0 ]; then
	echo "check-deploy-test: $failures failure(s)"
	exit 1
fi
echo "check-deploy-test: all assertions passed"
