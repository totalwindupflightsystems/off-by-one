#!/usr/bin/env bash
# scripts/backup-db.sh — consistent backup + restore drill for the Off-by-One graph store.
#
# The lab's product IS its SQLite database: the accumulated problem-class tree
# (problem_classes), the answer graph (answer_nodes / problem_edges), the FTS
# indexes over both, and queue_entries. The file is gitignored (`*.db`, .gitignore:22)
# and the daemon opens it read-write for the whole life of the process, so a copy of
# the live file is not a backup and a `cp` of it is actively wrong.
#
# Three arms, all offline-safe:
#
#   backup (default)   Take an ONLINE consistent snapshot through the SQLite backup
#                      API (`.backup`), i.e. the same mechanism as `VACUUM INTO`, never
#                      a filesystem copy of a live database. Then prove the artifact is
#                      real before anyone trusts it: integrity_check must be `ok`, the
#                      table count must be non-zero, and a per-table row-count manifest
#                      is written beside the dump so a later drill has something to
#                      compare against that no live write can move.
#   --drill            Restore the newest dump into a throwaway database, verify
#                      integrity, and compare EVERY table's row count against the
#                      dump-time manifest. Exit 1 on any mismatch, including a table
#                      that exists on only one side.
#   --verify <path>    The same comparison against an arbitrary database file, no
#                      restore. This is the operator verb for "is this copy really the
#                      same graph?" and it is what makes the negative control cheap.
#
# Environment:
#   OFF_BY_ONE_DB            path to the live store           (default ./off-by-one.db, the
#                            same precedence cmd/off-by-one/main.go uses)
#   OFF_BY_ONE_BACKUP_DIR    dump destination                 (default <repo>/backups)
#   OFF_BY_ONE_BACKUP_KEEP   dumps to retain                  (default 14, 0 = keep all)
#
# Usage:
#   bash scripts/backup-db.sh                  # dump + manifest + retention
#   bash scripts/backup-db.sh --drill           # restore newest dump, compare counts
#   bash scripts/backup-db.sh --verify <db>     # compare live store against <db>
#   bash scripts/backup-db.sh --drill /path/to/dump.db   # drill one specific dump
#
# Exit codes: 0 success/gates passed, 1 any gate failed, 2 usage.
#
# Runbook: docs/runbooks/backup-restore.md
# Board row: OB-OPS-006

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

DB_PATH="${OFF_BY_ONE_DB:-${REPO_ROOT}/off-by-one.db}"
BACKUP_DIR="${OFF_BY_ONE_BACKUP_DIR:-${REPO_ROOT}/backups}"
KEEP="${OFF_BY_ONE_BACKUP_KEEP:-14}"

log()  { printf '%s\n' "$*"; }
die()  { printf 'FAIL: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "required tool not found: $1"; }

need sqlite3
need sha256sum

# Row counts for every user table, as "name|count" lines, sorted by name.
# sqlite_* internals are excluded; FTS shadow tables are INCLUDED on purpose —
# they are part of the searchable graph and a restore that loses them loses
# full-text discovery, so a count comparison must see them.
table_counts() {
  local db="$1" t n
  local -a rows=()
  while IFS= read -r t; do
    [ -n "$t" ] || continue
    n="$(sqlite3 -batch -noheader "$db" "select count(*) from \"${t//\"/\"\"}\";")" || return 1
    rows+=("${t}|${n}")
  done < <(sqlite3 -batch -noheader "$db" \
            "select name from sqlite_master where type='table' and name not like 'sqlite_%' order by name;")
  [ "${#rows[@]}" -gt 0 ] || return 1
  printf '%s\n' "${rows[@]}" | sort
}

integrity_ok() {
  local db="$1" res
  res="$(sqlite3 -batch -noheader "$db" 'pragma integrity_check;' 2>&1)" || return 1
  [ "$res" = "ok" ] || return 1
}

