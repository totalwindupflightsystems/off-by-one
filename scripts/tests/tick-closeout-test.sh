#!/bin/bash
# Regression test for DF-OFF-BY-ONE-52 — the tick close-out caller
# (scripts/tick-closeout.sh) must actually INVOKE the deploy gate and propagate
# its verdict, not merely document it.
#
# The defect: OB-GAP-085 shipped scripts/gate-deploy with no caller anywhere
# (CI ran only the hermetic check-deploy-test), so a superseded image served
# for 4 h 45 m while the gate sat red with nobody to trip it. This test pins
# the caller shut:
#
#   [A] green gate  -> caller invokes it (marker written) and exits 0
#   [B] red gate    -> caller invokes it and propagates the NON-ZERO exit
#   [C] SKIP gate   -> caller relays the loud SKIP verbatim and exits 0
#   [D] missing gate     -> caller fails closed (a missing gate is never a pass)
#   [E] non-executable gate -> caller fails closed with its own message
#   [F] no override -> the caller's default resolves to the REAL
#       scripts/gate-deploy in this checkout: running it from this worktree
#       invokes the real gate, which must SKIP loudly (worktree) and exit 0.
#
# Deterministic: temp dirs and stub gates only. No systemctl, no live-service
# contact; case F exercises the real gate's worktree SKIP path, which by
# design never probes the unit from a worktree.
#
# Usage: scripts/tests/tick-closeout-test.sh   (or: make tick-closeout-selftest)
#   exit 0 = all cases hold
#   exit 1 = a case regressed
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CALLER="$REPO_ROOT/scripts/tick-closeout.sh"
REAL_GATE="$REPO_ROOT/scripts/gate-deploy"

ROOT="$(mktemp -d /tmp/ob1-tick-closeout.XXXXXX)"
FAILS=0

cleanup() { rm -rf "$ROOT"; }
trap cleanup EXIT

ok()  { echo "  ✔ $1"; }
bad() { echo "  ✘ $1"; FAILS=$((FAILS + 1)); }

echo "caller under test: $CALLER"
echo "real gate:         $REAL_GATE"

# Stub gate factory: $1=path, $2=exit code, $3=verdict line. Every stub writes
# a marker OUTSIDE itself so the test proves invocation happened — a caller
# that greps docs instead of running the gate leaves no marker.
make_stub_gate() {
	local path="$1" rc="$2" line="$3" marker="$4"
	cat > "$path" <<STUB
#!/bin/sh
echo "$line"
echo invoked > "$marker"
exit $rc
STUB
	chmod +x "$path"
}

# ── Case A: green gate -> invoked + exit 0 ──
echo "[A] green gate must be invoked and propagated as exit 0"
A="$ROOT/a"
mkdir -p "$A"
make_stub_gate "$A/gate" 0 "gate-deploy: PASS — stub green" "$A/invoked"
OB1_GATE_DEPLOY="$A/gate" bash "$CALLER" >"$A/out" 2>&1; rc=$?
if [ "$rc" -eq 0 ] && [ -f "$A/invoked" ] && grep -q "gate-deploy: PASS" "$A/out"; then
	ok "green gate invoked (marker present), PASS relayed, caller exit 0"
else
	bad "green gate not invoked/propagated (rc=$rc, marker $([ -f "$A/invoked" ] && echo present || echo MISSING)): $(tail -1 "$A/out")"
fi

# ── Case B: red gate -> invoked + non-zero propagated unchanged ──
echo "[B] red gate must be invoked and its failure propagated"
B="$ROOT/b"
mkdir -p "$B"
make_stub_gate "$B/gate" 1 "gate-deploy: FAIL — stub red" "$B/invoked"
OB1_GATE_DEPLOY="$B/gate" bash "$CALLER" >"$B/out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && [ -f "$B/invoked" ] && grep -q "gate-deploy: FAIL" "$B/out" \
   && grep -q "tick-closeout: FAIL" "$B/out"; then
	ok "red gate invoked (marker present), exit 1 propagated, close-out refuses"
