// Package seed loads the bundled flat answer corpus (data/answers/*.json)
// into the graph store. It powers the `off-by-one seed` subcommand so a
// fresh install starts with a populated, discoverable catalog instead of
// an empty database where every discovery 404s (issue #1).
//
// The loader is idempotent: problem classes are upserted by title, and
// an answer is created only when the class has no existing answer with
// the same (env, lang, version, solution) tuple. Re-running after a
// corpus sync therefore imports only the delta, never duplicates.
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
}

// Seed loads every *.json corpus file under <dir>/answers/ into the
// store. dir is the data directory (default ./data); the corpus itself
// lives in its answers/ subdirectory.
//
// Idempotent: classes are upserted by title, answers are deduplicated
// against existing rows by (env, lang, version, solution). Seeded
// answers are marked verified — the corpus is the verified-answer
// export, so it carries the solver's verified status on import.
func Seed(ctx context.Context, store *graph.Store, dir string) (*Stats, error) {
	answersDir := filepath.Join(dir, "answers")
	entries, err := os.ReadDir(answersDir)
	if err != nil {
		return nil, fmt.Errorf("read corpus dir %s: %w", answersDir, err)
	}
	stats := &Stats{}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".json") {
			continue
		}
		if err := seedFile(ctx, store, filepath.Join(answersDir, ent.Name()), stats); err != nil {
			return nil, fmt.Errorf("%s: %w", ent.Name(), err)
		}
	}
	return stats, nil
}

// seedFile loads one corpus file and merges it into the store.
func seedFile(ctx context.Context, store *graph.Store, path string, stats *Stats) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}
	var cf CorpusFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if cf.Title == "" {
		return errors.New("corpus file has empty title")
	}
	stats.FilesLoaded++

	class, created, err := store.UpsertProblemClass(ctx, cf.Title, cf.Description)
	if err != nil {
		return fmt.Errorf("upsert problem class: %w", err)
	}
	if created {
		stats.ClassesCreated++
	} else {
		stats.ClassesExisting++
	}

	// Dedup key is the full (env, lang, version, solution) tuple. The
	// corpus has no stable answer IDs across databases (answer_id is
	// export-scoped), so content identity is what makes re-runs safe.
	existing, err := store.ListAnswers(ctx, class.ID)
	if err != nil {
		return fmt.Errorf("list existing answers: %w", err)
	}
	seen := make(map[string]bool, len(existing))
	for _, a := range existing {
		seen[answerKey(a.Env, a.Lang, a.Version, a.Solution)] = true
	}

	for _, ans := range cf.Answers {
		key := answerKey(ans.Environment, ans.Language, ans.Version, ans.Solution)
		if seen[key] {
			stats.AnswersSkipped++
			continue
		}
		sigs, err := marshalSignatures(ans.Signatures)
		if err != nil {
			return fmt.Errorf("answer %d signatures: %w", ans.AnswerID, err)
		}
		id, err := store.CreateAnswerNode(ctx, class.ID, 0,
			ans.Environment, ans.Language, ans.Version,
			ans.Solution, ans.Evidence, sigs)
		if err != nil {
			return fmt.Errorf("create answer node: %w", err)
		}
		if err := store.UpdateAnswerStatus(ctx, id, graph.AnswerVerified); err != nil {
			return fmt.Errorf("mark answer verified: %w", err)
		}
		seen[key] = true
		stats.AnswersCreated++
	}
	return nil
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
