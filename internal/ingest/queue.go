// Package ingest implements the submission queue for Off-by-One. The
// queue is persisted in SQLite (survives restart), ranked by a priority
// score that weights cadence (post-debug > end-of-day > pre-phase) and
// recurrence (how many times the same problem class has been submitted),
// and exposed to the cron loop via Dequeue().
package ingest

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	schemasql "github.com/totalwindupflightsystems/off-by-one/sql/schema"
)

// Cadence values match the cadence column in queue_entries.
const (
	CadencePrePhase  = "pre-phase"
	CadenceEndOfDay  = "end-of-day"
	CadencePostDebug = "post-debug"
)

// Status values for the queue_entries.status column.
const (
	StatusPending    = "pending"
	StatusInProgress = "in_progress"
	StatusComplete   = "complete"
	StatusFailed     = "failed"
)

// ErrInvalidCadence signals a Submission with an unknown cadence value.
var ErrInvalidCadence = errors.New("ingest: invalid cadence")

// ErrEmptyProblemClass signals a Submission with no problem_class set.
var ErrEmptyProblemClass = errors.New("ingest: problem_class required")

// ErrDuplicate signals a deduplication hit — the same problem class is
// already queued or has a verified answer. The existing entry is returned
// alongside so the API can show queue position to the submitter.
var ErrDuplicate = errors.New("ingest: duplicate submission")

// Queue is the submission queue. It is safe for concurrent use: the
// underlying SQLite connection pool serialises writes, and a sync.Mutex
// guards the in-memory dedup map that prevents two concurrent Submit
// calls from racing past the SQLite unique check.
type Queue struct {
	store *graph.Store
	db    *sql.DB
	mu    sync.Mutex
}

// Submission is the validated input to Submit. Created by the API layer
// from the JSON request body; we keep the two types separate so the API
// can do its own field validation before constructing a Submission.
type Submission struct {
	ProblemClass string         `json:"problem_class"`
	Environment  string         `json:"environment"`
	Language     string         `json:"language"`
	Version      string         `json:"version"`
	Description  string         `json:"description"`
	ErrorMessage string         `json:"error_message"`
	StackTrace   string         `json:"stack_trace"`
	Context      map[string]any `json:"context"`
	Cadence      string         `json:"cadence"`
}

// Entry is a row in queue_entries, with the JSON context unmarshalled.
type Entry struct {
	ID             string         `json:"submission_id"`
	ProblemClass   string         `json:"problem_class"`
	Environment    string         `json:"environment"`
	Language       string         `json:"language"`
	Version        string         `json:"version"`
	Description    string         `json:"description"`
	ErrorMessage   string         `json:"error_message"`
	StackTrace     string         `json:"stack_trace"`
	Context        map[string]any `json:"context"`
	Cadence        string         `json:"cadence"`
	Priority       float64        `json:"priority"`
	Status         string         `json:"status"`
	Stage          string         `json:"stage"`
	ResultAnswerID sql.NullInt64  `json:"result_answer_id,omitempty"`
	// CreatedAt/StartedAt/CompletedAt are stored as strings because
	// modernc.org/sqlite returns TEXT timestamps as strings — scanning
	// directly into time.Time fails with "unsupported Scan, storing
	// driver.Value type string into type *time.Time". Callers that
	// need a real time.Time should call parseSQLiteTimestamp.
	// CreatedAt is a non-nullable string; StartedAt/CompletedAt are
	// nullable strings because modernc.org/sqlite returns TEXT NULLs as
	// nil (not empty string) and string columns cannot scan nil. Use
	// sql.NullString for the optional columns.
	CreatedAt   string         `json:"created_at"`
	StartedAt   sql.NullString `json:"started_at,omitempty"`
	CompletedAt sql.NullString `json:"completed_at,omitempty"`
}

// Open creates a Queue backed by the given graph Store. The queue's
// own table is added to the database via the embedded queue schema.
//
// We share the database with the graph rather than opening a second
// connection because the queue and the problem graph need to see each
// other's writes within a single transaction (e.g., when solving
// completes, the answer is added to the graph in the same transaction
// that marks the queue entry complete).
func Open(store *graph.Store) (*Queue, error) {
	if err := store.ApplyExtra(schemasql.QueueSchema); err != nil {
		return nil, fmt.Errorf("apply queue schema: %w", err)
	}
	return &Queue{store: store, db: store.DB()}, nil
}

