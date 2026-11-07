#!/usr/bin/env bash
# scripts/tests/backup-db-selftest.sh — regression arms for scripts/backup-db.sh (OB-OPS-006).
#
# Deterministic and offline: every arm runs against a throwaway SQLite fixture
# seeded from the tracked schema (sql/schema/schema.sql) under a temp dir. No
# network, no credentials, and never the live store.
#
# Arms:
#   A  live-write safety — a dump taken while a writer is committing still passes
#      integrity_check and contains every user table.
#   B  dump arm        — exits 0, writes a dump + manifest, prints `backup PASSED`.
#   C  drill arm       — exits 0, prints per-table counts and `drill PASSED`.
#   D  negative control— a TRUNCATED dump makes the drill exit NON-zero. Without
#      this arm a green drill could just mean "the comparison never ran".
#   E  verify arm      — a database with different counts fails `--verify`.
#
# Usage: bash scripts/tests/backup-db-selftest.sh   (or: make backup-selftest)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BACKUP_SH="${REPO_ROOT}/scripts/backup-db.sh"
SCHEMA="${REPO_ROOT}/sql/schema/schema.sql"
QUEUE_SCHEMA="${REPO_ROOT}/sql/schema/queue.sql"

PASS=0
FAIL=0
ok()   { PASS=$((PASS + 1)); printf 'PASS  %s\n' "$*"; }
bad()  { FAIL=$((FAIL + 1)); printf 'FAIL  %s\n' "$*"; }

