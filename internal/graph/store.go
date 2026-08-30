// Package graph implements the problem-class tree and lateral graph edges
// that power Off-by-One's discovery engine. SQLite stores the data; the
// recursive CTE in Discovery() walks the version history of a single
// problem class, and the FTS5 virtual tables back full-text search.
package graph

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	schemasql "github.com/totalwindupflightsystems/off-by-one/sql/schema"
)

// Edge types match the relationship column in problem_edges. They are the
// canonical values Muster and the web UI surface.
const (
	EdgeSameRootCause = "same_root_cause"
	EdgePrerequisite  = "prerequisite"
	EdgeSupersededBy  = "superseded_by"
	EdgeGeneralizes   = "generalizes"
)

// Status values match the status column in answer_nodes.
const (
	AnswerPending  = "pending"
	AnswerVerified = "verified"
	AnswerFailed   = "failed"
	AnswerCIPassed = "ci_passed"
)

// Store is a SQLite-backed problem-class graph. It is safe for concurrent
// use — the underlying *sql.DB enforces connection-level serialisation, and
// we set WAL mode + busy_timeout in Open for predictable concurrent reads.
type Store struct {
	db *sql.DB
}

// ProblemClass is a top-level category of debugging problem.
type ProblemClass struct {
	ID          int64
	Title       string
	Description string
	CreatedAt   time.Time
}

// AnswerNode is a single verified answer (or candidate answer) for a
// problem class. The (class, env, lang, version) tuple scopes the answer;
// parent_id points to a predecessor version of the same answer.
type AnswerNode struct {
	ID         int64
	ClassID    int64
	ParentID   sql.NullInt64
	Env        string
	Lang       string
	Version    string
	Solution   string
	Evidence   string
	Signatures string
	Status     string
	CreatedAt  time.Time
}

// Edge is a typed relationship between two problem classes.
type Edge struct {
	ID           int64
	SourceID     int64
	TargetID     int64
	Relationship string
	Weight       float64
	CreatedAt    time.Time
}

// DiscoveryResult is the structured output of the BFS query at the heart
// of /api/v1/problems/discover.
type DiscoveryResult struct {
	Class           ProblemClass
	Exact           *AnswerNode
	Versions        []AnswerNode
	Related         []RelatedEdge
	VersionWarnings []string
}

// RelatedEdge joins a problem class to a related class with a relationship
// type and weight.
type RelatedEdge struct {
	TargetTitle  string
	Relationship string
	Weight       float64
}

// SearchHit is one row of full-text search output, with a snippet of the
// matched content and a relevance score. Description, CreatedAt, AnswerCount
// and Status carry the same per-class metadata the plain list endpoint
// returns (status is the derived best answer status), so FTS search results
// are interchangeable with list rows (OB-GAP-050).
type SearchHit struct {
	ClassID     int64
	Title       string
	Snippet     string
	Score       float64
	AnswerID    sql.NullInt64
	Description string
	CreatedAt   time.Time
	AnswerCount int
	Status      string
}

// Open creates a Store backed by the SQLite database at dbPath. The schema
// is created on first call and migrations are applied.
//
// Set dbPath=":memory:" for tests. For in-memory mode, modernc.org/sqlite
// uses a per-connection database by default, which breaks the
// "connection pool shares one DB" assumption. We use a shared-cache
// DSN so all connections in the pool see the same data.
//
// Note: ":memory:" with `cache=shared` is shared across connections
// within the same process but the cache name must be unique per
// database. We use the PID + a counter to make the name unique. To
// force per-test isolation, callers should use OpenShared() with their
// own name, or use a temp-file path via t.TempDir() + "/test.db".
func Open(dbPath string) (*Store, error) {
	return openDSN(dbPathToDSN(dbPath))
}

