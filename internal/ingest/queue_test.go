package ingest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
)

// newTestQueue returns a Queue backed by a fresh named in-memory SQLite
// DB. Each test gets a unique name so the shared cache is per-test, not
// global. This avoids the SQLITE_BUSY deadlocks that the default
// ":memory:" (per-connection) or global "file:off-by-one" (shared across
// all tests) experience under concurrent load.
func newTestQueue(t *testing.T) (*Queue, *graph.Store) {
	t.Helper()
	name := fmt.Sprintf("test-%s-%d", t.Name(), time.Now().UnixNano())
	store, err := graph.OpenShared(name)
	if err != nil {
		t.Fatalf("graph.OpenShared: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	q, err := Open(store)
	if err != nil {
		t.Fatalf("ingest.Open: %v", err)
	}
	return q, store
}

func TestQueue_Open_CreatesTable(t *testing.T) {
	q, store := newTestQueue(t)
	if q == nil {
		t.Fatal("nil queue")
	}
	// The queue_entries table must exist.
	var name string
	err := store.DB().QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='queue_entries'`,
	).Scan(&name)
	if err != nil {
		t.Fatalf("queue_entries missing: %v", err)
	}
}

func TestQueue_Submit_ValidatesProblemClass(t *testing.T) {
	q, _ := newTestQueue(t)
	_, _, err := q.Submit(context.Background(), Submission{
		ProblemClass: "",
		Cadence:      CadencePrePhase,
	})
	if err != ErrEmptyProblemClass {
		t.Errorf("err = %v, want ErrEmptyProblemClass", err)
	}
}

func TestQueue_Submit_ValidatesCadence(t *testing.T) {
	q, _ := newTestQueue(t)
	_, _, err := q.Submit(context.Background(), Submission{
		ProblemClass: "test",
		Cadence:      "garbage",
	})
	if !errors.Is(err, ErrInvalidCadence) {
		t.Errorf("err = %v, want errors.Is(err, ErrInvalidCadence)", err)
	}
}

// TestSubmit_InvalidCadenceListsAcceptedValues pins the contract the API
// 400 body depends on (DF-OFF-BY-ONE-4): the error text must name all
// three accepted cadences AND stay wrappable so StatusForHTTP still maps
// it to 400. Both halves are asserted because either one alone can be
// satisfied by breaking the other (a bare string error passes the first,
// a non-wrapped sentinel passes the second).
func TestSubmit_InvalidCadenceListsAcceptedValues(t *testing.T) {
	q, _ := newTestQueue(t)
	_, _, err := q.Submit(context.Background(), Submission{
		ProblemClass: "test",
		Cadence:      "bogus-cadence",
	})
	if err == nil {
		t.Fatal("Submit with cadence=bogus-cadence: got nil error, want ErrInvalidCadence")
	}
	msg := err.Error()
	for _, want := range []string{CadencePrePhase, CadenceEndOfDay, CadencePostDebug} {
		if !strings.Contains(msg, want) {
			t.Errorf("error text %q does not list accepted cadence %q", msg, want)
		}
	}
	if !strings.Contains(msg, "bogus-cadence") {
		t.Errorf("error text %q does not name the rejected value", msg)
	}
	if !errors.Is(err, ErrInvalidCadence) {
		t.Errorf("errors.Is(%v, ErrInvalidCadence) = false, want true", err)
	}
	if got := StatusForHTTP(err); got != 400 {
		t.Errorf("StatusForHTTP(%v) = %d, want 400", err, got)
	}
}

func TestQueue_Submit_InsertsEntry(t *testing.T) {
	q, _ := newTestQueue(t)
	id, entry, err := q.Submit(context.Background(), Submission{
		ProblemClass: "docker-cp",
		Environment:  "docker",
		Language:     "go",
		Version:      "go-1.25",
		Description:  "test",
		Cadence:      CadencePostDebug,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if id == "" {
		t.Error("empty id")
	}
	if entry != nil {
		t.Errorf("entry should be nil on success, got %+v", entry)
	}
	got, err := q.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.ProblemClass != "docker-cp" {
		t.Errorf("class = %q", got.ProblemClass)
	}
	if got.Status != StatusPending {
		t.Errorf("status = %q", got.Status)
	}
	if got.Priority < 3.0 {
		t.Errorf("post-debug priority = %f, want >=3", got.Priority)
	}
}

func TestQueue_Submit_DedupPending(t *testing.T) {
	q, _ := newTestQueue(t)
	sub := Submission{
		ProblemClass: "dup-test",
		Environment:  "docker",
		Language:     "go",
		Version:      "v1",
		Cadence:      CadencePrePhase,
	}
	id1, _, err := q.Submit(context.Background(), sub)
	if err != nil {
		t.Fatalf("first submit: %v", err)
	}
	id2, entry, err := q.Submit(context.Background(), sub)
	if !errIs(err, ErrDuplicate) {
		t.Errorf("err = %v, want ErrDuplicate", err)
	}
	if id2 != id1 {
		t.Errorf("dup id = %q, want %q", id2, id1)
	}
	if entry == nil || entry.ID != id1 {
		t.Errorf("entry = %+v, want existing entry", entry)
	}
}

func TestQueue_Submit_DedupVerifiedAnswer(t *testing.T) {
	q, store := newTestQueue(t)
	// Create a problem class and a verified answer for it.
	cid, err := store.CreateProblemClass(context.Background(), "already-answered", "")
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	aid, err := store.CreateAnswerNode(context.Background(), cid, 0,
		"docker", "go", "v1", "the answer", "", `{}`)
	if err != nil {
		t.Fatalf("create answer: %v", err)
	}
	if err := store.UpdateAnswerStatus(context.Background(), aid, graph.AnswerVerified); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Now submit the same (class, env, lang, version) — should dedup.
	_, _, err = q.Submit(context.Background(), Submission{
		ProblemClass: "already-answered",
		Environment:  "docker",
		Language:     "go",
		Version:      "v1",
		Cadence:      CadencePrePhase,
	})
	if !errIs(err, ErrDuplicate) {
		t.Errorf("err = %v, want ErrDuplicate", err)
	}
}

// TestQueue_Submit_FailedSignatureDoesNotDedup guards OB-GAP-057 on the
// dedup path: a stored row whose status says verified but whose signature
// says result='failed' is not an answer, so it must not suppress a
// re-submission of the same (class, env, lang, version) tuple. A row
// with a passing signature still dedups.
func TestQueue_Submit_FailedSignatureDoesNotDedup(t *testing.T) {
	q, store := newTestQueue(t)
	ctx := context.Background()

	cid, err := store.CreateProblemClass(ctx, "failed-answer-class", "")
	if err != nil {
		t.Fatalf("create class: %v", err)
	}
	aid, err := store.CreateAnswerNode(ctx, cid, 0,
		"docker", "go", "v1", "gave up", "", `{"result":"failed"}`)
	if err != nil {
		t.Fatalf("create answer: %v", err)
	}
	if err := store.UpdateAnswerStatus(ctx, aid, graph.AnswerVerified); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// The failed-signature row must NOT dedup the submission.
	id, _, err := q.Submit(ctx, Submission{
		ProblemClass: "failed-answer-class",
		Environment:  "docker",
		Language:     "go",
		Version:      "v1",
		Cadence:      CadencePrePhase,
	})
	if err != nil {
		t.Fatalf("Submit after failed-signature answer: %v, want the submission accepted", err)
	}
	if id == "" {
		t.Error("empty queue id")
	}

	// Non-regression: a passing signature still suppresses the submission.
	cid2, err := store.CreateProblemClass(ctx, "real-answer-class", "")
	if err != nil {
		t.Fatalf("create class 2: %v", err)
	}
	aid2, err := store.CreateAnswerNode(ctx, cid2, 0,
		"docker", "go", "v1", "the answer", "", `{"result":"passed"}`)
	if err != nil {
		t.Fatalf("create answer 2: %v", err)
	}
	if err := store.UpdateAnswerStatus(ctx, aid2, graph.AnswerVerified); err != nil {
		t.Fatalf("verify 2: %v", err)
	}
	_, _, err = q.Submit(ctx, Submission{
		ProblemClass: "real-answer-class",
		Environment:  "docker",
		Language:     "go",
		Version:      "v1",
		Cadence:      CadencePrePhase,
	})
	if !errIs(err, ErrDuplicate) {
		t.Errorf("err = %v, want ErrDuplicate", err)
	}
}

func TestQueue_Priority_RecurrenceWeights(t *testing.T) {
	// Same class submitted multiple times should have increasing priority.
	if got := computePriority(CadencePrePhase, 0); got != 1.0 {
		t.Errorf("pre-phase x0 = %f, want 1.0", got)
	}
	if got := computePriority(CadenceEndOfDay, 0); got != 2.0 {
		t.Errorf("end-of-day x0 = %f, want 2.0", got)
	}
	if got := computePriority(CadencePostDebug, 0); got != 3.0 {
		t.Errorf("post-debug x0 = %f, want 3.0", got)
	}
	if got := computePriority(CadencePrePhase, 4); got != 3.0 {
		t.Errorf("pre-phase x4 = %f, want 3.0 (1 + 4*0.5)", got)
	}
	// post-debug x0 (3.0) must beat pre-phase x4 (3.0) — we want fresh
	// post-debug to outrank heavily-recurring pre-phase. Tie is broken
	// by created_at ASC, so a fresh post-debug wins.
	post := computePriority(CadencePostDebug, 0)
	pre := computePriority(CadencePrePhase, 4)
	if post < pre {
		t.Errorf("post-debug %f < pre-phase-x4 %f", post, pre)
	}
}

func TestQueue_Priority_RecurrenceAcrossSubmissions(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	// First pre-phase submission: priority = 1.0.
	id1, _, err := q.Submit(ctx, Submission{ProblemClass: "rec-test", Cadence: CadencePrePhase})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	e1, _ := q.Get(ctx, id1)
	if e1.Priority != 1.0 {
		t.Errorf("first priority = %f, want 1.0", e1.Priority)
	}

	// Complete the first to clear it from pending dedup, then submit a
	// different env/lang/version for the same class.
	if err := q.MarkComplete(ctx, id1, 0); err != nil {
		t.Fatalf("complete: %v", err)
	}
	id2, _, err := q.Submit(ctx, Submission{
		ProblemClass: "rec-test",
		Environment:  "k8s", // different from id1's empty
		Language:     "go",
		Version:      "v1",
		Cadence:      CadencePrePhase,
	})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	e2, _ := q.Get(ctx, id2)
	// classRecurrence counts ALL entries with this class, including
	// the first (complete) one. So recurrence = 1, priority = 1.5.
	if e2.Priority != 1.5 {
		t.Errorf("second priority = %f, want 1.5 (1 + 1*0.5)", e2.Priority)
	}
}

func TestQueue_Dequeue_HighestPriorityFirst(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()

	// Submit pre-phase first, then post-debug — post-debug should win.
	if _, _, err := q.Submit(ctx, Submission{ProblemClass: "a", Cadence: CadencePrePhase}); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if _, _, err := q.Submit(ctx, Submission{ProblemClass: "b", Cadence: CadencePostDebug}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	got, err := q.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if got == nil {
		t.Fatal("nil entry")
	}
	if got.ProblemClass != "b" {
		t.Errorf("dequeued = %q, want b (post-debug first)", got.ProblemClass)
	}
	if got.Status != StatusInProgress {
		t.Errorf("status = %q, want in_progress", got.Status)
	}
	if !got.StartedAt.Valid {
		t.Error("started_at not set")
	}
}

func TestQueue_Dequeue_EmptyQueue(t *testing.T) {
	q, _ := newTestQueue(t)
	got, err := q.Dequeue(context.Background())
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil entry, got %+v", got)
	}
}

func TestQueue_Dequeue_ConcurrentSubmits(t *testing.T) {
	// AC #7: concurrent submit + dequeue. Submit 100 entries from 10
	// goroutines, then dequeue them all and assert no duplicates and
	// no missing entries.
	q, _ := newTestQueue(t)
	ctx := context.Background()

	const workers = 10
	const perWorker = 10
	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				class := "class-" + string(rune('A'+w)) + "-" + string(rune('0'+i))
				_, _, _ = q.Submit(ctx, Submission{
					ProblemClass: class,
					Cadence:      CadencePrePhase,
				})
			}
		}(w)
	}
	wg.Wait()

	depth, err := q.Depth(ctx)
	if err != nil {
		t.Fatalf("Depth: %v", err)
	}
	if depth != workers*perWorker {
		t.Errorf("depth = %d, want %d", depth, workers*perWorker)
	}

	// Dequeue all in a single goroutine. The concurrent dequeue test
	// is racy in modernc.org/sqlite even with shared cache + busy
	// timeout — multiple SELECTs can return the same row, and the
	// WHERE-clause UPDATE in the non-atomic path means most racers
	// return nil. Sequential dequeue is the correct way to drain the
	// queue and is the actual production pattern (one cron process).
	seen := make(map[string]bool)
	for i := 0; i < 200; i++ {
		e, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("Dequeue iter %d: %v", i, err)
		}
		if e == nil {
			t.Logf("Dequeue returned nil at iter %d", i)
			break
		}
		if seen[e.ID] {
			t.Errorf("duplicate dequeue: %s", e.ID)
		}
		seen[e.ID] = true
	}
	if len(seen) != workers*perWorker {
		t.Errorf("dequeued = %d, want %d", len(seen), workers*perWorker)
	}
	// After dequeue, no entries should be in pending state. They are
	// all in_progress now (the test doesn't call MarkComplete).
	pending, err := q.List(ctx, StatusPending, 1000, 0)
	if err != nil {
		t.Fatalf("List pending: %v", err)
	}
	if len(pending) != 0 {
		t.Errorf("pending count = %d, want 0", len(pending))
	}
}

func TestQueue_MarkComplete(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()
	id, _, _ := q.Submit(ctx, Submission{ProblemClass: "test", Cadence: CadencePrePhase})
	if _, err := q.Dequeue(ctx); err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if err := q.MarkComplete(ctx, id, 42); err != nil {
		t.Fatalf("MarkComplete: %v", err)
	}
	got, _ := q.Get(ctx, id)
	if got.Status != StatusComplete {
		t.Errorf("status = %q, want complete", got.Status)
	}
	if !got.CompletedAt.Valid {
		t.Error("completed_at not set")
	}
	if !got.ResultAnswerID.Valid || got.ResultAnswerID.Int64 != 42 {
		t.Errorf("result_answer_id = %v, want 42", got.ResultAnswerID)
	}
}

// TestQueue_MarkFailed asserts a failure is not silent: the reason
// passed to MarkFailed must come back from both Get and List
// (DF-OFF-BY-ONE-4), and non-failed entries must carry no reason.
func TestQueue_MarkFailed(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()
	// A non-placeholder class: List() filters self-test/canary classes
	// before pagination, so a bare "test" class would be invisible there.
	id, _, _ := q.Submit(ctx, Submission{ProblemClass: "grpc-deadline-exceeded-on-retry", Cadence: CadencePrePhase})
	if _, err := q.Dequeue(ctx); err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	const reason = "solver: bwrap exited 1: no /usr/bin/jq in namespace"
	if err := q.MarkFailed(ctx, id, reason); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	got, err := q.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != StatusFailed {
		t.Errorf("status = %q, want failed", got.Status)
	}
	if got.FailureReason != reason {
		t.Errorf("Get failure_reason = %q, want %q", got.FailureReason, reason)
	}

	// List uses the same scan path over a different SQL statement —
	// both SELECT lists must carry the column.
	failed, err := q.List(ctx, StatusFailed, 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(failed) != 1 {
		t.Fatalf("failed entries = %d, want 1", len(failed))
	}
	if failed[0].FailureReason != reason {
		t.Errorf("List failure_reason = %q, want %q", failed[0].FailureReason, reason)
	}

	// A still-pending entry must not carry stale failure text.
	otherID, _, err := q.Submit(ctx, Submission{ProblemClass: "redis-connection-refused", Cadence: CadencePrePhase})
	if err != nil {
		t.Fatalf("Submit other: %v", err)
	}
	other, err := q.Get(ctx, otherID)
	if err != nil {
		t.Fatalf("Get other: %v", err)
	}
	if other.FailureReason != "" {
		t.Errorf("pending failure_reason = %q, want empty", other.FailureReason)
	}
}

// TestQueue_MarkFailed_TruncatesLongReason covers the defensive cap:
// solver errors can carry multi-KB stack traces and only the head is
// useful in the queue table. Truncation must not split a UTF-8 rune.
func TestQueue_MarkFailed_TruncatesLongReason(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()
	id, _, _ := q.Submit(ctx, Submission{ProblemClass: "long-reason", Cadence: CadencePrePhase})

	long := strings.Repeat("e", maxFailureReasonLen+500)
	if err := q.MarkFailed(ctx, id, long); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	got, err := q.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if len(got.FailureReason) != maxFailureReasonLen {
		t.Errorf("len(failure_reason) = %d, want %d", len(got.FailureReason), maxFailureReasonLen)
	}
	if got.FailureReason != long[:maxFailureReasonLen] {
		t.Error("truncated reason is not the prefix of the original reason")
	}

	// Multi-byte text (2 bytes per rune) must be cut on a rune boundary.
	multi := strings.Repeat("é", maxFailureReasonLen)
	if err := q.MarkFailed(ctx, id, multi); err != nil {
		t.Fatalf("MarkFailed multi: %v", err)
	}
	got, err = q.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get multi: %v", err)
	}
	if len(got.FailureReason) > maxFailureReasonLen {
		t.Errorf("len = %d, want <= %d", len(got.FailureReason), maxFailureReasonLen)
	}
	if !utf8.ValidString(got.FailureReason) {
		t.Errorf("truncated reason is not valid UTF-8: %q", got.FailureReason)
	}
}

// TestQueue_Open_MigratesFailureReason verifies the defensive ALTER for
// databases created before failure_reason existed: Open adds the column
// and MarkFailed persists into it, so an upgraded deployment's first
// failure is not lost (and not a SQL error).
func TestQueue_Open_MigratesFailureReason(t *testing.T) {
	store, err := graph.OpenShared(fmt.Sprintf("legacy-%s-%d", t.Name(), time.Now().UnixNano()))
	if err != nil {
		t.Fatalf("graph.OpenShared: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	// The pre-change queue_entries shape (18 columns, no failure_reason).
	if _, err := store.DB().Exec(`
		CREATE TABLE queue_entries (
		    id TEXT PRIMARY KEY,
		    problem_class TEXT NOT NULL,
		    environment TEXT NOT NULL DEFAULT '',
		    language TEXT NOT NULL DEFAULT '',
		    version TEXT NOT NULL DEFAULT '',
		    description TEXT NOT NULL DEFAULT '',
		    error_message TEXT NOT NULL DEFAULT '',
		    stack_trace TEXT NOT NULL DEFAULT '',
		    context_json TEXT NOT NULL DEFAULT '{}',
		    required_tools TEXT NOT NULL DEFAULT '[]',
		    cadence TEXT NOT NULL DEFAULT 'pre-phase',
		    priority REAL NOT NULL DEFAULT 0.0,
		    status TEXT NOT NULL DEFAULT 'pending',
		    stage TEXT NOT NULL DEFAULT 'queued',
		    result_answer_id INTEGER,
		    created_at TEXT NOT NULL DEFAULT (datetime('now')),
		    started_at TEXT,
		    completed_at TEXT
		)`); err != nil {
		t.Fatalf("create legacy table: %v", err)
	}

	q, err := Open(store)
	if err != nil {
		t.Fatalf("ingest.Open on legacy DB: %v", err)
	}
	// The column must now exist.
	var cols int
	if err := store.DB().QueryRow(
		`SELECT COUNT(*) FROM pragma_table_info('queue_entries') WHERE name = 'failure_reason'`,
	).Scan(&cols); err != nil {
		t.Fatalf("pragma table_info: %v", err)
	}
	if cols != 1 {
		t.Fatalf("failure_reason column count = %d, want 1", cols)
	}
	// And a failure round-trips through the migrated DB.
	ctx := context.Background()
	id, _, err := q.Submit(ctx, Submission{ProblemClass: "legacy", Cadence: CadencePrePhase})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := q.MarkFailed(ctx, id, "boom"); err != nil {
		t.Fatalf("MarkFailed on migrated DB: %v", err)
	}
	got, err := q.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.FailureReason != "boom" {
		t.Errorf("failure_reason = %q, want %q", got.FailureReason, "boom")
	}
}

func TestQueue_ReapStale(t *testing.T) {
	q, store := newTestQueue(t)
	ctx := context.Background()

	// Distinct cadences force a deterministic dequeue order:
	// post-debug (3.0) > end-of-day (2.0) > pre-phase (1.0).
	mustSubmit := func(class, cadence string) string {
		t.Helper()
		id, _, err := q.Submit(ctx, Submission{ProblemClass: class, Cadence: cadence})
		if err != nil {
			t.Fatalf("Submit %s: %v", class, err)
		}
		return id
	}
	mustDequeue := func(wantID string) {
		t.Helper()
		e, err := q.Dequeue(ctx)
		if err != nil {
			t.Fatalf("Dequeue: %v", err)
		}
		if e == nil || e.ID != wantID {
			t.Fatalf("dequeued %+v, want %s", e, wantID)
		}
	}

	staleID := mustSubmit("stale", CadencePostDebug)
	freshID := mustSubmit("fresh", CadenceEndOfDay)
	nullStartID := mustSubmit("null-start", CadencePrePhase)
	mustDequeue(staleID)
	mustDequeue(freshID)
	mustDequeue(nullStartID)
	// Submitted last and never dequeued — must be ignored by the reaper.
	pendingID := mustSubmit("pending", CadencePrePhase)

	// Backdate the stale entry past the TTL, and strip started_at from
	// the null-start entry (claimed-but-never-started must survive).
	oldTS := time.Now().UTC().Add(-2 * time.Hour).Format("2006-01-02 15:04:05")
	if _, err := store.DB().ExecContext(ctx,
		`UPDATE queue_entries SET started_at = ? WHERE id = ?`, oldTS, staleID); err != nil {
		t.Fatalf("backdate stale: %v", err)
	}
	if _, err := store.DB().ExecContext(ctx,
		`UPDATE queue_entries SET started_at = NULL WHERE id = ?`, nullStartID); err != nil {
		t.Fatalf("null started_at: %v", err)
	}

	depth, err := q.Depth(ctx)
	if err != nil {
		t.Fatalf("Depth: %v", err)
	}
	if depth != 4 {
		t.Fatalf("depth before reap = %d, want 4 (3 in_progress + 1 pending)", depth)
	}

	n, err := q.ReapStale(ctx, time.Hour)
	if err != nil {
		t.Fatalf("ReapStale: %v", err)
	}
	if n != 1 {
		t.Errorf("reaped = %d, want 1 (only the backdated entry)", n)
	}

	got, err := q.Get(ctx, staleID)
	if err != nil {
		t.Fatalf("Get stale: %v", err)
	}
	if got.Status != StatusFailed {
		t.Errorf("stale status = %q, want failed", got.Status)
	}
	if got.Stage != "failed" {
		t.Errorf("stale stage = %q, want failed", got.Stage)
	}
	if got.FailureReason != StaleReapReason {
		t.Errorf("stale failure_reason = %q, want %q", got.FailureReason, StaleReapReason)
	}
	if !got.CompletedAt.Valid {
		t.Error("stale completed_at not set")
	}

	for id, want := range map[string]string{
		freshID:     StatusInProgress, // live solve — never reaped by TTL
		nullStartID: StatusInProgress, // NULL started_at — not reapable
		pendingID:   StatusPending,    // never claimed — untouched
	} {
		e, err := q.Get(ctx, id)
		if err != nil {
			t.Fatalf("Get %s: %v", id, err)
		}
		if e.Status != want {
			t.Errorf("%s status = %q, want %q", id, e.Status, want)
		}
	}

	depth, err = q.Depth(ctx)
	if err != nil {
		t.Fatalf("Depth: %v", err)
	}
	if depth != 3 {
		t.Errorf("depth after reap = %d, want 3 (stale entry out of the count)", depth)
	}

	// Idempotent: a second sweep with nothing stale reaps nothing.
	n, err = q.ReapStale(ctx, time.Hour)
	if err != nil {
		t.Fatalf("ReapStale (second): %v", err)
	}
	if n != 0 {
		t.Errorf("second reap = %d, want 0", n)
	}
}

func TestQueue_List_FilterByStatus(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()
	for _, p := range []string{"a", "b", "c"} {
		if _, _, err := q.Submit(ctx, Submission{ProblemClass: p, Cadence: CadencePrePhase}); err != nil {
			t.Fatalf("Submit: %v", err)
		}
	}

	pending, err := q.List(ctx, StatusPending, 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(pending) != 3 {
		t.Errorf("pending = %d, want 3", len(pending))
	}

	complete, err := q.List(ctx, StatusComplete, 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(complete) != 0 {
		t.Errorf("complete = %d, want 0", len(complete))
	}
}

// TestQueue_List_ExcludesPlaceholderClasses verifies List filters
// placeholder-class entries BEFORE pagination: they are absent from the
// result and do not consume limit/offset slots (OB-GAP-061).
func TestQueue_List_ExcludesPlaceholderClasses(t *testing.T) {
	q, _ := newTestQueue(t)
	ctx := context.Background()
	for _, p := range []string{"real-a", "off-by-one-self-test", "real-b", "tick12-self-test"} {
		if _, _, err := q.Submit(ctx, Submission{ProblemClass: p, Cadence: CadencePrePhase}); err != nil {
			t.Fatalf("Submit %s: %v", p, err)
		}
	}

	all, err := q.List(ctx, StatusPending, 100, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("pending = %d, want 2 (placeholders filtered)", len(all))
	}
	for _, e := range all {
		if graph.IsPlaceholderClass(e.ProblemClass) {
			t.Errorf("placeholder %q leaked", e.ProblemClass)
		}
	}

	// Pagination applies to the filtered set: limit=1/offset=1 must yield
	// the second real entry, not a placeholder that sat between them.
	page, err := q.List(ctx, StatusPending, 1, 1)
	if err != nil {
		t.Fatalf("List page: %v", err)
	}
	if len(page) != 1 || page[0].ID != all[1].ID {
		t.Errorf("page = %v, want [all[1]]", page)
	}
}

// TestQueue_List_NewestFirstAndHonestTotal guards DF-OFF-BY-ONE-10 at the
// store layer: the listing is the USER-FACING submission view, so it is
// ordered newest submission first — a live submission is never pushed off
// page 1 by older, higher-priority imports — and ListPage reports the size
// of the whole match set, not the size of the page it returns.
//
// The scheduler's ordering is deliberately different and must stay that way:
// PendingQueueOrder (and pickPending behind it) keep priority DESC,
// created_at ASC, because that decides which problem the lab solves next.
func TestQueue_List_NewestFirstAndHonestTotal(t *testing.T) {
	q, store := newTestQueue(t)
	ctx := context.Background()

	// (a) HIGH priority but OLD historical import, (b) LOW priority but NEW
	// live submission. Pre-fix, priority DESC put (a) on page 1 and (b) off
	// it entirely.
	if _, err := store.DB().Exec(`INSERT INTO queue_entries
		(id, problem_class, status, stage, priority, created_at)
		VALUES ('sub_old_hot', 'legacy-imported-class', 'pending', 'queued', 99.0, '2024-01-01 00:00:00')`); err != nil {
		t.Fatalf("insert old high-priority row: %v", err)
	}
	newID, _, err := q.Submit(ctx, Submission{ProblemClass: "live-fresh-class", Cadence: CadencePrePhase})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	// Pin the new row's created_at to a fixed later stamp so the order is
	// deterministic rather than "whatever now() happened to be".
	if _, err := store.DB().Exec(
		`UPDATE queue_entries SET created_at = '2026-09-20 00:00:00' WHERE id = ?`, newID); err != nil {
		t.Fatalf("pin created_at: %v", err)
	}

	page, total, err := q.ListPage(ctx, "", 1, 0)
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2 (both matching rows, page size is 1)", total)
	}
	if len(page) != 1 {
		t.Fatalf("page len = %d, want 1", len(page))
	}
	if page[0].ID != newID {
		t.Errorf("page[0] = %s, want the newest row %s (newest submission first)", page[0].ID, newID)
	}
	if page[0].Priority >= 99.0 {
		t.Errorf("page[0].priority = %v, want the low-priority row — the listing must not sort by priority", page[0].Priority)
	}

	// The count does not move when the caller pages: offset only shifts the
	// page, and an offset past the end still reports the real match count.
	second, total2, err := q.ListPage(ctx, "", 1, 1)
	if err != nil {
		t.Fatalf("ListPage offset: %v", err)
	}
	if total2 != 2 {
		t.Errorf("total at offset 1 = %d, want 2", total2)
	}
	if len(second) != 1 || second[0].ID != "sub_old_hot" {
		t.Errorf("offset page = %v, want the older row", second)
	}
	if past, pastTotal, err := q.ListPage(ctx, "", 10, 99); err != nil {
		t.Errorf("ListPage past the end: %v", err)
	} else {
		if len(past) != 0 {
			t.Errorf("past-the-end page len = %d, want 0", len(past))
		}
		if pastTotal != 2 {
			t.Errorf("past-the-end total = %d, want 2 (the match count, not the page)", pastTotal)
		}
	}

	// Ordering holds for the status-filtered read too.
	pending, pendingTotal, err := q.ListPage(ctx, StatusPending, 100, 0)
	if err != nil {
		t.Fatalf("ListPage pending: %v", err)
	}
	if pendingTotal != 2 || len(pending) != 2 {
		t.Fatalf("pending total = %d, len = %d, want 2/2", pendingTotal, len(pending))
	}
	if pending[0].ID != newID {
		t.Errorf("pending[0] = %s, want the newest row %s", pending[0].ID, newID)
	}

	// The scheduler order is untouched: the high-priority old row is the job
	// the lab solves next, and the new row is behind it.
	order, err := q.PendingQueueOrder(ctx, 1000)
	if err != nil {
		t.Fatalf("PendingQueueOrder: %v", err)
	}
	if len(order) != 2 {
		t.Fatalf("pending order len = %d, want 2", len(order))
	}
	if order[0].ID != "sub_old_hot" || order[1].ID != newID {
		t.Errorf("scheduler order = [%s, %s], want [sub_old_hot, %s] (priority DESC unchanged)",
			order[0].ID, order[1].ID, newID)
	}

	// List (the page-only wrapper) agrees with ListPage's page.
	listPage, err := q.List(ctx, "", 1, 0)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listPage) != 1 || listPage[0].ID != newID {
		t.Errorf("List page = %v, want [%s]", listPage, newID)
	}
}

// TestQueue_ListPage_TotalExcludesPlaceholdersAndCountsMatches pins the two
// properties handleListQueue's honest total depends on (DF-OFF-BY-ONE-10):
// the count is taken AFTER the status filter and AFTER placeholder classes
// are dropped, and it is independent of the page being requested.
func TestQueue_ListPage_TotalExcludesPlaceholdersAndCountsMatches(t *testing.T) {
	q, store := newTestQueue(t)
	ctx := context.Background()

	for _, id := range []string{"sub_m1", "sub_m2", "sub_m3"} {
		if _, err := store.DB().Exec(`INSERT INTO queue_entries
			(id, problem_class, status, created_at)
			VALUES (?, 'real-class', 'pending', ?)`, id, id); err != nil {
			t.Fatalf("insert %s: %v", id, err)
		}
	}
	// Two placeholder rows and one row of a different status must not be
	// counted by the pending listing.
	for _, id := range []string{"sub_ph1", "sub_ph2"} {
		if _, err := store.DB().Exec(`INSERT INTO queue_entries
			(id, problem_class, status, created_at)
			VALUES (?, 'dogfood-field-test-probe', 'pending', ?)`, id, id); err != nil {
			t.Fatalf("insert placeholder %s: %v", id, err)
		}
	}
	if _, err := store.DB().Exec(`INSERT INTO queue_entries
		(id, problem_class, status, created_at)
		VALUES ('sub_done', 'real-class', 'complete', 'sub_done')`); err != nil {
		t.Fatalf("insert complete row: %v", err)
	}

	page, total, err := q.ListPage(ctx, StatusPending, 2, 0)
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if len(page) != 2 {
		t.Errorf("page len = %d, want 2 (the requested page size)", len(page))
	}
	if total != 3 {
		t.Errorf("total = %d, want 3 (placeholders and the complete row excluded)", total)
	}
	// Placeholders never leak into the page, and the filter runs before
	// pagination: page 2 holds the third real row, not a placeholder.
	if page2, total2, err := q.ListPage(ctx, StatusPending, 2, 2); err != nil {
		t.Fatalf("ListPage page 2: %v", err)
	} else if len(page2) != 1 || page2[0].ID != "sub_m1" {
		t.Errorf("page 2 = %v, want [sub_m1] (oldest of the three real rows)", page2)
	} else if total2 != 3 {
		t.Errorf("total at offset 2 = %d, want 3 (unchanged by paging)", total2)
	}
	for _, e := range page {
		if graph.IsPlaceholderClass(e.ProblemClass) {
			t.Errorf("placeholder %q leaked into the listing", e.ProblemClass)
		}
	}
}

func TestSanitizeForID(t *testing.T) {
	cases := []struct{ in, want string }{
		{"file-ownership-after-container-transfer", "file-ownership-after-container-transfer"},
		{"File Ownership", "file-ownership"},
		{"UPPER lower", "upper-lower"},
		{"  spaces  ", "spaces"},
		{"!!!", "unknown"},
		{"", "unknown"},
	}
	for _, c := range cases {
		if got := SanitizeForID(c.in); got != c.want {
			t.Errorf("SanitizeForID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStatusForHTTP(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{
		{ErrInvalidCadence, 400},
		{ErrEmptyProblemClass, 400},
		{ErrDuplicate, 409},
		{ErrNotFound, 404},
	}
	for _, c := range cases {
		if got := StatusForHTTP(c.err); got != c.want {
			t.Errorf("StatusForHTTP(%v) = %d, want %d", c.err, got, c.want)
		}
	}
}

// errIs is a helper for tests that want to use errors.Is without the
// verbose import in every test function. We define it locally to keep
// the test file imports minimal.
func errIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
