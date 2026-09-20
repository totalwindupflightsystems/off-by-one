-- Off-by-One: Queue entries for the pre-solve lab
-- SQLite schema extension (separate from the main problem-class graph).

CREATE TABLE IF NOT EXISTS queue_entries (
    id           TEXT PRIMARY KEY,                -- human-readable ID like "sub_a1b2c3"
    problem_class TEXT NOT NULL,                  -- title of the problem class
    environment  TEXT NOT NULL DEFAULT '',
    language     TEXT NOT NULL DEFAULT '',
    version      TEXT NOT NULL DEFAULT '',
    description  TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    stack_trace  TEXT NOT NULL DEFAULT '',
    context_json TEXT NOT NULL DEFAULT '{}',      -- JSON blob of additional context
    required_tools TEXT NOT NULL DEFAULT '[]',     -- JSON-encoded array of tool names (e.g. ["jq","parallel"])
    cadence      TEXT NOT NULL DEFAULT 'pre-phase',
    priority     REAL NOT NULL DEFAULT 0.0,      -- computed priority score
    status       TEXT NOT NULL DEFAULT 'pending', -- pending, in_progress, complete, failed
    stage        TEXT NOT NULL DEFAULT 'queued',  -- queued, sandbox_solve, etc.
    result_answer_id INTEGER,                     -- FK to answer_nodes.id on success
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    started_at   TEXT,
    completed_at TEXT,
    failure_reason TEXT NOT NULL DEFAULT ''          -- why a solve failed (empty until status='failed')
);

-- idx_queue_entries_status covers the status filter alone; the three added
-- here exist because the serve paths read the queue in an ORDER the plain
-- index cannot supply, so SQLite sorted every matching row in a temp B-tree
-- before LIMIT could stop it:
--   status_created  → newest-first listing (created_at DESC, id ASC)
--   created         → newest-first listing with no status filter
--   status_priority → solver order (priority DESC, created_at ASC)
-- With them each paged read walks an index in order and stops at LIMIT
-- instead of sorting the whole status partition (OB-GAP-084). Measured on
-- 200k rows: status-filtered page 17.3ms → 0.11ms, unfiltered page 36ms →
-- 0.04ms, pending order 43ms → 1.2ms.
CREATE INDEX IF NOT EXISTS idx_queue_entries_status ON queue_entries(status);
CREATE INDEX IF NOT EXISTS idx_queue_entries_status_created ON queue_entries(status, created_at DESC, id ASC);
CREATE INDEX IF NOT EXISTS idx_queue_entries_created ON queue_entries(created_at DESC, id ASC);
CREATE INDEX IF NOT EXISTS idx_queue_entries_status_priority ON queue_entries(status, priority DESC, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_queue_entries_priority ON queue_entries(priority DESC, created_at);
CREATE INDEX IF NOT EXISTS idx_queue_entries_problem_class ON queue_entries(problem_class);
