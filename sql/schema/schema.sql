-- Off-by-One: Problem-class tree with lateral graph edges
-- SQLite schema — identical to PostgreSQL version (uses same recursive CTE syntax)

CREATE TABLE IF NOT EXISTS problem_classes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    title       TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS answer_nodes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    class_id    INTEGER NOT NULL REFERENCES problem_classes(id),
    parent_id   INTEGER REFERENCES answer_nodes(id),  -- self-ref for version branches
    env         TEXT NOT NULL DEFAULT '',              -- docker, k8s, bare-metal, etc.
    lang        TEXT NOT NULL DEFAULT '',              -- go, python, typescript, etc.
    version     TEXT NOT NULL DEFAULT '',              -- python-3.11, go-1.26, etc.
    solution    TEXT NOT NULL DEFAULT '',
    evidence    TEXT NOT NULL DEFAULT '',              -- how the answer was verified
    signatures  TEXT NOT NULL DEFAULT '{}',            -- JSON: validator signatures
    status      TEXT NOT NULL DEFAULT 'pending',       -- pending, verified, failed, ci_passed
    created_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS problem_edges (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    source_id    INTEGER NOT NULL REFERENCES problem_classes(id),
    target_id    INTEGER NOT NULL REFERENCES problem_classes(id),
    relationship TEXT NOT NULL DEFAULT '',  -- same_root_cause, prerequisite, superseded_by, generalizes
    weight       REAL NOT NULL DEFAULT 1.0,
    created_at   TEXT NOT NULL DEFAULT (datetime('now')),
    UNIQUE(source_id, target_id, relationship)
);

CREATE INDEX IF NOT EXISTS idx_answer_nodes_class_id ON answer_nodes(class_id);
CREATE INDEX IF NOT EXISTS idx_answer_nodes_parent_id ON answer_nodes(parent_id);
CREATE INDEX IF NOT EXISTS idx_answer_nodes_env_lang ON answer_nodes(env, lang, version);
CREATE INDEX IF NOT EXISTS idx_problem_edges_source ON problem_edges(source_id);
CREATE INDEX IF NOT EXISTS idx_problem_edges_target ON problem_edges(target_id);

-- Discovery query: BFS from matched problem class
-- WITH RECURSIVE tree AS (
--     SELECT *, 0 as depth FROM answer_nodes WHERE class_id = ? AND env = ? AND version = ?
--     UNION ALL
--     SELECT a.*, t.depth + 1 FROM answer_nodes a JOIN tree t ON a.parent_id = t.id
-- )
-- SELECT DISTINCT a.*, e.relationship, e.weight
-- FROM tree t
-- LEFT JOIN problem_edges e ON e.source_id = t.class_id
-- LEFT JOIN answer_nodes a ON a.class_id = e.target_id
-- ORDER BY t.depth, e.weight DESC;
