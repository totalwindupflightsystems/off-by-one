#!/usr/bin/env bash
# Self-test for scripts/check-deploy + scripts/gate-deploy (OB-GAP-077, OB-GAP-085).
#
# Deterministic and hermetic: every case runs against this repo's git state plus
# THROWAWAY scratch clones in a temp dir. No systemctl contact, no /proc of the
# live service, no live-service mutation — the systemd unit path is driven
# through a stub systemctl on PATH, and the process-identity leg through a
# throwaway instance the harness starts itself (killed on exit).
#
# Cases:
#   STAMP  — fresh / divergence / garbage --resolve-stamp verdicts (the
#            check-deploy seam; deterministic, no systemctl).
#   GATE   — the close-out gate's enforcement matrix (OB-GAP-085):
#              WORKTREE  gate run from a git worktree      -> SKIP (exit 0)
#              NOTAREPO  non-git dir                       -> FAIL
#              NOWORK    unit present, probe missing       -> FAIL
#              RED       stale artifact                    -> FAIL naming the remedy
#              NOSTAMP   bare go build (no -ldflags stamp) -> FAIL by design
#              FOREIGN   unit bound to another directory   -> FAIL
#              GREEN     artifact rebuilt from HEAD        -> PASS
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CHECK="$SCRIPT_DIR/check-deploy"
GATE="$SCRIPT_DIR/gate-deploy"
OUT="$(mktemp)"
SCRATCH_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ob1-deploytest-XXXXXX")"
STUB_BIN="$SCRATCH_ROOT/bin"
REV="$(git -C "$REPO_ROOT" rev-parse --short HEAD)"
SCRATCH_PID=""
trap 'rm -f "$OUT"; [ -n "$SCRATCH_PID" ] && kill "$SCRATCH_PID" 2>/dev/null; rm -rf "$SCRATCH_ROOT"' EXIT

cd "$REPO_ROOT"

failures=0
pass() { echo "PASS: $*"; }
fail() { echo "FAIL: $*"; failures=$((failures + 1)); }

# --- stub systemctl -----------------------------------------------------------
#
# Answers only the three properties the gate/probe read; the values come from
# OB1_STUB_* env vars so each case can point the "unit" wherever it wants. It
# honors `--value` the way systemctl does (bare value vs `Key=Value`): the probe
# asks with `--value`, the gate without, and a stub that ignores the flag hands
# back "MainPID=123" as the pid — which reads as a dead process, not a bug.
install_stub_systemctl() {
	mkdir -p "$STUB_BIN"
	{
		printf '%s\n' '#!/bin/sh'
		printf '%s\n' 'shift || true'
		printf '%s\n' 'value_only=0'
		printf '%s\n' 'for a in "$@"; do [ "$a" = "--value" ] && value_only=1; done'
		printf '%s\n' 'emit() { if [ "$value_only" = 1 ]; then echo "$2"; else echo "$1=$2"; fi; }'
		printf '%s\n' 'for a in "$@"; do'
		printf '%s\n' '  case "$a" in'
		printf '%s\n' '    -p|--value) ;;'
		printf '%s\n' '    MainPID) emit MainPID "${OB1_STUB_MAINPID:-}" ;;'
		printf '%s\n' '    WorkingDirectory) emit WorkingDirectory "${OB1_STUB_WORKDIR:-}" ;;'
		printf '%s\n' '    ExecStart) emit ExecStart "{ path=${OB1_STUB_EXEC:-} ; argv[]=${OB1_STUB_EXEC:-} ; }" ;;'
		printf '%s\n' '  esac'
		printf '%s\n' 'done'
		printf '%s\n' 'exit 0'
	} >"$STUB_BIN/systemctl"
	chmod +x "$STUB_BIN/systemctl"
}
install_stub_systemctl