# Compare a database against a manifest file: "name|count" lines, sorted.
compare_to_manifest() {
  local db="$1" manifest="$2" tmp rc=0 name count actual
  tmp="$(mktemp -d)"
  if ! integrity_ok "$db"; then
    log "integrity_check: FAILED for ${db}"
    rm -rf "$tmp"
    return 1
  fi
  table_counts "$db" > "${tmp}/actual" || { log "could not read table counts from ${db}"; rm -rf "$tmp"; return 1; }
  table_counts_from_manifest "$manifest" > "${tmp}/expected" || { log "manifest unreadable: ${manifest}"; rm -rf "$tmp"; return 1; }

  log "compared $(wc -l < "${tmp}/expected" | tr -d ' ') tables:"
  while IFS='|' read -r name count; do
    actual="$(awk -F'|' -v n="$name" '$1==n{print $2}' "${tmp}/actual")"
    if [ -z "$actual" ]; then
      log "  MISSING-IN-RESTORE  ${name}  expected=${count}"
      rc=1
    elif [ "$actual" != "$count" ]; then
      log "  MISMATCH            ${name}  expected=${count} actual=${actual}"
      rc=1
    else
      log "  ok                  ${name}  rows=${count}"
    fi
  done < "${tmp}/expected"
  # A table present in the restored copy but absent from the manifest is a
  # restore-time surprise (a shadow table added by a newer SQLite); report it
  # rather than silently ignoring it.
  while IFS='|' read -r name count; do
    if ! awk -F'|' -v n="$name" '$1==n{found=1} END{exit !found}' "${tmp}/expected"; then
      log "  EXTRA-IN-RESTORE    ${name}  rows=${count}"
      rc=1
    fi
  done < "${tmp}/actual"
  rm -rf "$tmp"
  return "$rc"
}

# The manifest is a plain text mirror of `name|count` plus a small header, so it
# can be read without trusting the dump itself.
table_counts_from_manifest() {
  local manifest="$1"
  grep -v '^#' "$manifest" | grep -v '^$'
}

do_backup() {
  [ -f "$DB_PATH" ] || die "live store not found: ${DB_PATH}"
  mkdir -p "$BACKUP_DIR"
  local stamp out manifest
  stamp="$(date -u +%Y%m%dT%H%M%SZ)"
  out="${BACKUP_DIR}/off-by-one-${stamp}.db"
  manifest="${out}.manifest"

  log "source:  ${DB_PATH}"
  log "target:  ${out}"
  # `.backup` takes a consistent snapshot of a database other connections are
  # actively writing. Never `cp` this file. A writer holding the lock past the
  # busy handler aborts the copy with "database is locked", so set a busy timeout
  # and retry a bounded number of times; the partial file is removed before each
  # attempt so a failed run cannot leave a healthy-looking artifact behind.
  local attempt rc=1
  for attempt in 1 2 3; do
    rm -f "$out"
    if sqlite3 -batch -cmd '.timeout 10000' "$DB_PATH" ".backup '${out}'" 2>"${out}.err"; then
      rc=0
      break
    fi
    log "attempt ${attempt}: $(tr '\n' ' ' < "${out}.err")"
    sleep 1
  done
  rm -f "${out}.err"
  if [ "$rc" -ne 0 ]; then
    rm -f "$out" "$manifest"
    die "sqlite3 .backup failed after 3 attempts (another process may hold an exclusive lock)"
  fi

  [ -s "$out" ] || die "dump is empty: ${out}"
  integrity_ok "$out" || die "dump failed integrity_check: ${out}"

  table_counts "$out" > "${manifest}.tmp" || die "could not count tables in the dump"
  local n_tables
  n_tables="$(wc -l < "${manifest}.tmp" | tr -d ' ')"
  [ "$n_tables" -gt 0 ] || die "dump contains no user tables"

  {
    printf '# off-by-one graph store dump manifest\n'
    printf '# source: %s\n' "$DB_PATH"
    printf '# dump:   %s\n' "$(basename "$out")"
    printf '# utc:    %s\n' "$stamp"
    printf '# sha256: %s\n' "$(sha256sum "$out" | awk '{print $1}')"
    printf '# tables: %s\n' "$n_tables"
    cat "${manifest}.tmp"
  } > "$manifest"
  rm -f "${manifest}.tmp"

  log "sha256:  $(sha256sum "$out" | awk '{print $1}')"
  log "tables:  ${n_tables}"
  log "manifest: ${manifest}"
  log "backup PASSED"

  if [ "$KEEP" -gt 0 ] 2>/dev/null; then
    # Retention deletes only dumps THIS script wrote, newest first.
    local old
    old="$(ls -1t "${BACKUP_DIR}"/off-by-one-*.db 2>/dev/null | tail -n +$((KEEP + 1)) || true)"
    if [ -n "$old" ]; then
      while IFS= read -r f; do
        [ -n "$f" ] || continue
        rm -f "$f" "$f.manifest"
        log "pruned:  $(basename "$f")"
      done <<< "$old"
    fi
  fi
}

