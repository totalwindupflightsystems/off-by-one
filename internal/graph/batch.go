package graph

import (
	"context"
	"database/sql"
	"fmt"
)

// Tx is a batch of graph reads and writes bound to a single SQLite
// transaction (PERF-OFF-BY-ONE-1). It exists for bulk loaders — the seed
// corpus import is its first caller — where running every INSERT/UPDATE
// through the pool would autocommit each statement, and each autocommitted
// statement in WAL mode pays its own fsync. On the release install path
// that was one fsync per corpus row: ~9,100 fsyncs and 45-70s of wall time
// on ext4 for ~4s of actual work (the tmpfs control). One transaction
// turns the whole batch into a single commit and a single fsync.
//
// A Tx exposes the same write surface the loaders use — UpsertProblemClass,
// ListAnswers, CreateAnswerNode, UpdateAnswerStatus — and every method runs
// the SAME function body as its Store counterpart, just bound to the
// transaction's connection (see queryer in store.go). Validation,
// normalization (REVIEW-OB-010), duplicate mapping and scan behaviour
// therefore cannot drift between the standalone and batched paths.
//
// Semantics to know before writing a loader on Tx:
//
//   - Read-your-writes: statements inside the transaction see the batch's
//     own earlier writes, so e.g. upserting a title the batch already
//     created returns the existing class instead of inserting a duplicate
//     that would only fail the UNIQUE constraint at commit time.
//   - All-or-nothing: any error must be answered with Rollback (the
//     caller's deferred Rollback is the idiomatic shape), which leaves the
//     database exactly as it was. Bulk loaders are idempotent, so an
//     aborted batch is simply re-run.
//   - Single goroutine: a Tx wraps one *sql.Tx on one connection; it is
//     not safe for concurrent use, unlike Store.
//   - Keep filesystem or network I/O out of the transaction: read and
//     parse inputs first, then write. A deferred SQLite transaction takes
//     its write lock on the first write statement, so a batch of pure
//     writes never blocks WAL readers longer than its own commit.
type Tx struct {
	tx *sql.Tx
}

// BeginTx opens a write batch on the store's database. The batch ends with
// exactly one of Commit or Rollback; Rollback after Commit is a harmless
// no-op (sql.ErrTxDone), which is what makes deferred-rollback error paths
// safe.
func (s *Store) BeginTx(ctx context.Context) (*Tx, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin graph transaction: %w", err)
	}
	return &Tx{tx: tx}, nil
}

// Commit durably applies the whole batch. This is the point the batch pays
// its single fsync; every statement before it was logged to the WAL
// without one.
func (t *Tx) Commit() error {
	if err := t.tx.Commit(); err != nil {
		return fmt.Errorf("commit graph transaction: %w", err)
	}
	return nil
}

// Rollback discards the whole batch. Safe to call after Commit (returns
// sql.ErrTxDone); deferred rollback paths should ignore the error.
func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

// UpsertProblemClass is the Store.UpsertProblemClass body running inside
// the batch: find by title, create when missing. A created class takes the
// created_at column default.
func (t *Tx) UpsertProblemClass(ctx context.Context, title, description string) (*ProblemClass, bool, error) {
	return upsertProblemClass(ctx, t.tx, title, description, "")
}

// UpsertProblemClassAt is the Store.UpsertProblemClassAt body running inside
// the batch: the created_at value applies only when the class is created.
func (t *Tx) UpsertProblemClassAt(ctx context.Context, title, description, createdAt string) (*ProblemClass, bool, error) {
	return upsertProblemClass(ctx, t.tx, title, description, createdAt)
}

// ListAnswers is the Store.ListAnswers body running inside the batch. It
// sees both committed rows and the batch's own uncommitted writes — that
// is what keeps the seed loader's (env, lang, version, solution) dedup
// correct when two corpus files share one class title.
func (t *Tx) ListAnswers(ctx context.Context, classID int64) ([]AnswerNode, error) {
	return listAnswers(ctx, t.tx, classID)
}

// CreateAnswerNode is the Store.CreateAnswerNode body running inside the
// batch, including signatures validation and env/version normalization.
// created_at takes the column default.
func (t *Tx) CreateAnswerNode(ctx context.Context, classID int64, parentID int64, env, lang, version, solution, evidence, signatures string) (int64, error) {
	return createAnswerNode(ctx, t.tx, classID, parentID, env, lang, version, solution, evidence, signatures, "")
}

// CreateAnswerNodeAt is the Store.CreateAnswerNodeAt body running inside
// the batch — same validation and normalization, with an explicit
// created_at value for the corpus timestamp-preserving seed path
// (DF-OFF-BY-ONE-58).
func (t *Tx) CreateAnswerNodeAt(ctx context.Context, classID int64, parentID int64, env, lang, version, solution, evidence, signatures, createdAt string) (int64, error) {
	return createAnswerNode(ctx, t.tx, classID, parentID, env, lang, version, solution, evidence, signatures, createdAt)
}

// UpdateAnswerStatus is the Store.UpdateAnswerStatus body running inside
// the batch, including the status-vocabulary check.
func (t *Tx) UpdateAnswerStatus(ctx context.Context, id int64, status string) error {
	return updateAnswerStatus(ctx, t.tx, id, status)
}
