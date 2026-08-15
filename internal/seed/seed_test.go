package seed

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
)

// writeCorpus writes a corpus-shaped data dir (dataDir/answers/*.json)
// from a map of filename → file content.
func writeCorpus(t *testing.T, files map[string]string) string {
	t.Helper()
	dataDir := t.TempDir()
	answersDir := filepath.Join(dataDir, "answers")
	if err := os.MkdirAll(answersDir, 0o755); err != nil {
		t.Fatalf("mkdir answers: %v", err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(answersDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dataDir
}

func newTestStore(t *testing.T) *graph.Store {
	t.Helper()
	store, err := graph.Open(filepath.Join(t.TempDir(), "seed-test.db"))
	if err != nil {
		t.Fatalf("graph.Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

const corpusA = `{
  "class_id": 1,
  "title": "go-nil-pointer-deref",
  "description": "runtime panic on nil pointer",
  "created_at": "2026-07-24 22:52:43",
  "answers": [
    {
      "answer_id": 11,
      "language": "go",
      "environment": "go1.26",
      "version": "1.26",
      "solution": "# Solution A\nCheck the pointer before dereferencing.",
      "evidence": "reproducer attached",
      "signatures": {"model": "m1", "problem_class": "go-nil-pointer-deref", "result": "passed", "tests": 5},
      "status": "verified",
      "created_at": "2026-07-24 22:52:43"
    }
  ]
}`

const corpusB = `{
  "class_id": 2,
  "title": "python-count-word-freq",
  "description": "word frequency counter",
  "answers": [
    {
      "answer_id": 21,
      "language": "python",
      "environment": "py3.11",
      "version": "3.11",
      "solution": "# Solution B1\nUse collections.Counter.",
      "evidence": "tested on 3.11",
      "signatures": {"model": "m2", "result": "passed"},
      "status": "verified"
    },
    {
      "answer_id": 22,
      "language": "python",
      "environment": "py3.12",
      "version": "3.12",
      "solution": "# Solution B2\nCounter with type hints.",
      "evidence": "tested on 3.12",
      "signatures": null,
      "status": "verified"
    }
  ]
}`

const corpusC = `{
  "class_id": 3,
  "title": "js-array-dedupe",
  "description": "dedupe array",
  "answers": [
    {
      "answer_id": 31,
      "language": "javascript",
      "environment": "node22",
      "version": "22",
      "solution": "# Solution C\nnew Set(arr).",
      "evidence": "node 22",
      "signatures": {"model": "m3", "result": "passed"},
      "status": "verified"
    }
  ]
}`

func testCorpus() map[string]string {
	return map[string]string{
		"0001-go-nil-pointer-deref.json":   corpusA,
		"0002-python-count-word-freq.json": corpusB,
		"0003-js-array-dedupe.json":        corpusC,
	}
}

// TestSeedLoadsCorpus verifies the loader imports classes + answers,
// marks answers verified, and stores signatures as valid JSON.
func TestSeedLoadsCorpus(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	dir := writeCorpus(t, testCorpus())

	stats, err := Seed(ctx, store, dir)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if stats.FilesLoaded != 3 || stats.ClassesCreated != 3 || stats.AnswersCreated != 4 {
		t.Fatalf("stats: got files=%d classes=%d answers=%d, want 3/3/4",
			stats.FilesLoaded, stats.ClassesCreated, stats.AnswersCreated)
	}

	pc, err := store.GetProblemClassByTitle(ctx, "go-nil-pointer-deref")
	if err != nil {
		t.Fatalf("GetProblemClassByTitle: %v", err)
	}
	answers, err := store.ListAnswers(ctx, pc.ID)
	if err != nil {
		t.Fatalf("ListAnswers: %v", err)
	}
	if len(answers) != 1 {
		t.Fatalf("answers for class: got %d, want 1", len(answers))
	}
	if answers[0].Status != graph.AnswerVerified {
		t.Errorf("answer status: got %q, want %q", answers[0].Status, graph.AnswerVerified)
	}
	if !json.Valid([]byte(answers[0].Signatures)) {
		t.Errorf("stored signatures are not valid JSON: %q", answers[0].Signatures)
	}
	var sigs map[string]any
	if err := json.Unmarshal([]byte(answers[0].Signatures), &sigs); err != nil {
		t.Fatalf("unmarshal stored signatures: %v", err)
	}
	if sigs["model"] != "m1" {
		t.Errorf("signatures.model: got %v, want m1", sigs["model"])
	}

	// The null-signatures answer must have been stored as "{}".
	pcB, err := store.GetProblemClassByTitle(ctx, "python-count-word-freq")
	if err != nil {
		t.Fatalf("GetProblemClassByTitle B: %v", err)
	}
	answersB, err := store.ListAnswers(ctx, pcB.ID)
	if err != nil {
		t.Fatalf("ListAnswers B: %v", err)
	}
	if len(answersB) != 2 {
		t.Fatalf("answers for class B: got %d, want 2", len(answersB))
	}
	foundEmpty := false
	for _, a := range answersB {
		if a.Signatures == "{}" {
			foundEmpty = true
		}
	}
	if !foundEmpty {
		t.Errorf("expected one answer with null signatures stored as \"{}\", got %v", answersB)
	}

	// Discovery must find the seeded class with a verified answer —
	// this is the fresh-install discover path from issue #1.
	res, err := store.Discovery(ctx, "go-nil-pointer-deref", "", "", "", true)
	if err != nil {
		t.Fatalf("Discovery: %v", err)
	}
	if res.Exact == nil {
		t.Fatal("Discovery: no exact answer for seeded class")
	}
}

// TestSeedIdempotentRerun proves a second run creates nothing new.
func TestSeedIdempotentRerun(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	dir := writeCorpus(t, testCorpus())

	if _, err := Seed(ctx, store, dir); err != nil {
		t.Fatalf("Seed run 1: %v", err)
	}
	stats, err := Seed(ctx, store, dir)
	if err != nil {
		t.Fatalf("Seed run 2: %v", err)
	}
	if stats.ClassesCreated != 0 || stats.AnswersCreated != 0 {
		t.Errorf("re-run created content: classes=%d answers=%d, want 0/0",
			stats.ClassesCreated, stats.AnswersCreated)
	}
	if stats.AnswersSkipped != 4 {
		t.Errorf("re-run skipped: got %d, want 4", stats.AnswersSkipped)
	}
	st, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.TotalProblems != 3 || st.TotalAnswers != 4 {
		t.Errorf("totals after re-run: problems=%d answers=%d, want 3/4",
			st.TotalProblems, st.TotalAnswers)
	}
}

// TestSeedDeltaImport verifies a corpus update imports only the new
// answer while existing rows are skipped.
func TestSeedDeltaImport(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	files := testCorpus()
	files["0001-go-nil-pointer-deref.json"] = `{
	  "class_id": 1,
	  "title": "go-nil-pointer-deref",
	  "description": "runtime panic on nil pointer",
	  "answers": [
	    {
	      "answer_id": 11,
	      "language": "go",
	      "environment": "go1.26",
	      "version": "1.26",
	      "solution": "# Solution A\nCheck the pointer before dereferencing.",
	      "evidence": "reproducer attached",
	      "signatures": {"model": "m1", "result": "passed"},
	      "status": "verified"
	    },
	    {
	      "answer_id": 12,
	      "language": "go",
	      "environment": "go1.27",
	      "version": "1.27",
	      "solution": "# Solution A2\nUse generics-safe wrapper.",
	      "evidence": "tested on 1.27",
	      "signatures": {"model": "m1", "result": "passed"},
	      "status": "verified"
	    }
	  ]
	}`
	dir := writeCorpus(t, files)

	if _, err := Seed(ctx, store, dir); err != nil {
		t.Fatalf("Seed run 1: %v", err)
	}
	stats, err := Seed(ctx, store, dir)
	if err != nil {
		t.Fatalf("Seed run 2: %v", err)
	}
	if stats.AnswersCreated != 0 || stats.AnswersSkipped != 5 {
		t.Errorf("run 2: created=%d skipped=%d, want 0/5", stats.AnswersCreated, stats.AnswersSkipped)
	}
}

// TestSeedMissingDirErrors verifies a missing corpus dir fails loudly.
func TestSeedMissingDirErrors(t *testing.T) {
	store := newTestStore(t)
	if _, err := Seed(context.Background(), store, t.TempDir()); err == nil {
		t.Fatal("Seed on missing answers dir: want error, got nil")
	}
}

// TestSeedSkipsNonJSONAndSubdirs verifies only *.json files at the top
// of the answers dir are considered.
func TestSeedSkipsNonJSONAndSubdirs(t *testing.T) {
	store := newTestStore(t)
	files := testCorpus()
	files["notes.txt"] = "not json"
	dir := writeCorpus(t, files)
	if err := os.MkdirAll(filepath.Join(dir, "answers", "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "answers", "sub", "nested.json"), []byte(corpusA), 0o644); err != nil {
		t.Fatalf("write nested: %v", err)
	}

	stats, err := Seed(context.Background(), store, dir)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if stats.FilesLoaded != 3 {
		t.Errorf("files loaded: got %d, want 3 (txt + nested json ignored)", stats.FilesLoaded)
	}
}

// TestSeedDuplicateTitlesAcrossFiles verifies two corpus files sharing
// a title merge into one class instead of erroring.
func TestSeedDuplicateTitlesAcrossFiles(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)
	second := `{
	  "class_id": 9,
	  "title": "go-nil-pointer-deref",
	  "description": "duplicate title, different answer",
	  "answers": [
	    {
	      "answer_id": 91,
	      "language": "go",
	      "environment": "go1.25",
	      "version": "1.25",
	      "solution": "# Solution D\nDifferent tuple.",
	      "evidence": "n/a",
	      "signatures": {"model": "m9", "result": "passed"},
	      "status": "verified"
	    }
	  ]
	}`
	files := testCorpus()
	files["0009-dup.json"] = second
	dir := writeCorpus(t, files)

	if _, err := Seed(ctx, store, dir); err != nil {
		t.Fatalf("Seed: %v", err)
	}
	st, err := store.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.TotalProblems != 3 || st.TotalAnswers != 5 {
		t.Errorf("totals: problems=%d answers=%d, want 3/5", st.TotalProblems, st.TotalAnswers)
	}
}
