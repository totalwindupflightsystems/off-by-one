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
#     cp scripts/ob1-distribute.sh ~/.hermes/scripts/ob1-distribute.sh
#     chmod +x ~/.hermes/scripts/ob1-distribute.sh
#
# PART 2 (host publish) delegates to the tracked scripts/publish-catalog.sh,
# which owns the transport-class retry, the staged-pair activation and the
# operator configuration — see docs/publish-transport.md.
set -euo pipefail

REPO="${OB1_REPO:-$HOME/off-by-one}"
cd "$REPO"

# ══════════════ PART 1 — answer corpus -> GitHub ══════════════
python3 scripts/export-answers.py >/tmp/ob1_export.log 2>&1 || {
  echo "❌ export-answers.py failed:"
  cat /tmp/ob1_export.log
  exit 1
}

if ! git diff --quiet data/ scripts/export-answers.py README.md; then
  git add data/ scripts/export-answers.py README.md
  git commit -q -m "data: sync answer corpus — $(git diff --cached --numstat | wc -l) files changed

Auto-exported from SQLite by export-answers.py (cron)." || true
  if ! git push -q origin master 2>/tmp/ob1_push.log; then
    sleep 5
    git push -q origin master 2>>/tmp/ob1_push.log || {
      echo "❌ GitHub push failed:"
      cat /tmp/ob1_push.log
      exit 1
    }
  fi
  echo "📦 Answer corpus synced to GitHub:"
  git show --stat --oneline HEAD | head -8
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
