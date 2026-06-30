package graph

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// Discovery is the central query that powers the /api/v1/problems/discover
// endpoint. Given a problem class title, it returns:
//
//  1. The exact-match answer (if any) for the (env, lang, version) tuple
//     requested. If env/lang/version are empty, the most recent verified
//     answer is returned.
//  2. The version history — parent_id chains walked from the exact match
//     back through superseded versions.
//  3. Lateral edges — related problem classes connected via problem_edges,
//     with relationship type and weight.
//
// The version history is computed by a recursive CTE that walks parent_id
// chains. Lateral edges are joined in a second query. The two results are
// stitched together in Go because SQLite doesn't support recursive CTEs
// that join across two distinct tree shapes in a single statement.
//
// env, lang, version may be empty. Empty values act as wildcards — the
// query prefers more specific matches (filled in) over general ones.
func (s *Store) Discovery(ctx context.Context, title, env, lang, version string, includeRelated bool) (*DiscoveryResult, error) {
	pc, err := s.GetProblemClassByTitle(ctx, title)
	if err != nil {
		return nil, err
	}
	res := &DiscoveryResult{Class: *pc}

	// 1. Exact match (or fallback to most recent verified for the class).
	best, err := s.bestAnswer(ctx, pc.ID, env, lang, version)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	if best != nil {
		res.Exact = best
		// 2. Walk parent_id chain for version history.
		versions, err := s.versionHistory(ctx, best.ID)
		if err != nil {
			return nil, fmt.Errorf("version history: %w", err)
		}
		res.Versions = versions
		// 3. Surface version warnings for tuples with known superseding
		// answers.
		warnings, err := s.versionWarnings(ctx, pc.ID, env, lang, version)
		if err != nil {
			return nil, fmt.Errorf("version warnings: %w", err)
		}
		res.VersionWarnings = warnings
	}

	if includeRelated {
		edges, err := s.relatedClasses(ctx, pc.ID)
		if err != nil {
			return nil, fmt.Errorf("related: %w", err)
		}
		res.Related = edges
	}

	return res, nil
}

// bestAnswer picks the best-matching answer for a problem class. We
// rank by (env, lang, version) specificity: a row matching all three
// beats one matching only env+lang, which beats one matching only env.
// Within each specificity tier, the most recent verified answer wins.
func (s *Store) bestAnswer(ctx context.Context, classID int64, env, lang, version string) (*AnswerNode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, class_id, parent_id, env, lang, version, solution, evidence, signatures, status, created_at
		FROM answer_nodes
		WHERE class_id = ?
		  AND (env = ? OR ? = '')
		  AND (lang = ? OR ? = '')
		  AND (version = ? OR ? = '')
		  AND status IN ('verified', 'ci_passed')
		ORDER BY
		  (CASE WHEN env = ? THEN 3 ELSE 0 END) +
		  (CASE WHEN lang = ? THEN 2 ELSE 0 END) +
		  (CASE WHEN version = ? THEN 1 ELSE 0 END) DESC,
		  id DESC
		LIMIT 1
	`, classID, env, env, lang, lang, version, version, env, lang, version)
	if err != nil {
		return nil, fmt.Errorf("bestAnswer query: %w", err)
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, ErrNotFound
	}
	return scanAnswerNodeRows(rows)
}

// versionHistory walks parent_id chains forward (newer) and backward
// (older) from the starting answer, returning the chain ordered from
// oldest to newest.
//
// We do this with a simple iterative loop rather than a recursive CTE.
// modernc.org/sqlite (sqlite compiled to Go) has shown quirks with
// recursive CTEs in some versions — a deterministic loop is easier to
// reason about and test. The chain is bounded by the version-history
// depth, which is typically <10 levels; an iterative walk is fine.
func (s *Store) versionHistory(ctx context.Context, startID int64) ([]AnswerNode, error) {
	// First, collect the chain as IDs from oldest to newest.
	var chain []int64
	current := startID
	const maxDepth = 1000 // hard cap to prevent runaway loops on bad data
	for i := 0; i < maxDepth; i++ {
		chain = append([]int64{current}, chain...) // prepend → oldest first
		var parent sql.NullInt64
		err := s.db.QueryRowContext(ctx,
			`SELECT parent_id FROM answer_nodes WHERE id = ?`, current).Scan(&parent)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			return nil, fmt.Errorf("versionHistory lookup: %w", err)
		}
		if !parent.Valid {
			break
		}
		current = parent.Int64
	}

	// Now fetch the full rows for each id.
	out := make([]AnswerNode, 0, len(chain))
	for _, id := range chain {
		a, err := s.GetAnswerNode(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("versionHistory fetch %d: %w", id, err)
		}
		out = append(out, *a)
	}
	return out, nil
}

// relatedClasses returns all edges originating from a problem class,
// joined with the target's title. Ordered by weight descending.
func (s *Store) relatedClasses(ctx context.Context, sourceID int64) ([]RelatedEdge, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT pc.title, e.relationship, e.weight
		FROM problem_edges e
		JOIN problem_classes pc ON pc.id = e.target_id
		WHERE e.source_id = ?
		ORDER BY e.weight DESC
	`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("relatedClasses query: %w", err)
	}
	defer rows.Close()
	var out []RelatedEdge
	for rows.Next() {
		var r RelatedEdge
		if err := rows.Scan(&r.TargetTitle, &r.Relationship, &r.Weight); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// versionWarnings inspects the problem's edges for `superseded_by`
// relationships and returns human-readable warnings when the requested
// (env, lang, version) tuple is older than the one in the edge target.
func (s *Store) versionWarnings(ctx context.Context, classID int64, env, lang, version string) ([]string, error) {
	if version == "" {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT pc.title, a.version
		FROM problem_edges e
		JOIN problem_classes pc ON pc.id = e.target_id
		LEFT JOIN answer_nodes a ON a.class_id = pc.id AND a.status IN ('verified', 'ci_passed')
		WHERE e.source_id = ? AND e.relationship = 'superseded_by'
		ORDER BY a.id DESC
	`, classID)
	if err != nil {
		return nil, fmt.Errorf("versionWarnings query: %w", err)
	}
	defer rows.Close()
	var out []string
	seen := map[string]bool{}
	for rows.Next() {
		var t, v string
		if err := rows.Scan(&t, &v); err != nil {
			return nil, err
		}
		if v == "" || v == version {
			continue
		}
		key := t + ":" + v
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, fmt.Sprintf("This solution is for %s; newer answers exist for %s (see %s).", version, v, t))
	}
	return out, rows.Err()
}

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
		      WHERE a.class_id = pc.id AND a.status = ?))
		UNION
		SELECT pc.id, pc.title, snippet(answer_nodes_fts, 0, '[', ']', '…', 8) AS snip,
		       rank, a.id AS answer_id
		FROM answer_nodes_fts
		JOIN answer_nodes a ON a.id = answer_nodes_fts.rowid
		JOIN problem_classes pc ON pc.id = a.class_id
		WHERE answer_nodes_fts MATCH ?
		  AND (? = '' OR a.env = ?)
		  AND (? = '' OR a.lang = ?)
		  AND (? = '' OR a.status = ?)
		ORDER BY rank
		LIMIT ? OFFSET ?
	`, ftsQuery, env, env, lang, lang, status, status,
		ftsQuery, env, env, lang, lang, status, status,
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