newest_dump() {
  ls -1t "${BACKUP_DIR}"/off-by-one-*.db 2>/dev/null | head -1
}

do_drill() {
  local dump="${1:-}"
  if [ -z "$dump" ]; then
    dump="$(newest_dump)"
    [ -n "$dump" ] || die "no dump found in ${BACKUP_DIR} — run the backup arm first"
  fi
  [ -f "$dump" ] || die "dump not found: ${dump}"
  local manifest="${dump}.manifest"
  local scratch
  scratch="$(mktemp -d)"
  local restored="${scratch}/restored.db"
  log "drill dump: ${dump}"
  log "restore -> ${restored}"

  # Restoring a SQLite dump is a file copy into a THROWAWAY path — the live store
  # is never opened for write, never moved, and never touched.
  cp "$dump" "$restored" || { rm -rf "$scratch"; die "could not stage the dump for restore"; }

  local rc=0
  if [ -f "$manifest" ]; then
    compare_to_manifest "$restored" "$manifest" || rc=1
  else
    log "WARN: no manifest beside the dump; comparing against the LIVE store (counts may drift)"
    table_counts "$DB_PATH" > "${scratch}/live" || { rm -rf "$scratch"; die "could not read the live store"; }
    local live_manifest="${scratch}/live.manifest"
    printf '# live store snapshot\n' > "$live_manifest"
    cat "${scratch}/live" >> "$live_manifest"
    compare_to_manifest "$restored" "$live_manifest" || rc=1
  fi

  rm -rf "$scratch"
  if [ "$rc" -ne 0 ]; then
    log "drill FAILED"
    return 1
  fi
  log "drill PASSED"
}

do_verify() {
  local other="${1:-}"
  [ -n "$other" ] || { log "usage: $0 --verify <database-file>" >&2; exit 2; }
  [ -f "$other" ] || die "database not found: ${other}"
  [ -f "$DB_PATH" ] || die "live store not found: ${DB_PATH}"
  local scratch
  scratch="$(mktemp -d)"
  table_counts "$DB_PATH" > "${scratch}/live"
  local m="${scratch}/m"
  printf '# live store snapshot\n' > "$m"
  cat "${scratch}/live" >> "$m"
  local rc=0
  compare_to_manifest "$other" "$m" || rc=1
  rm -rf "$scratch"
  if [ "$rc" -ne 0 ]; then
    log "verify FAILED (${other} does not match ${DB_PATH})"
    return 1
  fi
  log "verify PASSED"
}

case "${1:-}" in
  ""|backup)   do_backup ;;
  --drill)     do_drill "${2:-}" ;;
  --verify)    do_verify "${2:-}" ;;
  -h|--help)   sed -n '2,40p' "${BASH_SOURCE[0]}"; exit 0 ;;
  *)           log "unknown argument: $1 (try --help)" >&2; exit 2 ;;
esac