// OpenShared creates a Store backed by a named in-memory database that
// is shared across all connections in the process. The name must be
// unique per test — pass t.Name() or a fresh UUID.
func OpenShared(name string) (*Store, error) {
	return openDSN("file:" + name + "?mode=memory&cache=shared&_pragma=foreign_keys(ON)")
}

func dbPathToDSN(dbPath string) string {
	if dbPath == ":memory:" {
		return "file:off-by-one?mode=memory&cache=shared&_pragma=foreign_keys(ON)"
	}
	if !strings.Contains(dbPath, "?") {
		return dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	}
	return dbPath
}

func openDSN(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	// PRAGMA busy_timeout (in ms) controls how long SQLite waits for a
	// write lock before returning SQLITE_BUSY. The DSN _pragma syntax
	// doesn't always apply to in-memory shared caches, so we set it
	// explicitly per-connection.
	if _, err := db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set busy_timeout: %w", err)
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close releases the database connection.
func (s *Store) Close() error { return s.db.Close() }

// DB returns the underlying *sql.DB. Useful for tests and ad-hoc queries
// that don't fit the high-level API. Don't use this for production code
// paths — go through the typed methods so error handling and validation
// stay consistent.
func (s *Store) DB() *sql.DB { return s.db }

// migrate applies the embedded schema and creates the FTS5 virtual
// tables. Idempotent — running on an existing DB is a no-op.
func (s *Store) migrate() error {
	if _, err := s.db.Exec(schemasql.Schema); err != nil {
		return fmt.Errorf("apply schema.sql: %w", err)
	}
	if _, err := s.db.Exec(schemasql.FTS5Extra); err != nil {
		return fmt.Errorf("create fts5: %w", err)
	}
	return nil
}

// ApplyExtra runs additional SQL on the underlying connection. Used by
// other packages (e.g., the ingest queue) to add their own tables to the
// same database. The caller is responsible for idempotency (use IF NOT
// EXISTS) and ordering (call this AFTER Open returns).
func (s *Store) ApplyExtra(sql string) error {
	if _, err := s.db.Exec(sql); err != nil {
		return fmt.Errorf("apply extra schema: %w", err)
	}
	return nil
}

// CreateProblemClass inserts a new problem class. The (title) column has a
// UNIQUE constraint, so calling with a duplicate title returns ErrDuplicate.
//
// Returns the new ID.
func (s *Store) CreateProblemClass(ctx context.Context, title, description string) (int64, error) {
	if title == "" {
		return 0, errors.New("title must not be empty")
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO problem_classes (title, description) VALUES (?, ?)`,
		title, description)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrDuplicate
		}
		return 0, fmt.Errorf("insert problem_class: %w", err)
	}
	return res.LastInsertId()
}

// ErrDuplicate signals a unique-constraint violation. The most common
// case is re-creating a problem class with an existing title.
var ErrDuplicate = errors.New("graph: duplicate")

// isUniqueViolation reports whether err is a SQLite UNIQUE constraint
// failure. modernc.org/sqlite exposes error code 2067 in the message.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "2067") || strings.Contains(msg, "UNIQUE constraint failed")
}

// GetProblemClass fetches a single problem class by ID. Returns
// ErrNotFound if no row matches.
func (s *Store) GetProblemClass(ctx context.Context, id int64) (*ProblemClass, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, created_at FROM problem_classes WHERE id = ?`, id)
	return scanProblemClass(row)
}

// GetProblemClassByTitle is the lookup the discovery engine and the web UI
// autocomplete use — titles are slugified identifiers and unique.
func (s *Store) GetProblemClassByTitle(ctx context.Context, title string) (*ProblemClass, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, title, description, created_at FROM problem_classes WHERE title = ?`, title)
	return scanProblemClass(row)
}

// UpsertProblemClass finds the class by title, returning it if found, or
// creating it if not. Returns the class and a boolean indicating whether
// it was just created.
func (s *Store) UpsertProblemClass(ctx context.Context, title, description string) (*ProblemClass, bool, error) {
	if existing, err := s.GetProblemClassByTitle(ctx, title); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, ErrNotFound) {
		return nil, false, err
	}
	id, err := s.CreateProblemClass(ctx, title, description)
	if err != nil {
		return nil, false, err
	}
	pc, err := s.GetProblemClass(ctx, id)
	if err != nil {
		return nil, false, err
	}
	return pc, true, nil
}

// ListProblemClasses returns problem classes ordered by ID with optional
// limit and offset for pagination.
func (s *Store) ListProblemClasses(ctx context.Context, limit, offset int) ([]ProblemClass, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, title, description, created_at FROM problem_classes
		 ORDER BY id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list problem_classes: %w", err)
	}
	defer rows.Close()
	var out []ProblemClass
	for rows.Next() {
		pc, err := scanProblemClassRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *pc)
	}
	return out, rows.Err()
}

// ErrNotFound is returned by lookups when no row matches.
var ErrNotFound = errors.New("graph: not found")

// CreateAnswerNode inserts a new answer. parentID may be 0 for no parent.
//
// The signatures column is a JSON blob — pass an empty string for
// "no signatures" or a json.Marshal'd object.
func (s *Store) CreateAnswerNode(ctx context.Context, classID int64, parentID int64, env, lang, version, solution, evidence, signatures string) (int64, error) {
	if classID == 0 {
		return 0, errors.New("class_id must not be zero")
	}
	if signatures == "" {
		signatures = "{}"
	}
	if signatures != "{}" && !json.Valid([]byte(signatures)) {
		return 0, fmt.Errorf("signatures must be valid JSON, got %q", signatures)
	}
	var parent sql.NullInt64
	if parentID > 0 {
		parent = sql.NullInt64{Int64: parentID, Valid: true}
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO answer_nodes (class_id, parent_id, env, lang, version, solution, evidence, signatures, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		classID, parent, env, lang, version, solution, evidence, signatures, AnswerPending)
	if err != nil {
		return 0, fmt.Errorf("insert answer_node: %w", err)
	}
	return res.LastInsertId()
}

// GetAnswerNode fetches a single answer by ID.
func (s *Store) GetAnswerNode(ctx context.Context, id int64) (*AnswerNode, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, class_id, parent_id, env, lang, version, solution, evidence, signatures, status, created_at
		 FROM answer_nodes WHERE id = ?`, id)
	return scanAnswerNode(row)
}

