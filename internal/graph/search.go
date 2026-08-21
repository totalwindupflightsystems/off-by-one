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
	//
	// A class can match BOTH branches (title mentions the term AND an
	// answer's solution does too) — the UNION rows differ in snippet/
	// rank/answer_id, so SQLite keeps both. We dedupe by class id with
	// ROW_NUMBER, preferring the class-level hit (answer_id NULL), so
	// pagination aligns with SearchCount (which counts distinct ids).
	//
	// The deduped hits are then joined back to problem_classes and the
	// answer aggregate so each hit carries the same per-class metadata as
	// ListProblemClassesWithCountsFiltered: description, created_at,
	// answer_count, and the derived best status (ci_passed > verified >
	// pending > failed, coalescing to 'pending'). That keeps the API
	// search path from doing N+1 lookups per hit (OB-GAP-050).
	rows, err := s.db.QueryContext(ctx, `
		SELECT hits.id, hits.title, hits.snip, hits.rank, hits.answer_id,
		       pc.description, pc.created_at,
		       COALESCE(ac.cnt, 0) AS answer_count,
		       COALESCE(ac.best_status, 'pending') AS status
		FROM (
			SELECT *, ROW_NUMBER() OVER (
				PARTITION BY id ORDER BY has_answer, rank
			) AS rn FROM (
				SELECT pc.id, pc.title, snippet(problem_classes_fts, 1, '[', ']', '…', 8) AS snip,
				       rank, NULL AS answer_id, 0 AS has_answer
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
				       rank, a.id AS answer_id, 1 AS has_answer
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
		) hits
		JOIN problem_classes pc ON pc.id = hits.id
		LEFT JOIN (
			SELECT class_id,
			       COUNT(*) AS cnt,
			       CASE
			           WHEN MAX(CASE WHEN status = 'ci_passed' THEN 1 ELSE 0 END) = 1 THEN 'ci_passed'
			           WHEN MAX(CASE WHEN status = 'verified' THEN 1 ELSE 0 END) = 1 THEN 'verified'
			           WHEN MAX(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) = 1 THEN 'pending'
			           ELSE 'failed'
			       END AS best_status
			FROM answer_nodes
			GROUP BY class_id
		) ac ON ac.class_id = hits.id
		WHERE hits.rn = 1
		ORDER BY hits.rank
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
		var created string
		if err := rows.Scan(&hit.ClassID, &hit.Title, &hit.Snippet, &hit.Score, &hit.AnswerID,
			&hit.Description, &created, &hit.AnswerCount, &hit.Status); err != nil {
			return nil, fmt.Errorf("scan search hit: %w", err)
		}
		hit.CreatedAt = parseSQLiteTime(created)
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
