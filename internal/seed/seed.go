// Package seed loads the bundled flat answer corpus (data/answers/*.json)
// into the graph store. It powers the `off-by-one seed` subcommand so a
// fresh install starts with a populated, discoverable catalog instead of
// an empty database where every discovery 404s (issue #1).
//
// The loader is idempotent: problem classes are upserted by title, and
// an answer is created only when the class has no existing answer with
// the same (env, lang, version, solution, created_at) tuple — answers
// without a corpus created_at dedup on the content tuple alone, so
// legacy files stay re-runnable. Re-running after a corpus sync
// therefore imports only the delta, never duplicates.
//
// Timestamps are preserved, not re-stamped (DF-OFF-BY-ONE-58): a new
// class or answer stores the corpus file's own created_at, and rows that
// already exist are never touched — so a re-run leaves every stored
// created_at byte-identical.
package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
)

// CorpusAnswer is one answer inside a corpus file. Field names match
// the on-disk JSON shape written by scripts/export-answers.py.
type CorpusAnswer struct {
	AnswerID    int64           `json:"answer_id"`
	Language    string          `json:"language"`
	Environment string          `json:"environment"`
	Version     string          `json:"version"`
	Solution    string          `json:"solution"`
	Evidence    string          `json:"evidence"`
	Signatures  json.RawMessage `json:"signatures"`
	Status      string          `json:"status"`
	CreatedAt   string          `json:"created_at"`
}

