# Backup and restore — Off-by-One graph store

The lab's durable product is a single SQLite database: the problem-class tree, the
answer graph (`answer_nodes`, `problem_edges`), the full-text indexes over both, and
`queue_entries`. The file is **gitignored** (`*.db`, `.gitignore:22`), the repository
holds only the schema (`sql/schema/schema.sql`, `sql/schema/queue.sql`) and the
exported answer corpus (`data/answers/*.json`). A lost database file is therefore a
lost solve graph — schema plus corpus cannot rebuild it.

Owner of this procedure: **the ops lane of the off-by-one family** (`off-by-one-ops`),
with the on-call host owner for the cron arm. Board row: `OB-OPS-006`.

## The store

| item | value |
|---|---|
| path | `$OFF_BY_ONE_DB`, default `./off-by-one.db` relative to the daemon's working directory (`cmd/off-by-one/main.go:415`) |
| live instance | `/home/kara/off-by-one/off-by-one.db` (+ `-wal`, `-shm`) |
| writer | the daemon (`off-by-one --port 8766 -cron-interval 60s`) holds the file open read-write for its whole life |
| schema | applied from `sql/schema/schema.sql` and `sql/schema/queue.sql` |

Because the daemon writes continuously, **never copy the file** (`cp`, `rsync`, `tar`
of a live SQLite file can capture a torn page plus a WAL that no longer matches). All
dumps go through the SQLite online backup API.

## Taking a backup

```bash
bash scripts/backup-db.sh          # or: make backup
```

What it does:

1. `.backup` — the SQLite online backup API — into
   `$OFF_BY_ONE_BACKUP_DIR/off-by-one-<UTC stamp>.db` (default `<repo>/backups/`).
   A busy handler with a 10 s timeout plus 3 bounded retries means a writer holding
   the lock does not turn into a corrupt artifact; a failed attempt is deleted before
   the retry, so the dump directory never holds a half-written "good looking" file.
2. `pragma integrity_check` on the dump must answer `ok`.
3. A per-table row-count **manifest** (`<dump>.manifest`) is written beside the dump,
   together with its sha256. The manifest is what a later drill compares against — the
   counts come from the snapshot, so no live write can make a drill flaky.
4. Retention: the newest `$OFF_BY_ONE_BACKUP_KEEP` dumps are kept (default 14, `0` =
   keep everything). Pruning only ever removes files this script wrote.

Exit 0 and `backup PASSED` means the dump exists and is provably openable.

## Verifying a restore (the drill)

```bash
bash scripts/backup-db.sh --drill            # newest dump, or:
bash scripts/backup-db.sh --drill /path/to/off-by-one-<stamp>.db
```

The drill stages the dump into a throwaway directory (the live store is never opened
for write, never moved), runs `integrity_check`, and compares **every** user table's
row count against the manifest, printing `ok` / `MISMATCH` / `MISSING-IN-RESTORE` /
`EXTRA-IN-RESTORE` per table. Any difference exits non-zero with `drill FAILED`.

`--verify <db>` runs the same comparison against an arbitrary database file without a
restore — the operator verb for "prove this copy is the same graph before I trust it".

Both arms are covered by `make backup-selftest` (`scripts/tests/backup-db-selftest.sh`,
9 arms: live-writer safety, dump, manifest, drill, a truncated-dump negative control,
and `--verify` in both directions).

## Restoring for real

The restore is deliberately manual: replacing a live store is a destructive act and
must not be scripted behind an unattended surface.

1. Stop the daemon so nothing is mid-solve: `systemctl stop off-by-one` (or the
   supervisor in use on the host).
2. Move the current file aside — do not delete it:
   `mv off-by-one.db off-by-one.db.pre-restore-$(date -u +%Y%m%dT%H%M%SZ)`, and remove
   the stale `off-by-one.db-wal` / `-shm` (they belong to the replaced file; leaving
   them lets SQLite replay a WAL that no longer matches).
3. Copy the chosen dump into place: `cp backups/off-by-one-<stamp>.db off-by-one.db`.
4. Confirm the restored file before starting anything:
   `sqlite3 off-by-one.db 'pragma integrity_check;'` → `ok`, then
   `bash scripts/backup-db.sh --verify off-by-one.db` is not applicable (it compares
   against the live path) — instead run the drill arm against the dump you restored
   from, which is the same comparison performed before the copy.
5. Start the daemon and confirm liveness: `curl -sS --max-time 5 http://localhost:8766/`
   answers, and one discovery query returns rows
   (`POST /api/v1/problems/discover`).

## Schedule and its owner

The backup is not yet on a timer; the documented schedule is **daily, 03:15 host
local**, retention 14 (≈14 × the store size, currently ~56 MB each, ~800 MB steady
state):

```cron
15 3 * * * OFF_BY_ONE_BACKUP_KEEP=14 /bin/bash /home/kara/off-by-one/scripts/backup-db.sh >> /home/kara/off-by-one/backups/backup.log 2>&1
```

Owner of that cron entry: the **host owner** (`kara`), because it mutates host state
outside the project tree. It is deliberately not installed by the ops tick that wrote
this file — installing a host timer unattended is an owner decision. Until it is
installed, the manual `make backup` above is the only registered path, and a missed
run is invisible. A weekly `make backup-selftest` alongside it keeps the drill honest.

Failures are loud: the script prints `FAIL: …` to stderr and exits 1, so a cron entry
that captures stderr (as above) records the run as failed rather than silent.

## Measured transcript (2026-11-07, OB-OPS-006)

Against the live store `/home/kara/off-by-one/off-by-one.db` while the daemon was up
(PID 1871714, `off-by-one --port 8766 -cron-interval 60s`), dump destination a scratch
directory so no dump data lands in the repo:

- dump size 56,913,920 bytes, sha256 `12f1572f8fa65d391d0b37cc0d70b6934bb16b9f3f93d45fdcbea87042f7edcc`,
  `integrity_check` = `ok`, 14 tables in the manifest
- `--drill` compared all 14 tables → `drill PASSED`, exit 0:
  `answer_nodes` 4929, `answer_nodes_fts` 4929, `answer_nodes_fts_data` 1976,
  `answer_nodes_fts_docsize` 4929, `answer_nodes_fts_idx` 1595, `problem_classes` 3194,
  `problem_classes_fts` 3194, `problem_classes_fts_data` 58, `problem_classes_fts_docsize` 3194,
  `problem_classes_fts_idx` 56, `problem_edges` 11622, `queue_entries` 5657
  (plus the two `*_fts_config` tables at 1 row each)
- negative control (that dump truncated to 1 KB) → `integrity_check: FAILED`, `drill FAILED`,
  exit 1
- the live file's mtime was unchanged by both arms (05:59:28 -0500 before and after): the
  drill stages a throwaway copy and never opens the store for write