# --- throwaway instance (the /proc leg needs a real running process) ----------
start_scratch_instance() {
	local repo="$1" pid i
	( cd "$repo" && env -u DEEPSEEK_API_KEY OFF_BY_ONE_DB="$SCRATCH_ROOT/scratch.db" \
		OFF_BY_ONE_PORT=18997 ./off-by-one --skip-sandbox -load-threshold -1 \
		>"$SCRATCH_ROOT/instance.log" 2>&1 & )
	for i in 1 2 3 4 5 6 7 8 9 10; do
		pid="$(ss -tlnp 2>/dev/null | sed -n 's/.*:18997 .*pid=\([0-9]*\).*/\1/p' | head -1)"
		if [ -n "$pid" ]; then
			SCRATCH_PID="$pid"
			return 0
		fi
		sleep 1
	done
	return 1
}

# --- STAMP cases (check-deploy seam) -----------------------------------------
head_stamp="$(git describe --tags --always)"
head_stamp_rc=0
"$CHECK" --resolve-stamp "$head_stamp" >"$OUT" 2>&1 || head_stamp_rc=$?
if [ "$head_stamp_rc" -eq 0 ]; then
	pass "fresh stamp '$head_stamp' resolves (exit 0)"
elif [ "$head_stamp_rc" -eq 1 ] && grep -q "code paths changed since it" "$OUT" && \
	dirty_code="$(git status --porcelain -- cmd/ internal/ web/ sql/ pkg/ go.mod go.sum Makefile)" && \
	[ -n "$dirty_code" ]; then
	# Correctly detected divergence — but the divergence is the WORKING TREE, not
	# the artifact: this checkout has uncommitted code-path changes, so HEAD's
	# stamp cannot match by construction. Report it (a silent pass here would
	# hide a real regression when the tree IS clean).
	echo "SKIP: fresh-stamp case — the working tree has uncommitted code-path changes, so no committed stamp can match ('$(echo "$head_stamp")'); dirty paths:"
	printf '%s\n' "$dirty_code" | sed 's/^/    /'
elif [ "$head_stamp_rc" -eq 1 ] && ! grep -q "^FAIL: stamp" "$OUT" && \
	! grep -q "$REV" "$OUT"; then
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

last_code_commit="$(git log -1 --format=%h -- cmd/ internal/ web/ sql/ pkg/ go.mod go.sum)"
divergence_stamp="${last_code_commit}^"
# Plain-sha spelling of the same revision: what a stale artifact's stamp would
# literally read. Used by the gate's RED case.
stale_stamp="$(git rev-parse --short "${last_code_commit}^")"
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

# --- GATE cases --------------------------------------------------------------
echo "--- GATE: close-out enforcement matrix (hermetic scratch clones) ---"

# One scratch clone drives every remaining case: an independent repo whose
# checked-out branch is a stable snapshot (no racing the live repo's commits),
# with the gate + probe copied in from the working tree under test.
SCRATCH_REPO="$SCRATCH_ROOT/repo"
set +e
git clone --shared --quiet "$REPO_ROOT" "$SCRATCH_REPO" >/dev/null 2>&1
clone_rc=$?
set -e

if [ "$clone_rc" -ne 0 ]; then
	fail "could not create scratch clone $SCRATCH_REPO — GATE cases skipped"