// CorpusFile is the on-disk shape of data/answers/<id>-<slug>.json.
type CorpusFile struct {
	ClassID     int64          `json:"class_id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	CreatedAt   string         `json:"created_at"`
	Answers     []CorpusAnswer `json:"answers"`
}

// Stats reports what a single Seed run did.
type Stats struct {
	FilesLoaded     int
	ClassesCreated  int
	ClassesExisting int
	AnswersCreated  int
	AnswersSkipped  int
	// EdgesCreated counts the `similar` graph edges the linker wrote after
	// the corpus files were loaded. Zero on a re-run — edges are keyed by
	// (source, target, relationship), so an existing edge is never rewritten.
	EdgesCreated int
}

// Linker defaults used by the corpus seed. Two shared title lexemes is the
// weakest overlap that still means something (one shared token is usually a
// generic word such as "python" or "drift"), and five neighbours keeps a
// class from being linked to the whole corpus through one broad term.
const (
	linkMinShared    = 2
	linkMaxNeighbors = 5
)

// Seed loads every *.json corpus file under <dir>/answers/ into the
// store. dir is the data directory (default ./data); the corpus itself
// lives in its answers/ subdirectory.
//
// Idempotent: classes are upserted by title, answers are deduplicated
// against existing rows by (env, lang, version, solution, created_at) —
// see seedFile for the legacy no-timestamp fallback. Each answer's
// status comes from its own corpus verdict (see corpusAnswerStatus) — a
// signature marked failed is imported as failed, everything else keeps
// the export's verified status.
//
// Once the files are loaded the classes are linked into the lateral
// `similar` graph (graph.LinkAllSimilar) — the corpus carries no error
// signatures, so title lexemes are the only relationship signal available,
// and without this step /problems/<class>/related answers [] on a
// perfectly seeded database.
//
// Batching (PERF-OFF-BY-ONE-1): the corpus writes run as ONE SQLite
// transaction instead of one autocommitted statement per row. Before
// batching, the release install seed paid an fsync per statement in WAL
// mode — ~9,100 fsyncs, 45-70s wall on ext4 for ~4s of work (the tmpfs
// control runs the identical code in 3.4-3.7s). The load now runs in two
// phases:
//
//  1. read + decode every corpus file into memory — filesystem I/O stays
//     OUTSIDE the write transaction, so the batch never holds the
//     database across disk reads;
//  2. one graph.Tx for every class upsert, answer insert and status
//     update, committed once — one fsync for the whole corpus.
//
// The transaction makes the corpus load atomic: a parse/validation/write
// failure or a cancelled context rolls the batch back and leaves the
// database untouched (before batching, a mid-corpus failure left the
// already-loaded prefix committed). Because the loader is idempotent,
// re-running after an aborted seed simply imports everything again —
// recovery semantics are unchanged. The linker still runs AFTER the
// commit in its own batched transaction (writeSimilarEdges), exactly as
// before: a linking failure still leaves the corpus committed, and a
// re-run still creates 0 edges.
func Seed(ctx context.Context, store *graph.Store, dir string) (*Stats, error) {
	answersDir := filepath.Join(dir, "answers")
	entries, err := os.ReadDir(answersDir)
	if err != nil {
		return nil, fmt.Errorf("read corpus dir %s: %w", answersDir, err)
	}

	// Phase 1 — filesystem. Read and decode every corpus file BEFORE the
	// write transaction opens. os.ReadDir returns entries sorted by name,
	// so the load order (and therefore row ID assignment) is identical to
	// the pre-batch loader.
	var files []parsedCorpusFile
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("load corpus: %w", err)
		}
		cf, err := parseCorpusFile(filepath.Join(answersDir, ent.Name()))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ent.Name(), err)
		}
		files = append(files, parsedCorpusFile{name: ent.Name(), cf: cf})
	}

	stats := &Stats{FilesLoaded: len(files)}

	// Phase 2 — one transaction for every corpus write. The deferred
	// rollback fires on any error path (write failure, cancelled context)
	// and is a no-op after a successful commit.
	tx, err := store.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin seed batch: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	for _, f := range files {
		if err := seedFile(ctx, tx, f.cf, stats); err != nil {
			return nil, fmt.Errorf("%s: %w", f.name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit seed batch: %w", err)
	}
	committed = true

	// Link the classes into the lateral graph once every file is loaded.
	// Linking happens after the loop (not per file) so a class can pair with
	// classes from files that sort later, and so the corpus is only scanned
	// once. Idempotent like the rest of the loader: a re-run creates 0 edges.
	edges, err := store.LinkAllSimilar(ctx, linkMinShared, linkMaxNeighbors)
	if err != nil {
		return nil, fmt.Errorf("link similar classes: %w", err)
	}
	stats.EdgesCreated = edges
	return stats, nil
}

// parsedCorpusFile is one corpus file read and decoded during phase 1,
// before the write transaction opens.
type parsedCorpusFile struct {
	name string
	cf   *CorpusFile
}

// parseCorpusFile reads and validates one corpus file. It performs no
// store I/O, so it can run outside the write transaction.
func parseCorpusFile(path string) (*CorpusFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}
	var cf CorpusFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	if cf.Title == "" {
		return nil, errors.New("corpus file has empty title")
	}
	return &cf, nil
}

// seedFile merges one parsed corpus file into the store through the
// batch transaction. New classes and answers carry the corpus's own
// created_at values (DF-OFF-BY-ONE-58); a corpus file with no created_at
// falls back to the column default inside the store.
func seedFile(ctx context.Context, tx *graph.Tx, cf *CorpusFile, stats *Stats) error {
	class, created, err := tx.UpsertProblemClassAt(ctx, cf.Title, cf.Description, cf.CreatedAt)
	if err != nil {
		return fmt.Errorf("upsert problem class: %w", err)
	}
	if created {
		stats.ClassesCreated++
	} else {
		stats.ClassesExisting++
	}

	// Dedup identity is the (env, lang, version, solution) tuple PLUS
	// created_at when the corpus carries one (DF-OFF-BY-ONE-58): the
	// corpus holds re-solve snapshots that share the content tuple and
	// differ only in when they were written — treating those as
	// duplicates would drop the newer snapshot and its timestamp. A
	// corpus answer with NO created_at falls back to the plain content
	// tuple, so legacy files stay idempotent against rows the schema
	// default stamped with a wall clock the corpus cannot know.
	//
	// The corpus has no stable answer IDs across databases (answer_id is
	// export-scoped), so content identity is what makes re-runs safe.
	existing, err := tx.ListAnswers(ctx, class.ID)
	if err != nil {
		return fmt.Errorf("list existing answers: %w", err)
	}
	seen4 := make(map[string]bool, len(existing))
	seen5 := make(map[string]bool, len(existing))
	for _, a := range existing {
		k4 := answerKey(a.Env, a.Lang, a.Version, a.Solution)
		seen4[k4] = true
		// The stored stamp round-trips through time.Time; every writer
		// (schema default, corpus import) uses the "2006-01-02 15:04:05"
		// layout, so the formatted value matches the corpus string
		// byte-for-byte.
		seen5[k4+"\x00"+a.CreatedAt.Format("2006-01-02 15:04:05")] = true
	}

	for _, ans := range cf.Answers {
		k4 := answerKey(ans.Environment, ans.Language, ans.Version, ans.Solution)
		k5 := k4 + "\x00" + ans.CreatedAt
		if (ans.CreatedAt == "" && seen4[k4]) || (ans.CreatedAt != "" && seen5[k5]) {
			// Skip path: an already-present answer keeps the status it
			// already has. This deliberately does NOT rewrite the row
			// (re-seeding is idempotent) — historical rows whose status
			// disagrees with their signature are reclassified by the
			// one-off migration, not by the seed. The stored created_at
			// is likewise left untouched (DF-OFF-BY-ONE-58).
			stats.AnswersSkipped++
			continue
		}
		sigs, err := marshalSignatures(ans.Signatures)
		if err != nil {
			return fmt.Errorf("answer %d signatures: %w", ans.AnswerID, err)
		}
		id, err := tx.CreateAnswerNodeAt(ctx, class.ID, 0,
			ans.Environment, ans.Language, ans.Version,
			ans.Solution, ans.Evidence, sigs, ans.CreatedAt)
		if err != nil {
			return fmt.Errorf("create answer node: %w", err)
		}
		if err := tx.UpdateAnswerStatus(ctx, id, corpusAnswerStatus(ans)); err != nil {
			return fmt.Errorf("set answer status: %w", err)
		}
		seen4[k4] = true
		seen5[k5] = true
		stats.AnswersCreated++
	}
	return nil
}

// corpusAnswerStatus maps one corpus answer to the status the store
// should record for it. The corpus verdict wins over the file's own
// status field:
//
//   - signatures result "failed" → failed. The flat corpus is
//     regenerated from the DB by scripts/export-answers.py, and
//     historical exports blanket-stamped every row verified, so stored
//     answers exist with status="verified" alongside a failed
//     signature (OB-GAP-057). Trusting the verified stamp would
//     re-import those failed solves as verified on every fresh install.
//   - otherwise honour ans.Status when it is one of the store's valid
//     status constants.
//   - otherwise default to verified: legacy corpus files carry no
//     status field at all and must still seed as verified.
func corpusAnswerStatus(ans CorpusAnswer) string {
	if signatureResultFailed(ans.Signatures) {
		return graph.AnswerFailed
	}
	switch ans.Status {
	case graph.AnswerVerified, graph.AnswerCIPassed, graph.AnswerFailed, graph.AnswerPending:
		return ans.Status
	}
	return graph.AnswerVerified
}

// signatureResultFailed reports whether a corpus answer's signatures
// JSON carries result="failed". Absent, empty, or malformed signatures
// are not a failure verdict.
func signatureResultFailed(raw json.RawMessage) bool {
	if len(raw) == 0 {
		return false
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return false
	}
	res, ok := m["result"].(string)
	return ok && res == graph.AnswerFailed
}

// answerKey is the dedup identity for one answer row.
func answerKey(env, lang, version, solution string) string {
	return env + "\x00" + lang + "\x00" + version + "\x00" + solution
}

// marshalSignatures normalizes the corpus signatures field into the
// JSON string the graph column expects. The corpus stores signatures as
// an object; missing/null values become "{}".
func marshalSignatures(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "{}", nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	if len(m) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