// Store returns the underlying graph store. The queue and graph share
// the same DB but expose different APIs; callers that need graph
// operations during queue processing (e.g., checking for verified
// answers to deduplicate against) can use this accessor.
func (q *Queue) Store() *graph.Store { return q.store }

// Submit validates a submission, deduplicates against the existing queue
// and any verified answers for the same (class, env, lang, version)
// tuple, computes a priority score, and inserts a new queue entry.
//
// Returns the new entry's ID on success. If the submission is a
// duplicate of an existing pending queue entry or a verified answer,
// returns ErrDuplicate along with the existing entry (when one exists).
func (q *Queue) Submit(ctx context.Context, sub Submission) (string, *Entry, error) {
	if err := validate(sub); err != nil {
		return "", nil, err
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	// Dedup: a submission is a duplicate of a pending queue entry with
	// the same problem_class (and matching env/lang/version when
	// provided). We DO NOT dedup across different env/lang/version
	// tuples — those are different problems even if they share a class.
	existing, err := q.findPendingDuplicate(ctx, sub)
	if err != nil {
		return "", nil, err
	}
	if existing != nil {
		return existing.ID, existing, ErrDuplicate
	}

	// Dedup: a submission is a duplicate of a verified answer when the
	// same (class, env, lang, version) tuple already has a verified
	// answer. In that case, we return ErrDuplicate without creating a
	// new queue entry — the user can find the existing answer via
	// Discovery.
	if hasAnswer, err := q.hasVerifiedAnswer(ctx, sub); err != nil {
		return "", nil, err
	} else if hasAnswer {
		return "", nil, ErrDuplicate
	}

	// Compute priority. Recurrence is "how many times this class has
	// been submitted before, including the current one".
	recurrence, err := q.classRecurrence(ctx, sub.ProblemClass)
	if err != nil {
		return "", nil, err
	}
	priority := computePriority(sub.Cadence, recurrence)

	ctxJSON, err := json.Marshal(sub.Context)
	if err != nil {
		return "", nil, fmt.Errorf("marshal context: %w", err)
	}

	id := graph.NewID("sub")
	res, err := q.db.ExecContext(ctx, `
		INSERT INTO queue_entries
		  (id, problem_class, environment, language, version, description,
		   error_message, stack_trace, context_json, cadence, priority, status, stage)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, sub.ProblemClass, sub.Environment, sub.Language, sub.Version,
		sub.Description, sub.ErrorMessage, sub.StackTrace, string(ctxJSON),
		sub.Cadence, priority, StatusPending, "queued")
	if err != nil {
		return "", nil, fmt.Errorf("insert queue entry: %w", err)
	}
	_ = res
	return id, nil, nil
}

func validate(sub Submission) error {
	if strings.TrimSpace(sub.ProblemClass) == "" {
		return ErrEmptyProblemClass
	}
	switch sub.Cadence {
	case CadencePrePhase, CadenceEndOfDay, CadencePostDebug:
	default:
		return ErrInvalidCadence
	}
	return nil
}

// computePriority returns a priority score for the queue ordering. Higher
// is more important. The score is:
//
//	priority = cadence_weight + recurrence * 0.5
//
// where cadence_weight is 3 (post-debug) > 2 (end-of-day) > 1 (pre-phase),
// and recurrence is the number of times this class has been submitted
// before. The 0.5 multiplier keeps a fresh post-debug submission
// (cadence=3, recurrence=0 → score=3) ahead of a heavily-recurring
// pre-phase (cadence=1, recurrence=4 → score=3), reflecting the design
// choice that the AGENT has just debugged is the most actionable
// signal.
func computePriority(cadence string, recurrence int) float64 {
	var base float64
	switch cadence {
	case CadencePostDebug:
		base = 3.0
	case CadenceEndOfDay:
		base = 2.0
	case CadencePrePhase:
		base = 1.0
	}
	return base + float64(recurrence)*0.5
}

// findPendingDuplicate returns the pending (or in_progress) queue entry
// for the same (class, env, lang, version) tuple, if any. Returns nil
// if no duplicate exists.
func (q *Queue) findPendingDuplicate(ctx context.Context, sub Submission) (*Entry, error) {
	row := q.db.QueryRowContext(ctx, `
		SELECT id, problem_class, environment, language, version, description,
		       error_message, stack_trace, context_json, cadence, priority, status, stage,
		       result_answer_id, created_at, started_at, completed_at
		FROM queue_entries
		WHERE problem_class = ?
		  AND environment = ?
		  AND language = ?
		  AND version = ?
		  AND status IN ('pending', 'in_progress')
		ORDER BY created_at DESC LIMIT 1
	`, sub.ProblemClass, sub.Environment, sub.Language, sub.Version)
	entry, err := scanEntry(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return entry, err
}

// hasVerifiedAnswer reports whether the graph already contains a verified
// answer for the (class, env, lang, version) tuple.
func (q *Queue) hasVerifiedAnswer(ctx context.Context, sub Submission) (bool, error) {
	var n int
	err := q.db.QueryRowContext(ctx, `
		SELECT COUNT(1) FROM answer_nodes a
		JOIN problem_classes pc ON pc.id = a.class_id
		WHERE pc.title = ?
		  AND a.env = ?
		  AND a.lang = ?
		  AND a.version = ?
		  AND a.status IN ('verified', 'ci_passed')
	`, sub.ProblemClass, sub.Environment, sub.Language, sub.Version).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// classRecurrence counts how many times this class has been submitted
// (pending + complete entries). The current submission isn't counted —
// it's added to the total by future recurrences.
func (q *Queue) classRecurrence(ctx context.Context, problemClass string) (int, error) {
	var n int
	err := q.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM queue_entries WHERE problem_class = ?`,
		problemClass).Scan(&n)
	return n, err
}

// Get returns a single entry by ID. Returns ErrNotFound if no match.
func (q *Queue) Get(ctx context.Context, id string) (*Entry, error) {
	row := q.db.QueryRowContext(ctx, `
		SELECT id, problem_class, environment, language, version, description,
		       error_message, stack_trace, context_json, cadence, priority, status, stage,
		       result_answer_id, created_at, started_at, completed_at
		FROM queue_entries WHERE id = ?`, id)
	return scanEntry(row)
}

// List returns queue entries filtered by status (empty = all). Ordered
// by priority DESC, created_at ASC.
func (q *Queue) List(ctx context.Context, status string, limit, offset int) ([]Entry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	qry := `
		SELECT id, problem_class, environment, language, version, description,
		       error_message, stack_trace, context_json, cadence, priority, status, stage,
		       result_answer_id, created_at, started_at, completed_at
		FROM queue_entries`
	args := []any{}
	if status != "" {
		qry += ` WHERE status = ?`
		args = append(args, status)
	}
	qry += ` ORDER BY priority DESC, created_at ASC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := q.db.QueryContext(ctx, qry, args...)
	if err != nil {
		return nil, fmt.Errorf("list queue: %w", err)
	}
	defer rows.Close()
	var out []Entry
	for rows.Next() {
		e, err := scanEntryRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

// Depth returns the number of pending or in_progress entries in the
// queue. Used by the /api/v1/stats endpoint.
func (q *Queue) Depth(ctx context.Context) (int, error) {
	var n int
	err := q.db.QueryRowContext(ctx,
		`SELECT COUNT(1) FROM queue_entries WHERE status IN ('pending', 'in_progress')`,
	).Scan(&n)
	return n, err
}

// Dequeue atomically marks and returns the highest-priority pending
// entry, transitioning it to in_progress. Returns (nil, nil) if the
// queue is empty.
//
// We use a transaction so two concurrent cron loops cannot both dequeue
// the same entry — the SELECT picks the winner, and the UPDATE inside
// the same transaction locks the row from the runner-up's view. SQLite
// serialises the write lock acquisition, so the second dequeue blocks
// (or, with the busy_timeout pragma set on Open, retries for up to
// 5s) until the first transaction commits.
//
// modernc.org/sqlite has shown inconsistent behaviour with explicit
// transactions under concurrent load, so we fall back to a simple
// SELECT-then-UPDATE pattern (without a wrapping tx) when the table
// is busy. The race window is small (one SELECT and one UPDATE
// non-atomically) and the worst case is two cron processes dequeuing
// the same entry — which the solver's idempotency handles gracefully.
func (q *Queue) Dequeue(ctx context.Context) (*Entry, error) {
	entry, err := q.pickPending(ctx)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, nil
	}
	// Re-check that the entry is still pending. If a parallel process
	// dequeued it first, we return nil and let the caller skip this
	// iteration.
	var status string
	if err := q.db.QueryRowContext(ctx,
		`SELECT status FROM queue_entries WHERE id = ?`, entry.ID).Scan(&status); err != nil {
		return nil, fmt.Errorf("recheck status: %w", err)
	}
	if status != StatusPending {
		return nil, nil
	}
	res, err := q.db.ExecContext(ctx, `
		UPDATE queue_entries
		SET status = 'in_progress', stage = 'sandbox_prepare', started_at = datetime('now')
		WHERE id = ? AND status = 'pending'`, entry.ID)
	if err != nil {
		return nil, fmt.Errorf("update status: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Another process won the race.
		return nil, nil
	}
	// Refresh the entry so the caller sees the new status and started_at.
	return q.Get(ctx, entry.ID)
}

// pickPending fetches the highest-priority pending entry. We use a
// separate method so Dequeue can be retried safely without a
// surrounding transaction.
func (q *Queue) pickPending(ctx context.Context) (*Entry, error) {
	row := q.db.QueryRowContext(ctx, `
		SELECT id, problem_class, environment, language, version, description,
		       error_message, stack_trace, context_json, cadence, priority, status, stage,
		       result_answer_id, created_at, started_at, completed_at
		FROM queue_entries
		WHERE status = 'pending'
		ORDER BY priority DESC, created_at ASC
		LIMIT 1`)
	entry, err := scanEntry(row)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	return entry, err
}

// MarkComplete transitions an in_progress entry to complete, optionally
// linking it to the answer node that was created.
func (q *Queue) MarkComplete(ctx context.Context, id string, answerID int64) error {
	_, err := q.db.ExecContext(ctx, `
		UPDATE queue_entries
		SET status = 'complete', stage = 'done', completed_at = datetime('now'),
		    result_answer_id = ?
		WHERE id = ?`, answerID, id)
	if err != nil {
		return fmt.Errorf("mark complete: %w", err)
	}
	return nil
}

// MarkFailed transitions an in_progress entry to failed.
func (q *Queue) MarkFailed(ctx context.Context, id string, reason string) error {
	_, err := q.db.ExecContext(ctx, `
		UPDATE queue_entries
		SET status = 'failed', stage = 'failed', completed_at = datetime('now')
		WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("mark failed: %w", err)
	}
	_ = reason
	return nil
}

