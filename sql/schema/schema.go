// Package schemasql embeds the canonical Off-by-One SQLite schema as a
// string for use by the graph engine. Keeping the embed in a dedicated
// package means the schema is loaded exactly once at process start and
// shared across all callers.
package schemasql

import _ "embed"

//go:embed schema.sql
var Schema string

// FTS5Extra is the SQL to set up the FTS5 virtual tables and triggers.
// Kept in Go (not the schema file) because the FTS5 extension is loaded
// by modernc.org/sqlite and we want the triggers to be in source control
// alongside the rest of the FTS plumbing.
const FTS5Extra = `
CREATE VIRTUAL TABLE IF NOT EXISTS problem_classes_fts USING fts5(
    title,
    description,
    content='problem_classes',
    content_rowid='id',
    tokenize='porter unicode61'
);
CREATE TRIGGER IF NOT EXISTS problem_classes_ai AFTER INSERT ON problem_classes BEGIN
    INSERT INTO problem_classes_fts(rowid, title, description) VALUES (new.id, new.title, new.description);
END;
CREATE TRIGGER IF NOT EXISTS problem_classes_ad AFTER DELETE ON problem_classes BEGIN
    INSERT INTO problem_classes_fts(problem_classes_fts, rowid, title, description) VALUES('delete', old.id, old.title, old.description);
END;
CREATE TRIGGER IF NOT EXISTS problem_classes_au AFTER UPDATE ON problem_classes BEGIN
    INSERT INTO problem_classes_fts(problem_classes_fts, rowid, title, description) VALUES('delete', old.id, old.title, old.description);
    INSERT INTO problem_classes_fts(rowid, title, description) VALUES (new.id, new.title, new.description);
END;
CREATE VIRTUAL TABLE IF NOT EXISTS answer_nodes_fts USING fts5(
    solution,
    content='answer_nodes',
    content_rowid='id',
    tokenize='porter unicode61'
);
CREATE TRIGGER IF NOT EXISTS answer_nodes_ai AFTER INSERT ON answer_nodes BEGIN
    INSERT INTO answer_nodes_fts(rowid, solution) VALUES (new.id, new.solution);
END;
CREATE TRIGGER IF NOT EXISTS answer_nodes_ad AFTER DELETE ON answer_nodes BEGIN
    INSERT INTO answer_nodes_fts(answer_nodes_fts, rowid, solution) VALUES('delete', old.id, old.solution);
END;
CREATE TRIGGER IF NOT EXISTS answer_nodes_au AFTER UPDATE ON answer_nodes BEGIN
    INSERT INTO answer_nodes_fts(answer_nodes_fts, rowid, solution) VALUES('delete', old.id, old.solution);
    INSERT INTO answer_nodes_fts(rowid, solution) VALUES (new.id, new.solution);
END;
`
