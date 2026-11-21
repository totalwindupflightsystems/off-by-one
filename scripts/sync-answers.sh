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

# Regenerate the static bot-facing site (site/) from the corpus. Uses the
# system python (has the markdown lib); idempotent when nothing changed.
/usr/bin/python3 scripts/generate-static-site.py >/tmp/ob1_site.log 2>&1 || {
  echo "❌ generate-static-site.py failed:"
  cat /tmp/ob1_site.log
  exit 1
}

# Nothing changed → stay silent (watchdog pattern)
if git diff --quiet data/ site/ scripts/export-answers.py scripts/generate-static-site.py README.md; then
  exit 0
fi

# data/answers.jsonl is a GENERATED, gitignored artifact (OBS-OB-006): `git add
# data/` skips it automatically, and it must never be force-added — committing
# it cost history tens of MB per export. The tracked corpus is data/answers/.
git add data/ site/ scripts/export-answers.py scripts/generate-static-site.py README.md
git commit -q -m "data: sync answer corpus — $(git diff --cached --numstat | wc -l) files changed

Auto-exported from SQLite by export-answers.py + generate-static-site.py (cron)." || exit 0

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
# Counts come from the tracked per-class files, not from the generated bulk
# artifact (which may be absent on a fresh clone).
answer_total=$(python3 -c 'import glob,json;print(sum(len(json.load(open(p))["answers"]) for p in glob.glob("data/answers/*.json")))')
echo "Stats: ${answer_total} verified answers · $(ls data/answers/ | wc -l) problem classes"