// ListAnswers returns all answers for a problem class, newest first.
func (s *Store) ListAnswers(ctx context.Context, classID int64) ([]AnswerNode, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, class_id, parent_id, env, lang, version, solution, evidence, signatures, status, created_at
		 FROM answer_nodes WHERE class_id = ? ORDER BY id DESC`, classID)
	if err != nil {
		return nil, fmt.Errorf("list answer_nodes: %w", err)
	}
	defer rows.Close()
	var out []AnswerNode
	for rows.Next() {
		a, err := scanAnswerNodeRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// UpdateAnswerStatus changes an answer's status. Used by the solver to
// mark answers as verified or failed.
func (s *Store) UpdateAnswerStatus(ctx context.Context, id int64, status string) error {
	switch status {
	case AnswerPending, AnswerVerified, AnswerFailed, AnswerCIPassed:
	default:
		return fmt.Errorf("invalid status %q", status)
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE answer_nodes SET status = ? WHERE id = ?`, status, id)
	if err != nil {
		return fmt.Errorf("update answer status: %w", err)
	}
	return nil
}

// CreateEdge creates a directed edge between two problem classes. The
// (source, target, relationship) tuple is unique, so a duplicate insert
// returns ErrDuplicate.
func (s *Store) CreateEdge(ctx context.Context, sourceID, targetID int64, relationship string, weight float64) (int64, error) {
	if sourceID == 0 || targetID == 0 {
		return 0, errors.New("source_id and target_id must be non-zero")
	}
	if sourceID == targetID {
		return 0, errors.New("self-edges are not allowed")
	}
	switch relationship {
	case EdgeSameRootCause, EdgePrerequisite, EdgeSupersededBy, EdgeGeneralizes:
	default:
		return 0, fmt.Errorf("invalid relationship %q", relationship)
	}
	if weight <= 0 {
		weight = 1.0
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO problem_edges (source_id, target_id, relationship, weight) VALUES (?, ?, ?, ?)`,
		sourceID, targetID, relationship, weight)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrDuplicate
		}
		return 0, fmt.Errorf("insert problem_edge: %w", err)
	}
	return res.LastInsertId()
}

// ListEdgesFrom returns all edges originating from a problem class.
func (s *Store) ListEdgesFrom(ctx context.Context, sourceID int64) ([]Edge, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, source_id, target_id, relationship, weight, created_at
		 FROM problem_edges WHERE source_id = ? ORDER BY weight DESC`, sourceID)
	if err != nil {
		return nil, fmt.Errorf("list edges: %w", err)
	}
	defer rows.Close()
	var out []Edge
	for rows.Next() {
		e, err := scanEdgeRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// RelatedTitles returns the titles of problem classes connected to
// sourceID via any problem_edge. Used by the API to populate the
// SubmitProblemResponse.related_problems list. Deduplicates by title
// and caps at 20 entries.
func (s *Store) RelatedTitles(ctx context.Context, sourceID int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT pc.title
		FROM problem_edges e
		JOIN problem_classes pc ON pc.id = e.target_id
		WHERE e.source_id = ?
		UNION
		SELECT DISTINCT pc.title
		FROM problem_edges e
		JOIN problem_classes pc ON pc.id = e.source_id
		WHERE e.target_id = ?
		LIMIT 20
	`, sourceID, sourceID)
	if err != nil {
		return nil, fmt.Errorf("related titles: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var title string
		if err := rows.Scan(&title); err != nil {
			return nil, err
		}
		out = append(out, title)
	}
	return out, rows.Err()
}

// Stats is the per-system counter snapshot the /api/v1/stats endpoint
// returns. Fields map 1:1 to the Stats schema in the OpenAPI spec.
type Stats struct {
	TotalProblems   int     `json:"total_problems"`
	TotalAnswers    int     `json:"total_answers"`
	VerifiedAnswers int     `json:"verified_answers"`
	QueueDepth      int     `json:"queue_depth"`
	HitRate         float64 `json:"hit_rate"`
	Coverage        float64 `json:"coverage"`
	AvgSolveTime    string  `json:"avg_solve_time"`
}

// Stats returns aggregate counters across the graph. Hit rate is
// computed as verified_answers / total_answers (when total > 0).
// Coverage is verified / total problems. Avg solve time is left empty
// here — the API layer computes it from queue_entries solve timings
// (see Queue.AvgSolveTime) and fills the field in handleStats.
//
// verified_answers excludes answer_nodes whose signatures JSON carries
// result='failed': those rows are status-verified but their solve
// signature reports failure, so counting them would over-report the
// cache's hit rate. Status remains the primary signal — 'passed',
// 'completed', and absent or malformed signature JSON still count.
func (s *Store) Stats(ctx context.Context) (*Stats, error) {
	var st Stats
	row := s.db.QueryRowContext(ctx, `
		SELECT
			(SELECT COUNT(*) FROM problem_classes),
			(SELECT COUNT(*) FROM answer_nodes),
			(SELECT COUNT(*) FROM answer_nodes WHERE status IN ('verified', 'ci_passed') AND COALESCE(json_extract(signatures, '$.result'), '') != 'failed')
	`)
	if err := row.Scan(&st.TotalProblems, &st.TotalAnswers, &st.VerifiedAnswers); err != nil {
		return nil, fmt.Errorf("stats: %w", err)
	}
	if st.TotalAnswers > 0 {
		st.HitRate = float64(st.VerifiedAnswers) / float64(st.TotalAnswers)
	}
	if st.TotalProblems > 0 {
		st.Coverage = float64(st.VerifiedAnswers) / float64(st.TotalProblems)
	}
	return &st, nil
}

// AnswerCount returns the number of answers associated with a problem
// class. Used by ProblemClass.AnswerCount after a List to populate the
// answer_count field the OpenAPI spec requires.
func (s *Store) AnswerCount(ctx context.Context, classID int64) (int, error) {
	var n int
	row := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM answer_nodes WHERE class_id = ?`, classID)
	if err := row.Scan(&n); err != nil {
		return 0, fmt.Errorf("answer count: %w", err)
	}
	return n, nil
}

// GetProblemClassStatus returns the derived best answer status for one
// problem class, using the same precedence as
// ListProblemClassesWithCountsFiltered: ci_passed > verified > pending >
// failed, coalescing to 'pending' when the class has no answers. The
// detail endpoint (/api/v1/problems/{class}) uses it so its status
// matches the list view (OB-GAP-024).
func (s *Store) GetProblemClassStatus(ctx context.Context, classID int64) (string, error) {
	var status string
	row := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(
			(SELECT CASE
			        WHEN MAX(CASE WHEN status = 'ci_passed' THEN 1 ELSE 0 END) = 1 THEN 'ci_passed'
			        WHEN MAX(CASE WHEN status = 'verified' THEN 1 ELSE 0 END) = 1 THEN 'verified'
			        WHEN MAX(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) = 1 THEN 'pending'
			        ELSE 'failed'
			    END
			 FROM answer_nodes
			 WHERE class_id = ?
			 GROUP BY class_id),
			'pending')
	`, classID)
	if err := row.Scan(&status); err != nil {
		return "", fmt.Errorf("problem class status: %w", err)
	}
	return status, nil
}

// ProblemClassWithCounts is the ProblemClass plus derived fields the
// /api/v1/problems endpoint needs: answer_count, status (best answer's
// status), hit_count, last_hit.
type ProblemClassWithCounts struct {
	ProblemClass
	AnswerCount int    `json:"answer_count"`
	Status      string `json:"status"`
	HitCount    int    `json:"hit_count"`
	LastHit     string `json:"last_hit,omitempty"`
}

// ListProblemClassesWithCounts returns problem classes joined with
// derived counts. status is the highest-precedence answer status
// (ci_passed > verified > pending > failed). hit_count and last_hit
// are placeholder zeros for now — the cron loop will populate them
// once the discovery endpoint begins logging hits.
func (s *Store) ListProblemClassesWithCounts(ctx context.Context, limit, offset int) ([]ProblemClassWithCounts, error) {
	return s.ListProblemClassesWithCountsFiltered(ctx, "", limit, offset)
}

// ListProblemClassesWithCountsFiltered is ListProblemClassesWithCounts with
// an explicit derived-status filter applied in SQL. Filtering in SQL (not in
// the caller) keeps LIMIT/OFFSET pagination correct. status may be one of
// ci_passed, verified, pending, failed, or the UI alias "solved" (matches
// verified OR ci_passed). Empty = no filter.
func (s *Store) ListProblemClassesWithCountsFiltered(ctx context.Context, status string, limit, offset int) ([]ProblemClassWithCounts, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT pc.id, pc.title, pc.description, pc.created_at,
		       COALESCE(ac.cnt, 0) AS answer_count,
		       COALESCE(ac.best_status, 'pending') AS status
		FROM problem_classes pc
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
		) ac ON ac.class_id = pc.id
		WHERE ? = '' OR (CASE WHEN ? = 'solved'
		                      THEN COALESCE(ac.best_status, 'pending') IN ('verified', 'ci_passed')
		                      ELSE COALESCE(ac.best_status, 'pending') = ? END)
		ORDER BY pc.id DESC
		LIMIT ? OFFSET ?
	`, status, status, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list problem_classes with counts: %w", err)
	}
	defer rows.Close()
	var out []ProblemClassWithCounts
	for rows.Next() {
		var p ProblemClassWithCounts
		var created string
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &created, &p.AnswerCount, &p.Status); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		p.CreatedAt = parseSQLiteTime(created)
		out = append(out, p)
	}
	return out, rows.Err()
}

