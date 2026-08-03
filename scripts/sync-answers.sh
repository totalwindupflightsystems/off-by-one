#!/bin/bash
# Sync the flat-file answer distribution to GitHub.
# Runs the export, commits + pushes if anything changed.
# Silent (no output) when nothing changed — safe for cron watchdog mode.
set -euo pipefail

REPO="${OB1_REPO:-$HOME/off-by-one}"
cd "$REPO"

# Regenerate the flat-file export from SQLite
python3 scripts/export-answers.py >/tmp/ob1_export.log 2>&1 || {
  echo "❌ export-answers.py failed:"
  cat /tmp/ob1_export.log
  exit 1
}

# Nothing changed → stay silent (watchdog pattern)
if git diff --quiet data/ scripts/export-answers.py README.md; then
  exit 0
fi

git add data/ scripts/export-answers.py README.md
git commit -q -m "data: sync answer corpus — $(git diff --cached --numstat | wc -l) files changed

Auto-exported from SQLite by export-answers.py (cron)." || exit 0

# Push (retry once on transient failure)
if ! git push -q origin master 2>/tmp/ob1_push.log; then
  sleep 5
  git push -q origin master 2>>/tmp/ob1_push.log || {
    echo "❌ push failed:"
    cat /tmp/ob1_push.log
    exit 1
  }
fi

# Report what changed (this output is delivered by cron)
echo "📦 Answer corpus synced to GitHub:"
git show --stat --oneline HEAD | head -8
echo ""
echo "Stats: $(grep -c '^' data/answers.jsonl) verified answers · $(ls data/answers/ | wc -l) problem classes"