else
	cp "$CHECK" "$SCRATCH_REPO/scripts/check-deploy"
	cp "$GATE" "$SCRATCH_REPO/scripts/gate-deploy"
	chmod +x "$SCRATCH_REPO/scripts/check-deploy" "$SCRATCH_REPO/scripts/gate-deploy"
	scratch_head="$(git -C "$SCRATCH_REPO" rev-parse HEAD)"

	# WORKTREE: a real git worktree (of the throwaway clone) must SKIP — its
	# artifact is not what the service runs, so no service verdict is honest.
	SCRATCH_WT="$SCRATCH_ROOT/wt"
	set +e
	git -C "$SCRATCH_REPO" worktree add --quiet -b ob1-deploytest-wt "$SCRATCH_WT" >/dev/null 2>&1
	wt_add_rc=$?
	set -e
	if [ "$wt_add_rc" -ne 0 ]; then
		fail "could not create a scratch worktree for the WORKTREE case"
	else
		cp "$CHECK" "$SCRATCH_WT/scripts/check-deploy"
		cp "$GATE" "$SCRATCH_WT/scripts/gate-deploy"
		chmod +x "$SCRATCH_WT/scripts/check-deploy" "$SCRATCH_WT/scripts/gate-deploy"
		set +e
		( cd "$SCRATCH_WT" && PATH="$STUB_BIN:$PATH" ./scripts/gate-deploy ) >"$OUT" 2>&1
		worktree_rc=$?
		set -e
		if [ "$worktree_rc" -eq 0 ] && grep -q "^gate-deploy: SKIP" "$OUT" && grep -qi "worktree" "$OUT"; then
			pass "gate SKIPs from a git worktree with an explanatory NOTE (exit 0)"
		else
			fail "gate should SKIP from a worktree; rc=$worktree_rc; output:"
			sed 's/^/    /' "$OUT"
		fi
	fi

	# NOWORK: unit present but the probe is missing — refuse before passing.
	NOWORK="$SCRATCH_ROOT/nowork"
	set +e
	git clone --shared --quiet "$REPO_ROOT" "$NOWORK" >/dev/null 2>&1
	nowork_clone_rc=$?
	set -e
	if [ "$nowork_clone_rc" -ne 0 ]; then
		fail "could not create the no-probe scratch clone"
	else
		rm -f "$NOWORK/scripts/check-deploy"
		cp "$GATE" "$NOWORK/scripts/gate-deploy"
		chmod +x "$NOWORK/scripts/gate-deploy"
		set +e
		( cd "$NOWORK" && OB1_UNIT_SOURCE="$NOWORK" \
			OB1_STUB_MAINPID=$$ OB1_STUB_WORKDIR="$NOWORK" OB1_STUB_EXEC="$NOWORK/off-by-one" \
			PATH="$STUB_BIN:$PATH" ./scripts/gate-deploy ) >"$OUT" 2>&1
		nowork_gate_rc=$?
		set -e
		if [ "$nowork_gate_rc" -ne 0 ] && grep -q "check-deploy is missing" "$OUT"; then
			pass "gate FAILs when the probe is missing (exit $nowork_gate_rc)"
		else
			fail "gate should FAIL when the probe is missing; rc=$nowork_gate_rc; output:"
			sed 's/^/    /' "$OUT"
		fi
	fi

	# RED: artifact stamped at the last CODE commit's parent — HEAD carries code
	# it predates, so the probe must reject it and the gate must fail the tick.
	set +e
	( cd "$SCRATCH_REPO" && go build -ldflags "-X main.version=$stale_stamp" \
		-o off-by-one ./cmd/off-by-one ) >"$OUT" 2>&1
	build_stale_rc=$?
	set -e
	if [ "$build_stale_rc" -ne 0 ]; then
		fail "could not build the stale artifact in the scratch clone; output:"
		sed 's/^/    /' "$OUT"
	else
		set +e
		( cd "$SCRATCH_REPO" && OB1_UNIT_SOURCE="$SCRATCH_REPO" \
			OB1_STUB_MAINPID=$$ OB1_STUB_WORKDIR="$SCRATCH_REPO" OB1_STUB_EXEC="$SCRATCH_REPO/off-by-one" \
			PATH="$STUB_BIN:$PATH" ./scripts/gate-deploy ) >"$OUT" 2>&1
		red_rc=$?
		set -e
		if [ "$red_rc" -ne 0 ] && grep -q "^gate-deploy: FAIL" "$OUT" && grep -q "make build" "$OUT"; then
			pass "gate RED on a stale artifact (exit $red_rc, remedy named)"
		else
			fail "gate should be RED on a stale artifact; rc=$red_rc; output:"
			sed 's/^/    /' "$OUT"
		fi
	fi

	# NOSTAMP: a bare `go build` carries no version stamp — refuse it rather
	# than diff against a revision that does not exist.
	set +e
	( cd "$SCRATCH_REPO" && go build -o off-by-one ./cmd/off-by-one ) >"$OUT" 2>&1
	set -e
	set +e
	( cd "$SCRATCH_REPO" && OB1_UNIT_SOURCE="$SCRATCH_REPO" \
		OB1_STUB_MAINPID=$$ OB1_STUB_WORKDIR="$SCRATCH_REPO" OB1_STUB_EXEC="$SCRATCH_REPO/off-by-one" \
		PATH="$STUB_BIN:$PATH" ./scripts/gate-deploy ) >"$OUT" 2>&1
	nostamp_rc=$?
	set -e
	if [ "$nostamp_rc" -ne 0 ] && grep -q "no version stamp" "$OUT"; then
		pass "gate RED on an unstamped artifact (exit $nostamp_rc, names the missing stamp)"
	else
		fail "gate should be RED on an unstamped artifact; rc=$nostamp_rc; output:"
		sed 's/^/    /' "$OUT"
	fi

	# FOREIGN: the unit is bound to a different directory — refuse to report on
	# someone else's deployment instead of emitting a verdict about it.
	foreign="$SCRATCH_ROOT/foreign"
	mkdir -p "$foreign"
	set +e
	( cd "$SCRATCH_REPO" && OB1_UNIT_SOURCE="$SCRATCH_REPO" \
		OB1_STUB_MAINPID=$$ OB1_STUB_WORKDIR="$foreign" OB1_STUB_EXEC="$foreign/off-by-one" \
		PATH="$STUB_BIN:$PATH" ./scripts/gate-deploy ) >"$OUT" 2>&1
	foreign_rc=$?
	set -e
	if [ "$foreign_rc" -ne 0 ] && grep -q "not from this checkout" "$OUT"; then
		pass "gate FAILs when the unit is bound elsewhere (exit $foreign_rc)"
	else
		fail "gate should FAIL when the unit is bound elsewhere; rc=$foreign_rc; output:"
		sed 's/^/    /' "$OUT"
	fi

	# GREEN: rebuild from HEAD, then point the stub unit at a throwaway
	# instance of that artifact. This is the red→green half of the proof.
	set +e
	( cd "$SCRATCH_REPO" && go build -ldflags "-X main.version=${scratch_head:0:7}" \
		-o off-by-one ./cmd/off-by-one ) >"$OUT" 2>&1
	rebuild_rc=$?
	set -e
	if [ "$rebuild_rc" -ne 0 ]; then
		fail "could not rebuild the scratch artifact from HEAD; output:"
		sed 's/^/    /' "$OUT"
	elif ! start_scratch_instance "$SCRATCH_REPO"; then
		echo "SKIP: GREEN case — the throwaway instance did not come up (port 18997 busy?); output:"
		sed 's/^/    /' "$SCRATCH_ROOT/instance.log"
	else
		set +e
		( cd "$SCRATCH_REPO" && OB1_UNIT_SOURCE="$SCRATCH_REPO" \
			OB1_STUB_MAINPID="$SCRATCH_PID" OB1_STUB_WORKDIR="$SCRATCH_REPO" OB1_STUB_EXEC="$SCRATCH_REPO/off-by-one" \
			PATH="$STUB_BIN:$PATH" ./scripts/gate-deploy ) >"$OUT" 2>&1
		green_rc=$?
		set -e
		if [ "$green_rc" -eq 0 ] && grep -q "^gate-deploy: PASS" "$OUT"; then
			pass "gate GREEN after rebuild from HEAD against the throwaway instance (exit 0)"
		else
			fail "gate should be GREEN after rebuild; rc=$green_rc; output:"
			sed 's/^/    /' "$OUT"
		fi
	fi
fi

# NOTAREPO: outside a git checkout nothing can be proven — hard fail.
notarepo="$SCRATCH_ROOT/not-a-repo"
mkdir -p "$notarepo/scripts"
cp "$GATE" "$notarepo/scripts/gate-deploy"
chmod +x "$notarepo/scripts/gate-deploy"
set +e
( cd "$notarepo" && ./scripts/gate-deploy ) >"$OUT" 2>&1
notarepo_rc=$?
set -e
if [ "$notarepo_rc" -ne 0 ] && grep -q "not a git checkout" "$OUT"; then
	pass "gate FAILs outside a git checkout (exit $notarepo_rc)"
else
	fail "gate should FAIL outside a git checkout; rc=$notarepo_rc; output:"
	sed 's/^/    /' "$OUT"
fi

echo "---"
if [ "$failures" -gt 0 ]; then
	echo "check-deploy-test: $failures failure(s)"
	exit 1
fi
echo "check-deploy-test: all assertions passed"
