// Full-text search powered by SQLite FTS5 virtual tables. The FTS5 tables
// (problem_classes_fts, answer_nodes_fts) are created by the schema's
// FTS5Extra SQL with content-sync triggers that keep the index in lockstep
// with the base tables. See sql/schema/schema.go for the DDL.
//
// The Search method is the single entry point used by:
//   - GET /api/v1/problems?q=<query>&env=...&lang=...&status=...
//   - Web UI search bar (debounced autocomplete)
//   - Muster discover tool (as a fallback when the graph discover has no
//     exact match)
//
// Results from both the problem-class index and the answer index are merged
// into a single ranked list. Each hit carries a snippet with the matched
// terms highlighted so the caller can display it without re-reading the full
// document.
package graph

import (
	"context"
	"fmt"
	"strings"
)

// Search runs a full-text query across problem_classes and answer_nodes.
// Results are ranked by FTS5 rank, with the matched snippet returned for
// display in the web UI.
//
// env, lang, status act as additional filters (empty = no filter).
// limit, offset paginate.
func (s *Store) Search(ctx context.Context, query, env, lang, status string, limit, offset int) ([]SearchHit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	// Escape the query for FTS5. We use double-quote wrapping so any
	// characters are treated as a phrase. FTS5 also has a "NEAR" syntax
	// that we don't expose — by quoting, the user gets literal matching.
	ftsQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`

	// We UNION two queries: problem_classes title/description and
	// answer_nodes solution. The answer query joins the FTS index with
	// its parent problem_class for the title. Status filter applies to
	// both.
	rows, err := s.db.QueryContext(ctx, `
		SELECT pc.id, pc.title, snippet(problem_classes_fts, 1, '[', ']', '…', 8) AS snip,
		       rank, NULL AS answer_id
		FROM problem_classes_fts
		JOIN problem_classes pc ON pc.id = problem_classes_fts.rowid
		WHERE problem_classes_fts MATCH ?
		  AND (? = '' OR EXISTS (
		      SELECT 1 FROM answer_nodes a
		      WHERE a.class_id = pc.id AND a.env = ?))
		  AND (? = '' OR EXISTS (
		      SELECT 1 FROM answer_nodes a
		      WHERE a.class_id = pc.id AND a.lang = ?))
		  AND (? = '' OR EXISTS (
		      SELECT 1 FROM answer_nodes a
		      WHERE a.class_id = pc.id AND (CASE WHEN ? = 'solved'
		                                         THEN a.status IN ('verified', 'ci_passed')
		                                         ELSE a.status = ? END)))
		UNION
		SELECT pc.id, pc.title, snippet(answer_nodes_fts, 0, '[', ']', '…', 8) AS snip,
		       rank, a.id AS answer_id
		FROM answer_nodes_fts
		JOIN answer_nodes a ON a.id = answer_nodes_fts.rowid
		JOIN problem_classes pc ON pc.id = a.class_id
		WHERE answer_nodes_fts MATCH ?
		  AND (? = '' OR a.env = ?)
		  AND (? = '' OR a.lang = ?)
		  AND (? = '' OR (CASE WHEN ? = 'solved'
		                       THEN a.status IN ('verified', 'ci_passed')
		                       ELSE a.status = ? END))
		ORDER BY rank
		LIMIT ? OFFSET ?
	`, ftsQuery, env, env, lang, lang, status, status, status,
		ftsQuery, env, env, lang, lang, status, status, status,
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()
	var out []SearchHit
	for rows.Next() {
		var hit SearchHit
		if err := rows.Scan(&hit.ClassID, &hit.Title, &hit.Snippet, &hit.Score, &hit.AnswerID); err != nil {
			return nil, fmt.Errorf("scan search hit: %w", err)
		}
		out = append(out, hit)
	}
	return out, rows.Err()
}

// SearchCount returns the total number of matches for a search query
// (same filters as Search, ignoring limit/offset). The API uses it to
// report an accurate total for pagination.
func (s *Store) SearchCount(ctx context.Context, query, env, lang, status string) (int, error) {
	if strings.TrimSpace(query) == "" {
		return 0, nil
	}
	ftsQuery := `"` + strings.ReplaceAll(query, `"`, `""`) + `"`
	var total int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT pc.id
			FROM problem_classes_fts
			JOIN problem_classes pc ON pc.id = problem_classes_fts.rowid
			WHERE problem_classes_fts MATCH ?
			  AND (? = '' OR EXISTS (
			      SELECT 1 FROM answer_nodes a
			      WHERE a.class_id = pc.id AND a.env = ?))
			  AND (? = '' OR EXISTS (
			      SELECT 1 FROM answer_nodes a
			      WHERE a.class_id = pc.id AND a.lang = ?))
			  AND (? = '' OR EXISTS (
			      SELECT 1 FROM answer_nodes a
			      WHERE a.class_id = pc.id AND (CASE WHEN ? = 'solved'
			                                         THEN a.status IN ('verified', 'ci_passed')
			                                         ELSE a.status = ? END)))
			UNION
			SELECT pc.id
			FROM answer_nodes_fts
			JOIN answer_nodes a ON a.id = answer_nodes_fts.rowid
			JOIN problem_classes pc ON pc.id = a.class_id
			WHERE answer_nodes_fts MATCH ?
			  AND (? = '' OR a.env = ?)
			  AND (? = '' OR a.lang = ?)
			  AND (? = '' OR (CASE WHEN ? = 'solved'
			                       THEN a.status IN ('verified', 'ci_passed')
			                       ELSE a.status = ? END))
		)
	`, ftsQuery, env, env, lang, lang, status, status, status,
		ftsQuery, env, env, lang, lang, status, status, status).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("search count: %w", err)
	}
	return total, nil
}
