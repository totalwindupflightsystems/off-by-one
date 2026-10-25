#!/usr/bin/env bash
# scripts/install-hooks.sh — install this repo's tracked hooks
# (scripts/hooks/*) into the hooks directory git ACTUALLY consults for this
# checkout (DF-OFF-BY-ONE-55).
#
# Why this exists: a core.hooksPath set at global/system scope (e.g. a
# fleet-wide hooks dir) makes git ignore $GIT_DIR/hooks entirely — a hook
# installed only there is inert. This installer
#   1. installs scripts/hooks/pre-commit into the repo's OWN hooks dir
#      (resolved via `git rev-parse --git-common-dir`, so it is also correct
#      from a linked worktree), and
#   2. when core.hooksPath is in effect and resolves somewhere else, sets a
#      REPO-LOCAL core.hooksPath pointing at the repo's hooks dir so git
#      consults it. Only this repo's .git/config is written; global/user
#      config and any foreign hooks dir are never modified.
#
# Idempotent; safe to re-run. A differing pre-existing pre-commit hook is
# backed up next to the target before being replaced.
#
# Usage: bash scripts/install-hooks.sh   (or: make install-hooks)

set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
common_dir="$(git rev-parse --git-common-dir)"
case "$common_dir" in
	/*) : ;;
	*)  common_dir="$repo_root/$common_dir" ;;
esac
hooks_dir="${common_dir%/}/hooks"

src="$repo_root/scripts/hooks/pre-commit"
dst="$hooks_dir/pre-commit"
[ -f "$src" ] || { echo "install-hooks: missing tracked source $src" >&2; exit 1; }

mkdir -p "$hooks_dir"
if [ -f "$dst" ] && ! cmp -s "$src" "$dst"; then
	backup="$dst.bak-$(date -u +%Y%m%dT%H%M%SZ)"
	cp -p "$dst" "$backup"
	echo "install-hooks: existing pre-commit differed; backed up to $backup"
fi
install -m 0755 "$src" "$dst"
echo "install-hooks: installed $dst"

# If core.hooksPath (any scope) currently steers git away from the repo's own
# hooks dir, counter it with a repo-local override.
configured="$(git config --get core.hooksPath || true)"
resolved="$(git rev-parse --git-path hooks)"
case "$resolved" in
	/*) : ;;
	*)  resolved="$repo_root/$resolved" ;;
esac
resolved="${resolved%/}"
if [ -n "$configured" ] && [ "$resolved" != "$hooks_dir" ]; then
	git config core.hooksPath "$hooks_dir"
	echo "install-hooks: core.hooksPath was '$configured' (non-local scope); set repo-local core.hooksPath=$hooks_dir"
fi

echo "install-hooks: git resolves pre-commit to $(git rev-parse --git-path hooks/pre-commit)"
