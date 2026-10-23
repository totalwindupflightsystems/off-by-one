package seed

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
)

// TestSeedAtomicRollbackOnWriteError proves the corpus load runs as ONE
// transaction (PERF-OFF-BY-ONE-1): a file whose answers fail validation
// during the WRITE phase aborts the whole seed, the error still names the
// file, and the database holds NOTHING from the batch — no classes, no
// answers from the files that loaded cleanly before it. Before batching,
// each statement autocommitted, so the same failure left the already
// processed prefix permanently committed.
//
// The write-phase failure is a signatures value that is valid JSON (so
// phase-1 parsing accepts the file) but not a JSON object (a list) — that
// fails marshalSignatures inside the transaction, after earlier files
// have already been written to it.
func TestSeedAtomicRollbackOnWriteError(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	files := testCorpus()
	files["9999-bad-signatures.json"] = `{
  "class_id": 99,
  "title": "bad-signatures-class",
  "description": "answer signatures are a list, not an object",
  "answers": [
    {
      "answer_id": 99,
      "language": "go",
      "environment": "go1.26",
      "version": "1.26",
      "solution": "# Broken\nNot importable.",
      "evidence": "n/a",
      "signatures": [1, 2, 3],
      "status": "verified"
    }
  ]
}`
	dir := writeCorpus(t, files)

	stats, err := Seed(ctx, store, dir)
	if err == nil {
		t.Fatal("Seed with an unimportable answer: want error, got nil")
	}
	if stats != nil {
		t.Fatalf("failed Seed returned stats %+v, want nil", stats)
	}
	if !strings.Contains(err.Error(), "9999-bad-signatures.json") {
		t.Fatalf("error does not name the failing file: %v", err)
	}

	// Atomicity: nothing from the batch survived — including the three
	// good files that were written into the transaction before the bad
	// file aborted it.
	st, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.TotalProblems != 0 || st.TotalAnswers != 0 {
		t.Fatalf("rolled-back seed left rows behind: problems=%d answers=%d, want 0/0",
			st.TotalProblems, st.TotalAnswers)
	}

	// Recovery: remove the bad file and re-run — the idempotent loader
	// imports the whole corpus cleanly.
	delete(files, "9999-bad-signatures.json")
	dir = writeCorpus(t, files)
	stats, err = Seed(ctx, store, dir)
	if err != nil {
		t.Fatalf("Seed after removing bad file: %v", err)
	}
	if stats.ClassesCreated != 3 || stats.AnswersCreated != 4 {
		t.Fatalf("recovery seed: classes=%d answers=%d, want 3/4",
			stats.ClassesCreated, stats.AnswersCreated)
	}
}

// TestSeedCancelledContextLeavesNoWrites proves a cancelled context aborts
// the load before any row is committed — the rollback path the deferred
// transaction guard exists for.
func TestSeedCancelledContextLeavesNoWrites(t *testing.T) {
	store := newTestStore(t)
	dir := writeCorpus(t, testCorpus())

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Seed(ctx, store, dir); err == nil {
		t.Fatal("Seed with cancelled context: want error, got nil")
	}
	st, err := store.Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.TotalProblems != 0 || st.TotalAnswers != 0 {
		t.Fatalf("cancelled seed left rows behind: problems=%d answers=%d, want 0/0",
			st.TotalProblems, st.TotalAnswers)
	}
}

// TestSeedDeterministicAcrossDatabases pins the loader's output: seeding
// the same corpus into two independent fresh databases must produce
// identical rows (IDs included, since insertion order follows the sorted
// filename order) in all three tables. This is the regression net for the
// batching change — any reordering, row drift or duplicate-handling
// change shows up as a diff here. created_at columns are excluded: they
// are wall-clock defaults, not loader output.
func TestSeedDeterministicAcrossDatabases(t *testing.T) {
	ctx := context.Background()
	corpus := testCorpus()
	for name, content := range relateCorpus() {
		corpus[name] = content
	}

	storeA := newTestStore(t)
	storeB := newTestStore(t)
	dir := writeCorpus(t, corpus)

	statsA, err := Seed(ctx, storeA, dir)
	if err != nil {
		t.Fatalf("Seed A: %v", err)
	}
	statsB, err := Seed(ctx, storeB, dir)
	if err != nil {
		t.Fatalf("Seed B: %v", err)
	}
	if *statsA != *statsB {
		t.Fatalf("stats differ across databases: A=%+v B=%+v", statsA, statsB)
	}
	if statsA.EdgesCreated == 0 {
		t.Fatal("seed created no edges — the edge parity check would be vacuous")
	}

	dumpA := dumpGraph(t, ctx, storeA)
	dumpB := dumpGraph(t, ctx, storeB)
	if len(dumpA) != len(dumpB) {
		t.Fatalf("row counts differ: A=%d B=%d", len(dumpA), len(dumpB))
	}
	for i := range dumpA {
		if dumpA[i] != dumpB[i] {
			t.Fatalf("row %d differs:\nA: %s\nB: %s", i, dumpA[i], dumpB[i])
		}
	}
	if len(dumpA) == 0 {
		t.Fatal("dump is empty — the parity check would be vacuous")
	}
}

// dumpGraph renders every content row of the three graph tables as
// canonical text lines (created_at excluded — wall-clock default, not
// loader output).
func dumpGraph(t *testing.T, ctx context.Context, store *graph.Store) []string {
	t.Helper()
	queries := []string{
		`SELECT id, title, description FROM problem_classes ORDER BY id`,
		`SELECT id, class_id, COALESCE(parent_id, -1), env, lang, version, solution, evidence, signatures, status FROM answer_nodes ORDER BY id`,
		`SELECT source_id, target_id, relationship, weight FROM problem_edges ORDER BY source_id, target_id, relationship`,
	}
	var lines []string
	for qi, q := range queries {
		rows, err := store.DB().QueryContext(ctx, q)
		if err != nil {
			t.Fatalf("dump query %d: %v", qi, err)
		}
		cols, err := rows.Columns()
		if err != nil {
			t.Fatalf("dump columns %d: %v", qi, err)
		}
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatalf("dump scan %d: %v", qi, err)
			}
			parts := make([]string, len(vals))
			for i, v := range vals {
				switch x := v.(type) {
				case nil:
					parts[i] = "<nil>"
				case []byte:
					parts[i] = string(x)
				default:
					parts[i] = fmt.Sprintf("%v", x)
				}
			}
			lines = append(lines, fmt.Sprintf("q%d | %s", qi, strings.Join(parts, " | ")))
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("dump rows %d: %v", qi, err)
		}
		_ = rows.Close()
	}
	return lines
}
