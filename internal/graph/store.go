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
// matched content and a relevance score.
type SearchHit struct {
	ClassID  int64
	Title    string
	Snippet  string
	Score    float64
	AnswerID sql.NullInt64
}

// Open creates a Store backed by the SQLite database at dbPath. The schema
// is created on first call and migrations are applied.
//
// Set dbPath=":memory:" for tests. WAL mode is enabled for on-disk DBs to
// support concurrent readers alongside a single writer.
func Open(dbPath string) (*Store, error) {
	dsn := dbPath
	if dbPath != ":memory:" && !strings.Contains(dsn, "?") {
		dsn = dbPath + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)"
	} else if dbPath == ":memory:" {
		dsn = "file::memory:?cache=shared&_pragma=foreign_keys(ON)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
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