// CountProblemClasses returns the number of problem classes, optionally
// filtered by derived best answer status (same semantics as
// ListProblemClassesWithCountsFiltered). Used for accurate pagination
// totals on /api/v1/problems.
func (s *Store) CountProblemClasses(ctx context.Context, status string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM problem_classes pc
		LEFT JOIN (
			SELECT class_id,
			       CASE
			           WHEN MAX(CASE WHEN status = 'ci_passed' THEN 1 ELSE 0 END) = 1 THEN 'ci_passed'
			           WHEN MAX(CASE WHEN status = 'verified' THEN 1 ELSE 0 END) = 1 THEN 'verified'
			           WHEN MAX(CASE WHEN status = 'pending' THEN 1 ELSE 0 END) = 1 THEN 'pending'
			           ELSE 'failed'
			       END AS best_status
			FROM answer_nodes
			GROUP BY class_id
		) ac ON ac.class_id = pc.id
		WHERE ? = '' OR (CASE WHEN ? = 'solved'
		                      THEN COALESCE(ac.best_status, 'pending') IN ('verified', 'ci_passed')
		                      ELSE COALESCE(ac.best_status, 'pending') = ? END)
	`, status, status, status).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count problem_classes: %w", err)
	}
	return n, nil
}

// --- Scanning helpers -----------------------------------------------------

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProblemClass(row rowScanner) (*ProblemClass, error) {
	var pc ProblemClass
	var created string
	if err := row.Scan(&pc.ID, &pc.Title, &pc.Description, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan problem_class: %w", err)
	}
	pc.CreatedAt = parseSQLiteTime(created)
	return &pc, nil
}

func scanProblemClassRows(rows *sql.Rows) (*ProblemClass, error) {
	return scanProblemClass(rows)
}

func scanAnswerNode(row rowScanner) (*AnswerNode, error) {
	var a AnswerNode
	var parent sql.NullInt64
	var created string
	if err := row.Scan(&a.ID, &a.ClassID, &parent, &a.Env, &a.Lang, &a.Version, &a.Solution, &a.Evidence, &a.Signatures, &a.Status, &created); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan answer_node: %w", err)
	}
	if parent.Valid {
		a.ParentID = parent
	}
	a.CreatedAt = parseSQLiteTime(created)
	return &a, nil
}

func scanAnswerNodeRows(rows *sql.Rows) (*AnswerNode, error) {
	return scanAnswerNode(rows)
}

func scanEdgeRows(rows *sql.Rows) (*Edge, error) {
	var e Edge
	var created string
	if err := rows.Scan(&e.ID, &e.SourceID, &e.TargetID, &e.Relationship, &e.Weight, &created); err != nil {
		return nil, fmt.Errorf("scan edge: %w", err)
	}
	e.CreatedAt = parseSQLiteTime(created)
	return &e, nil
}

// parseSQLiteTime parses the default SQLite CURRENT_TIMESTAMP format
// (YYYY-MM-DD HH:MM:SS) and the more verbose formats modernc.org/sqlite
// can produce. Returns the zero time on parse failure rather than
// erroring — created_at is informational and should not block inserts.
func parseSQLiteTime(s string) time.Time {
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// NewID returns a short, human-readable identifier for submissions and
// answers. Format: prefix_<6 hex chars>. We use crypto/rand for
// uniqueness — short IDs are fine because we only need to disambiguate
// within a single user's session.
func NewID(prefix string) string {
	var b [3]byte
	_, _ = rand.Read(b[:])
	return prefix + "_" + hex.EncodeToString(b[:])
}
