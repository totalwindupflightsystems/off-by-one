package graph

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

// newFileTestStore returns a Store backed by a fresh on-disk SQLite DB.
// Batch tests use a real file (not :memory:) so the transaction runs on
// the same WAL path the production seed uses, and so each test is
// hermetic — Open(":memory:") maps to one process-wide shared-cache name.
func newFileTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "batch-test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestTxCommitPersistsAndSeesOwnWrites proves the two properties the seed
// loader relies on: a batch sees its own uncommitted writes (upserting a
// title the batch already created returns the existing class instead of
// erroring at commit), and Commit makes the batch visible on the store.
func TestTxCommitPersistsAndSeesOwnWrites(t *testing.T) {
	ctx := context.Background()
	store := newFileTestStore(t)

	tx, err := store.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}

	class, created, err := tx.UpsertProblemClass(ctx, "go-nil-pointer-deref", "runtime panic on nil pointer")
	if err != nil {
		t.Fatalf("UpsertProblemClass: %v", err)
	}
	if !created {
		t.Fatal("first upsert: created = false, want true")
	}

	// Read-your-writes: the same upsert inside the SAME batch must find
	// the class the batch just created. The seed hits this path whenever
	// two corpus files share one class title.
	again, createdAgain, err := tx.UpsertProblemClass(ctx, "go-nil-pointer-deref", "ignored")
	if err != nil {
		t.Fatalf("second UpsertProblemClass: %v", err)
	}
	if createdAgain {
		t.Fatal("second upsert: created = true, want false (batch must see its own writes)")
	}
	if again.ID != class.ID {
		t.Fatalf("second upsert returned class %d, want %d", again.ID, class.ID)
	}

	answerID, err := tx.CreateAnswerNode(ctx, class.ID, 0,
		"go1.26", "go", "1.26", "check the pointer", "reproducer", `{"model":"m1","result":"passed"}`)
	if err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}
	if err := tx.UpdateAnswerStatus(ctx, answerID, AnswerVerified); err != nil {
		t.Fatalf("UpdateAnswerStatus: %v", err)
	}

	// The batch's own read path must report the uncommitted row with its
	// updated status.
	answers, err := tx.ListAnswers(ctx, class.ID)
	if err != nil {
		t.Fatalf("ListAnswers in batch: %v", err)
	}
	if len(answers) != 1 || answers[0].Status != AnswerVerified {
		t.Fatalf("ListAnswers in batch: got %+v, want 1 verified answer", answers)
	}

	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// After commit the store path sees exactly what the batch wrote.
	stored, err := store.GetProblemClassByTitle(ctx, "go-nil-pointer-deref")
	if err != nil {
		t.Fatalf("GetProblemClassByTitle after commit: %v", err)
	}
	storedAnswers, err := store.ListAnswers(ctx, stored.ID)
	if err != nil {
		t.Fatalf("ListAnswers after commit: %v", err)
	}
	if len(storedAnswers) != 1 || storedAnswers[0].Status != AnswerVerified {
		t.Fatalf("ListAnswers after commit: got %+v, want 1 verified answer", storedAnswers)
	}
	if storedAnswers[0].Solution != "check the pointer" || storedAnswers[0].Signatures != `{"model":"m1","result":"passed"}` {
		t.Fatalf("answer content drifted: %+v", storedAnswers[0])
	}
}

// TestTxRollbackDiscardsBatch proves an aborted batch leaves the database
// exactly as it was — the atomicity the seed loader's error and
// cancellation paths rely on.
func TestTxRollbackDiscardsBatch(t *testing.T) {
	ctx := context.Background()
	store := newFileTestStore(t)

	tx, err := store.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	class, _, err := tx.UpsertProblemClass(ctx, "rolled-back-class", "never committed")
	if err != nil {
		t.Fatalf("UpsertProblemClass: %v", err)
	}
	if _, err := tx.CreateAnswerNode(ctx, class.ID, 0, "env", "go", "1.0", "sol", "ev", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	if _, err := store.GetProblemClassByTitle(ctx, "rolled-back-class"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("class after rollback: got err=%v, want ErrNotFound", err)
	}
	st, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.TotalProblems != 0 || st.TotalAnswers != 0 {
		t.Fatalf("totals after rollback: problems=%d answers=%d, want 0/0", st.TotalProblems, st.TotalAnswers)
	}
}

// TestTxValidationParity proves the batched write path applies the same
// input validation as the standalone Store methods — batching must not
// become a way to smuggle invalid rows past the write funnel.
func TestTxValidationParity(t *testing.T) {
	ctx := context.Background()
	store := newFileTestStore(t)

	tx, err := store.BeginTx(ctx)
	if err != nil {
		t.Fatalf("BeginTx: %v", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, _, err := tx.UpsertProblemClass(ctx, "", "no title"); err == nil {
		t.Fatal("upsert with empty title: want error, got nil")
	}
	class, _, err := tx.UpsertProblemClass(ctx, "validation-class", "")
	if err != nil {
		t.Fatalf("UpsertProblemClass: %v", err)
	}
	if _, err := tx.CreateAnswerNode(ctx, 0, 0, "env", "go", "1.0", "sol", "ev", "{}"); err == nil {
		t.Fatal("create answer with class_id 0: want error, got nil")
	}
	if _, err := tx.CreateAnswerNode(ctx, class.ID, 0, "env", "go", "1.0", "sol", "ev", "not json"); err == nil {
		t.Fatal("create answer with invalid signatures JSON: want error, got nil")
	}
	answerID, err := tx.CreateAnswerNode(ctx, class.ID, 0, "env", "go", "1.0", "sol", "ev", "")
	if err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}
	if err := tx.UpdateAnswerStatus(ctx, answerID, "archived"); err == nil {
		t.Fatal("update to invalid status: want error, got nil")
	}
	if err := tx.UpdateAnswerStatus(ctx, answerID, AnswerCIPassed); err != nil {
		t.Fatalf("update to valid status: %v", err)
	}

	// Normalization parity (REVIEW-OB-010): prose env/version values are
	// canonicalized on the batched path exactly like the standalone path.
	proseID, err := tx.CreateAnswerNode(ctx, class.ID, 0, "Docker Desktop", "go", "Go 1.26", "sol2", "ev", "{}")
	if err != nil {
		t.Fatalf("CreateAnswerNode prose: %v", err)
	}
	row, err := tx.ListAnswers(ctx, class.ID)
	if err != nil {
		t.Fatalf("ListAnswers: %v", err)
	}
	var prose *AnswerNode
	for i := range row {
		if row[i].ID == proseID {
			prose = &row[i]
		}
	}
	if prose == nil {
		t.Fatalf("answer %d not listed", proseID)
	}
	if prose.Env != "docker" || prose.Version != "1.26" {
		t.Fatalf("normalization skipped on batched path: env=%q version=%q, want docker/1.26", prose.Env, prose.Version)
	}
}