[ -f "$BACKUP_SH" ] || { echo "missing ${BACKUP_SH}"; exit 1; }
[ -f "$SCHEMA" ]    || { echo "missing ${SCHEMA}"; exit 1; }
[ -f "$QUEUE_SCHEMA" ] || { echo "missing ${QUEUE_SCHEMA}"; exit 1; }
command -v sqlite3 >/dev/null 2>&1 || { echo "sqlite3 not on PATH"; exit 1; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

DB="${WORK}/fixture.db"
mkdir -p "${WORK}/dumps"

# Seed a fixture store from the real schema plus a handful of rows, so the
# row-count comparison has something to compare.
sqlite3 "$DB" < "$SCHEMA" || { echo "could not build fixture from schema"; exit 1; }
sqlite3 "$DB" < "$QUEUE_SCHEMA" || { echo "could not build queue fixture"; exit 1; }
sqlite3 "$DB" <<'SQL'
insert into problem_classes (title, description) values
  ('sqlite-fts-shadow-tables-missing-after-restore', 'fixture'),
  ('wal-not-checkpointed-before-copy', 'fixture'),
  ('pytest-ambient-shadowing', 'fixture');
insert into answer_nodes (class_id, env, lang, version, solution, evidence, status) values
  (1, 'docker', 'go', '1.26', 'fixture solution', 'fixture evidence', 'verified'),
  (2, 'k8s', 'python', '3.11', 'fixture solution', 'fixture evidence', 'verified'),
  (3, 'bare-metal', 'bash', '5.2', 'fixture solution', 'fixture evidence', 'pending');
insert into problem_edges (source_id, target_id, relationship, weight) values
  (1, 2, 'same_root_cause', 1.0), (2, 3, 'prerequisite', 0.5);
insert into queue_entries (id, problem_class, status) values
  ('sub_fixture1', 'sqlite-fts-shadow-tables-missing-after-restore', 'complete'),
  ('sub_fixture2', 'wal-not-checkpointed-before-copy', 'pending');
SQL

run_backup() {
  OFF_BY_ONE_DB="$DB" OFF_BY_ONE_BACKUP_DIR="${WORK}/dumps" OFF_BY_ONE_BACKUP_KEEP=0 \
    bash "$BACKUP_SH" "$@"
}

# --- Arm A: live-write safety ------------------------------------------------
# A writer keeps committing while the backup runs; the dump must still be a
# consistent, openable database (this is what `.backup` buys over `cp`).
(
  for _ in $(seq 1 400); do
    sqlite3 "$DB" "insert into queue_entries (id, problem_class) values ('sub_churn_'||abs(random()), 'churn');" >/dev/null 2>&1 || true
    sleep 0.005
  done
) &
WRITER_PID=$!
OUT_A="$(run_backup 2>&1)"
RC_A=$?
kill "$WRITER_PID" 2>/dev/null
wait "$WRITER_PID" 2>/dev/null

DUMP_A="$(ls -1t "${WORK}/dumps"/off-by-one-*.db 2>/dev/null | head -1)"
if [ "$RC_A" -eq 0 ] && printf '%s' "$OUT_A" | grep -q 'backup PASSED'; then
  ok "A live-write dump exits 0 and prints backup PASSED"
else
  bad "A live-write dump rc=${RC_A}: ${OUT_A}"
fi
if [ -n "$DUMP_A" ] && [ "$(sqlite3 -batch -noheader "$DUMP_A" 'pragma integrity_check;')" = "ok" ]; then
  ok "A dump taken under a live writer passes integrity_check"
else
  bad "A dump under a live writer is not a valid database"
fi

# --- Arm B: dump + manifest --------------------------------------------------
DUMP="$(ls -1t "${WORK}/dumps"/off-by-one-*.db 2>/dev/null | head -1)"
if [ -n "$DUMP" ] && [ -s "$DUMP" ] && [ -s "${DUMP}.manifest" ]; then
  ok "B dump and manifest exist ($(basename "$DUMP"), $(wc -l < "${DUMP}.manifest" | tr -d ' ') manifest lines)"
else
  bad "B dump or manifest missing"
fi

TABLES_DUMPED="$(sqlite3 -batch -noheader "$DUMP" "select count(*) from sqlite_master where type='table' and name not like 'sqlite_%';" 2>/dev/null)"
if [ "${TABLES_DUMPED:-0}" -ge 4 ]; then
  ok "B dump carries every user table (${TABLES_DUMPED})"
else
  bad "B dump carries ${TABLES_DUMPED:-0} user tables, expected >= 4"
fi

# --- Arm C: drill ------------------------------------------------------------
OUT_C="$(run_backup --drill 2>&1)"
RC_C=$?
if [ "$RC_C" -eq 0 ] && printf '%s' "$OUT_C" | grep -q 'drill PASSED'; then
  ok "C --drill exits 0 and prints drill PASSED"
else
  bad "C --drill rc=${RC_C}: ${OUT_C}"
fi
if printf '%s' "$OUT_C" | grep -q 'problem_classes' && printf '%s' "$OUT_C" | grep -q 'answer_nodes'; then
  ok "C --drill prints per-table compared counts"
else
  bad "C --drill printed no per-table counts"
fi

# --- Arm D: negative control (truncated dump) --------------------------------
TRUNC="${WORK}/truncated.db"
head -c 1024 "$DUMP" > "$TRUNC"
cp "${DUMP}.manifest" "${TRUNC}.manifest" 2>/dev/null || true
OUT_D="$(run_backup --drill "$TRUNC" 2>&1)"
RC_D=$?
if [ "$RC_D" -ne 0 ]; then
  ok "D truncated dump makes --drill exit ${RC_D} (negative control live)"
else
  bad "D truncated dump PASSED the drill — the comparison is not live: ${OUT_D}"
fi

# --- Arm E: verify against a differing database ------------------------------
OTHER="${WORK}/other.db"
sqlite3 "$OTHER" < "$SCHEMA"
sqlite3 "$OTHER" "insert into problem_classes (title) values ('only-in-other');"
OUT_E="$(run_backup --verify "$OTHER" 2>&1)"
RC_E=$?
if [ "$RC_E" -ne 0 ]; then
  ok "E --verify on a differing database exits ${RC_E}"
else
  bad "E --verify passed on a differing database: ${OUT_E}"
fi

# A matching database must still pass, so E cannot be satisfied by failing always.
SAME="${WORK}/same.db"
sqlite3 "$DB" ".backup '${SAME}'"
OUT_E2="$(run_backup --verify "$SAME" 2>&1)"
RC_E2=$?
if [ "$RC_E2" -eq 0 ] && printf '%s' "$OUT_E2" | grep -q 'verify PASSED'; then
  ok "E --verify on an identical snapshot exits 0"
else
  bad "E --verify on an identical snapshot rc=${RC_E2}: ${OUT_E2}"
fi

printf '\n%d passed, %d failed\n' "$PASS" "$FAIL"
[ "$FAIL" -eq 0 ]
