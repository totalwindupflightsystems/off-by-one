// seed_record.go — Prometheus textfile-format completion record for the
// `off-by-one seed` subcommand (OBS-OB-005, docs/observability-spec.md
// §5).
//
// Why a completion record at all: `seed` is a one-shot process. Every
// other obo_* family lives in the long-running API process, so a seed
// that failed at install time leaves no trace an operator can scrape —
// the only evidence was a line on a terminal nobody kept. The record
// makes the last run's outcome a metric: a node_exporter
// textfile-collector (or `promtool check metrics`) reads this file and
// the fleet sees obo_seed_last_exit_status without the product owning
// any transport (spec §5).
//
// The file is REWRITTEN on every run (never appended): a textfile is a
// snapshot of state, so an append would leave duplicate series and a
// stale success line next to a fresh failure.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/metrics"
	"github.com/totalwindupflightsystems/off-by-one/internal/seed"
)

// seedRecordDefaultName is the completion-record filename. It is
// written next to the SQLite database (so `-db ./off-by-one.db` writes
// ./obo_seed_completion.prom) unless -record or
// OFF_BY_ONE_SEED_RECORD names another path.
const seedRecordDefaultName = "obo_seed_completion.prom"

// seedRecordEnv is the environment variable that overrides the record
// path (it is the -record flag's default, so an explicit flag still
// wins).
const seedRecordEnv = "OFF_BY_ONE_SEED_RECORD"

// resolveSeedRecordPath decides where the completion record is written.
//
// Precedence: an explicit -record (or OFF_BY_ONE_SEED_RECORD, which is
// the flag's default) wins; otherwise <dir of the DB path>/
// obo_seed_completion.prom. The literal value "-" disables the record —
// an escape hatch for a caller that must not write files, kept explicit
// rather than folding it into an empty string so "" stays a mistake
// worth reporting.
func resolveSeedRecordPath(explicit, dbPath string) string {
	if trimmed := strings.TrimSpace(explicit); trimmed != "" {
		if trimmed == "-" {
			return ""
		}
		return filepath.Clean(trimmed)
	}
	db := strings.TrimSpace(dbPath)
	if db == "" {
		// No DB path at all: the historical default lives in the CWD.
		return seedRecordDefaultName
	}
	dir := filepath.Dir(filepath.Clean(db))
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, seedRecordDefaultName)
}

// seedCompletion is one run's observation set.
type seedCompletion struct {
	// ExitStatus is what the command will exit with: 0 on success, 1 on
	// any error path (the flag-parse path exits 2 before this struct is
	// built — see seedRun).
	ExitStatus int
	// FinishedAt is when the run completed; the record carries it as
	// fractional unix seconds, the shape Prometheus timestamps use.
	FinishedAt time.Time
	// Stats is nil when the run failed before the corpus load returned
	// statistics — every count then reads 0 while ExitStatus says why.
	Stats *seed.Stats
}

// seedRecordLabelSet renders the product/service const labels every
// obo_* series carries (OBS-OB-005), sorted the way the exposition
// format requires. The values are compile-time constants with no
// backslash/quote characters, so %q is a correct Prometheus label
// escape for them.
func seedRecordLabelSet() string {
	names := make([]string, 0, len(metrics.PromConstLabels))
	for name := range metrics.PromConstLabels {
		names = append(names, name)
	}
	sort.Strings(names)

	var b strings.Builder
	for i, name := range names {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, "%s=%q", name, metrics.PromConstLabels[name])
	}
	return b.String()
}

// seedCompletionText renders the record as Prometheus text exposition
// (version 0.0.4), the format a textfile collector ingests.
func seedCompletionText(rec seedCompletion) string {
	labels := seedRecordLabelSet()
	var stats seed.Stats
	if rec.Stats != nil {
		stats = *rec.Stats
	}
	family := func(name, help, typ string, value int64) string {
		var b strings.Builder
		fmt.Fprintf(&b, "# HELP %s %s\n", name, help)
		fmt.Fprintf(&b, "# TYPE %s %s\n", name, typ)
		fmt.Fprintf(&b, "%s{%s} %d\n", name, labels, value)
		return b.String()
	}

	var b strings.Builder
	b.WriteString(family("obo_seed_last_exit_status",
		"Exit status of the most recently completed off-by-one seed run: 0 = success, 1 = failed (the flag-parse path exits 2 before a record is written).",
		"gauge", int64(rec.ExitStatus)))
	fmt.Fprintf(&b, "# HELP obo_seed_last_run_timestamp_seconds Unix seconds (fractional) when the most recent seed run completed, whatever its outcome.\n")
	fmt.Fprintf(&b, "# TYPE obo_seed_last_run_timestamp_seconds gauge\n")
	fmt.Fprintf(&b, "obo_seed_last_run_timestamp_seconds{%s} %.3f\n", labels,
		float64(rec.FinishedAt.UnixNano())/1e9)
	b.WriteString(family("obo_seed_files_loaded",
		"Corpus files (data/answers/*.json) read by the most recent successful seed run.",
		"gauge", int64(stats.FilesLoaded)))
	b.WriteString(family("obo_seed_classes_loaded",
		"Problem classes the catalogue holds after the most recent seed run (created + already existing).",
		"gauge", int64(stats.ClassesCreated+stats.ClassesExisting)))
	b.WriteString(family("obo_seed_answers_loaded",
		"Answers the catalogue holds after the most recent seed run (created + already present).",
		"gauge", int64(stats.AnswersCreated+stats.AnswersSkipped)))
	b.WriteString(family("obo_seed_answers_created",
		"Answers newly inserted by the most recent seed run (a re-run of an unchanged corpus creates 0).",
		"gauge", int64(stats.AnswersCreated)))
	b.WriteString(family("obo_seed_answers_skipped",
		"Answers already present and left untouched by the most recent seed run.",
		"gauge", int64(stats.AnswersSkipped)))
	b.WriteString(family("obo_seed_edges_created",
		"Similar-graph edges written by the most recent seed run (0 on a re-run: edges are never rewritten).",
		"gauge", int64(stats.EdgesCreated)))
	return b.String()
}

// writeSeedCompletion writes the record atomically: a temp file in the
// destination directory, then a rename, so a reader never observes a
// half-written textfile (the textfile collector tolerates a missing
// file but not a truncated one full of parse errors). The destination
// directory must already exist — the DB lives there, so inventing it
// would mask a wrong -db path.
func writeSeedCompletion(path string, rec seedCompletion) error {
	if path == "" {
		return nil // explicitly disabled
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".obo-seed-record-*.prom")
	if err != nil {
		return fmt.Errorf("create temp file in %s: %w", dir, err)
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}
	if _, err := tmp.WriteString(seedCompletionText(rec)); err != nil {
		cleanup()
		return fmt.Errorf("write temp file %s: %w", tmpName, err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("close temp file %s: %w", tmpName, err)
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("chmod temp file %s: %w", tmpName, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("rename %s to %s: %w", tmpName, path, err)
	}
	return nil
}
