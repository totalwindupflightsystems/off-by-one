#!/usr/bin/env bash
# scripts/tick-closeout.sh — the repository-owned tick CLOSE-OUT caller for the
# deploy gate (DF-OFF-BY-ONE-52).
#
# Why this exists: OB-GAP-085 shipped scripts/gate-deploy as "the enforced entry
# point a tick close-out calls", and the docs made `make gate-deploy` the
# documented close-out step — but NOTHING in the repository actually invoked it.
# CI ran only the hermetic self-test (check-deploy-test). The predictable
# happened: on the tick-442 close-out the "rebuild + redeploy" claim was
# unfalsifiable, /proc/<MainPID>/exe pointed at a deleted superseded image, and
# `make gate-deploy` sat RED (rc=1) for 4 h 45 m with no caller to trip it. A
# gate that nobody calls is not a gate. This script is the machine-side caller:
# tick close-out runs `make tick-closeout` (or scripts/tick-closeout.sh
# directly), and the deploy gate's verdict becomes the close-out verdict.
#
# What it does, deliberately little:
#   1. The gate must exist AND be executable in this checkout — a close-out
#      caller whose gate is missing is a broken close-out, never a pass
#      (mirrors gate-deploy leg C for its probe).
#   2. The gate runs, and its exit status IS this caller's exit status. The
#      gate's SKIP semantics are preserved by construction: run from a git
#      worktree or a host without the unit, scripts/gate-deploy itself exits 0
#      with a loud NOTE naming the checkout that owns the deployment, and this
#      caller relays that output verbatim. No weakening, no re-implementation
#      of the detection logic here.
#
# What it never does: restart or mutate the service. The remedy (make build;
# kill <MainPID>) belongs to the operator in the checkout that owns the unit —
# this caller only refuses to let a tick close out on a stale deployment.
#
# Environment (optional):
#   OB1_GATE_DEPLOY   gate path override (default: <checkout>/scripts/gate-deploy).
#                     Test seam only — scripts/tests/tick-closeout-test.sh points
#                     it at stub gates; production close-out never sets it.
#
# Hermetic regression test (no live-service contact):
#   scripts/tests/tick-closeout-test.sh   (make tick-closeout-selftest)
#
# ch:trace row=DF-OFF-BY-ONE-52 spec=docs/dogfood/diagnostics.md#tick-close-out-enforcement wave=none:serial-tick test=scripts/tests/tick-closeout-test.sh::caller-invokes-gate-and-propagates-verdict doc=docs/dogfood/diagnostics.md prompt=none:worker-brief evidence=scripts/tests/tick-closeout-test.sh witness=none:repository-local-verification verdict=none:pending-foreman-judge commit=none:pending-foreman-trace memory=none:pending-foreman-memory
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
GATE="${OB1_GATE_DEPLOY:-$REPO_ROOT/scripts/gate-deploy}"

fail() {
	printf '%s\n' "tick-closeout: FAIL — $1" >&2
	exit 1
}

cd "$REPO_ROOT" || fail "cannot cd to $REPO_ROOT"

# ---------------------------------------------------------------------------
# Leg 1: the gate must be present and runnable in this checkout. A missing
# gate can never hide behind a caller that "ran" nothing.
# ---------------------------------------------------------------------------
if [ ! -f "$GATE" ]; then
	fail "deploy gate is missing ($GATE) — tick close-out has nothing to enforce the deploy rotation with (restore it: git checkout -- scripts/gate-deploy)"
fi
if [ ! -x "$GATE" ]; then
	fail "deploy gate is not executable ($GATE) — run 'chmod +x scripts/gate-deploy' and re-run the close-out"
fi

# ---------------------------------------------------------------------------
# Leg 2: run the gate. Its exit status IS the close-out verdict — PASS, FAIL
# and the loud worktree/foreign-host SKIP all propagate unchanged.
# ---------------------------------------------------------------------------
"$GATE"
rc=$?
if [ "$rc" -ne 0 ]; then
	printf '%s\n' "tick-closeout: FAIL — the deploy gate rejected this tick's close-out (gate exit $rc; see its output above)." >&2
	printf '%s\n' "tick-closeout: the tick stays open until 'make gate-deploy' passes in the checkout that owns the unit." >&2
	exit "$rc"
fi

printf '%s\n' "tick-closeout: gate verdict above is the deploy-rotation verdict for this tick (PASS, or an explicit SKIP naming the owning checkout)."
exit 0