else
	bad "red gate failure not propagated (rc=$rc, marker $([ -f "$B/invoked" ] && echo present || echo MISSING)): $(tail -1 "$B/out")"
fi

# ── Case C: SKIP gate (worktree/foreign host) -> relayed verbatim, exit 0 ──
echo "[C] SKIP verdict must pass through unchanged (gate SKIP semantics preserved)"
C="$ROOT/c"
mkdir -p "$C"
make_stub_gate "$C/gate" 0 "gate-deploy: SKIP — deploy gate not applicable in this checkout (stub)" "$C/invoked"
OB1_GATE_DEPLOY="$C/gate" bash "$CALLER" >"$C/out" 2>&1; rc=$?
if [ "$rc" -eq 0 ] && [ -f "$C/invoked" ] && grep -q "gate-deploy: SKIP" "$C/out"; then
	ok "SKIP relayed verbatim, caller exit 0 — SKIP semantics preserved"
else
	bad "SKIP verdict mangled (rc=$rc): $(tail -1 "$C/out")"
fi

# ── Case D: missing gate -> fail closed ──
echo "[D] missing gate must fail the close-out (a missing gate is never a pass)"
D="$ROOT/d"
mkdir -p "$D"
OB1_GATE_DEPLOY="$D/absent-gate" bash "$CALLER" >"$D/out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && grep -q "tick-closeout: FAIL" "$D/out" && grep -q "missing" "$D/out"; then
	ok "missing gate fails closed with a named remedy (exit 1)"
else
	bad "missing gate did not fail closed (rc=$rc): $(tail -1 "$D/out")"
fi

# ── Case E: non-executable gate -> fail closed with its own message ──
echo "[E] non-executable gate must fail closed as its own case"
E="$ROOT/e"
mkdir -p "$E"
printf '#!/bin/sh\nexit 0\n' > "$E/gate"
chmod -x "$E/gate"
OB1_GATE_DEPLOY="$E/gate" bash "$CALLER" >"$E/out" 2>&1; rc=$?
if [ "$rc" -eq 1 ] && grep -q "not executable" "$E/out"; then
	ok "non-executable gate fails closed (exit 1)"
else
	bad "non-executable gate not rejected (rc=$rc): $(tail -1 "$E/out")"
fi

# ── Case F: default wiring -> the REAL gate, invoked from this worktree ──
# No OB1_GATE_DEPLOY override: the caller must resolve to the tracked
# scripts/gate-deploy. This checkout is a git worktree of the off-by-one repo,
# so the real gate must take its worktree SKIP path (exit 0, loud NOTE) — never
# probing the live unit. This is the end-to-end arm: caller -> real gate.
echo "[F] default caller must invoke the REAL scripts/gate-deploy (worktree SKIP arm)"
if git -C "$REPO_ROOT" rev-parse --git-dir >/dev/null 2>&1; then
	( cd "$REPO_ROOT" && bash scripts/tick-closeout.sh ) >"$ROOT/f.out" 2>&1; rc=$?
	if [ "$rc" -eq 0 ] && grep -q "gate-deploy: SKIP" "$ROOT/f.out" \
	   && grep -qi "worktree" "$ROOT/f.out"; then
		ok "default wiring invoked the real gate; real gate SKIPs loudly from a worktree (exit 0)"
	else
		bad "default wiring did not reach the real gate's SKIP path (rc=$rc): $(tail -2 "$ROOT/f.out")"
	fi
else
	echo "  (not a git checkout — case F not applicable here)"
fi

echo
if [ "$FAILS" -eq 0 ]; then
	echo "PASS — all DF-OFF-BY-ONE-52 tick-closeout caller cases hold"
	exit 0
fi
echo "FAIL — $FAILS case(s) regressed"
exit 1
