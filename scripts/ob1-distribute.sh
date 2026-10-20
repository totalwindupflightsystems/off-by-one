#!/bin/bash
# ob1-distribute.sh — merged ob1 distribution (answer corpus + public catalog)
# Merged 2026-08-27 (Bane: ob1 had 3-4 jobs, merge into one) from:
#   - ob1-sync-answers.sh  (export answers -> GitHub push)
#   - ob1-public-sync.sh   (ship binary + DB snapshot -> public catalog box)
# Both ran at the same 4x/day cadence from the same repo, distributing the
# same artifacts — one script, one cron. Silent on success (watchdog pattern).
#
# CANONICAL COPY: this file is tracked in the repo. The cron entrypoint is an
# untracked deployed copy at ~/.hermes/scripts/ob1-distribute.sh; re-deploy it
# after any change here:
#     make deploy-distribute
# (`make check-distribute-deploy` verifies the pair; the drift guard below
# refuses to run a stale copy — OB-DIST-003.)
#
# PART 2 (host publish) delegates to the tracked scripts/publish-catalog.sh,
# which owns the transport-class retry, the staged-pair activation and the
# operator configuration — see docs/publish-transport.md.
#
# ── drift guard (OB-DIST-003) — must stay ABOVE the `set -euo pipefail` line:
# scripts/tests/ob1-distribute-regression.sh slices this script from that line
# onward and runs the slice from /tmp; a guard inside the slice would compare
# the slice against the canonical and fail every OB-DIST-001 case.
_ob1_canonical="${OB1_REPO:-$HOME/off-by-one}/scripts/ob1-distribute.sh"
_ob1_self_sha="$(sha256sum "$0" 2>/dev/null | awk '{print $1}')"
_ob1_canon_sha="$(sha256sum "$_ob1_canonical" 2>/dev/null | awk '{print $1}')"
if [ -z "$_ob1_self_sha" ] || [ -z "$_ob1_canon_sha" ] || [ "$_ob1_self_sha" != "$_ob1_canon_sha" ]; then
  echo "❌ ob1-distribute.sh drift guard: the running copy is not byte-identical to the tracked canonical" >&2
  echo "   running:   $0 (sha256 ${_ob1_self_sha:-<unreadable>})" >&2
  echo "   canonical: $_ob1_canonical (sha256 ${_ob1_canon_sha:-<unreadable>})" >&2
  echo "   remedy: make deploy-distribute" >&2
  exit 1
fi
unset _ob1_canonical _ob1_self_sha _ob1_canon_sha

set -euo pipefail

REPO="${OB1_REPO:-$HOME/off-by-one}"
cd "$REPO"

# ══════════════ PART 1 — answer corpus + static site -> GitHub ══════════════
# Exports the corpus (export-answers.py), regenerates the public static site
# (generate-static-site.py: site/index.html, site/classes/, site/sitemap.xml)
# and commits data/ + site/ + both scripts + README to GitHub.
python3 scripts/export-answers.py >/tmp/ob1_export.log 2>&1 || {
  echo "❌ export-answers.py failed:"
  cat /tmp/ob1_export.log
  exit 1
}

/usr/bin/python3 scripts/generate-static-site.py >/tmp/ob1_site.log 2>&1 || {
  echo "❌ generate-static-site.py failed:"
  cat /tmp/ob1_site.log
  exit 1
}

if ! git diff --quiet data/ site/ scripts/export-answers.py scripts/generate-static-site.py README.md; then
  git add data/ site/ scripts/export-answers.py scripts/generate-static-site.py README.md
  # FAIL LOUD (OB-DIST-001). The previous form was `git commit ... || true`, so a
  # rejected commit (hook, index lock, killed process) degraded the whole
  # distribution into a silent no-op: the script still printed "Answer corpus
  # synced to GitHub" while showing the STALE head, the push answered
  # "Everything up-to-date", and the cron run reported success — the corpus never
  # reached GitHub and the only trace was a dirty index. Observed 2026-10-19 17:00
  # local (guard PASSed, no commit object, reflog unchanged, run
  # 2026-10-19_17-00-49.md claimed success). Every step below is now verified.
  PRE_HEAD="$(git rev-parse HEAD)"
  set +e
  git commit -q -m "data: sync answer corpus — $(git diff --cached --numstat | wc -l) files changed

Auto-exported from SQLite by export-answers.py (cron)."
  COMMIT_RC=$?
  set -e
  if [ "$COMMIT_RC" -ne 0 ]; then
    echo "❌ corpus commit FAILED (rc=$COMMIT_RC) — nothing was distributed; working tree still dirty:"
    git status --short | head -20
    exit 1
  fi
  NEW_HEAD="$(git rev-parse HEAD)"
  if [ "$NEW_HEAD" = "$PRE_HEAD" ]; then
    echo "❌ corpus commit did not advance HEAD (${PRE_HEAD}) — refusing to report success"
    exit 1
  fi
  if ! git push -q origin master 2>/tmp/ob1_push.log; then
    sleep 5
    git push -q origin master 2>>/tmp/ob1_push.log || {
      echo "❌ GitHub push failed:"
      cat /tmp/ob1_push.log
      exit 1
    }
  fi
  # A push that answered "Everything up-to-date" (or any silent no-op) must never
  # be reported as distribution: verify the REMOTE carries the commit we just made.
  REMOTE_HEAD="$(git ls-remote origin refs/heads/master 2>/dev/null | awk '{print $1}')"
  if [ "$REMOTE_HEAD" != "$NEW_HEAD" ]; then
    if ! git fetch -q origin master 2>/dev/null || ! git merge-base --is-ancestor "$NEW_HEAD" origin/master; then
      echo "❌ push did not land: origin/master=${REMOTE_HEAD:-<unreachable>} does not contain ${NEW_HEAD}"
      exit 1
    fi
  fi
  echo "📦 Answer corpus synced to GitHub:"
  git show --stat --oneline "$NEW_HEAD" | head -8
  echo ""
fi

# ══════════════ PART 2 — binary + DB snapshot -> public catalog host ══════════════
# Tracked implementation: scripts/publish-catalog.sh
#   * transient ssh/scp resets retry the whole idempotent staged pair, bounded
#   * non-transport failures are attempted once and return their rc
#   * activation only after BOTH transfers, and never blindly retried
#   * operator knobs: OB1_PUBLISH_BOX / OB1_PUBLISH_MODE / TRANSPORT_RETRIES
# Exit codes are distinct (1 local/non-transport, 2 transport exhausted,
# 3 operator action required, 4 unhealthy after activation, 5 config,
# 6 activation refused an incomplete staged pair).
set +e
bash "$REPO/scripts/publish-catalog.sh"
PUBLISH_RC=$?
set -e
if [ "$PUBLISH_RC" -ne 0 ]; then
  echo "❌ public catalog publish leg failed (rc=$PUBLISH_RC) — verdict above"
  exit "$PUBLISH_RC"
fi

# Silent on full success (both parts healthy)
exit 0