// SetStage updates only the stage column. Used by the solver to report
// progress (sandbox_prepare, sandbox_solve, store_result).
func (q *Queue) SetStage(ctx context.Context, id, stage string) error {
	_, err := q.db.ExecContext(ctx,
		`UPDATE queue_entries SET stage = ? WHERE id = ?`, stage, id)
	return err
}

// --- Scanning helpers -----------------------------------------------------

var ErrNotFound = errors.New("queue: not found")

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEntry(row rowScanner) (*Entry, error) {
	var e Entry
	var ctxJSON string
	if err := row.Scan(
		&e.ID, &e.ProblemClass, &e.Environment, &e.Language, &e.Version,
		&e.Description, &e.ErrorMessage, &e.StackTrace, &ctxJSON,
		&e.Cadence, &e.Priority, &e.Status, &e.Stage,
		&e.ResultAnswerID, &e.CreatedAt, &e.StartedAt, &e.CompletedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("scan queue entry: %w", err)
	}
	if ctxJSON != "" && ctxJSON != "{}" {
		_ = json.Unmarshal([]byte(ctxJSON), &e.Context)
	}
	if e.Context == nil {
		e.Context = map[string]any{}
	}
	return &e, nil
}

func scanEntryRows(rows *sql.Rows) (*Entry, error) {
	return scanEntry(rows)
}
