// Package main — `off-by-one seed` subcommand implementation.
//
// The seed subcommand is dispatched from main() before the server flag
// set is parsed; it owns its own FlagSet so it never collides with
// server flags.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/seed"
)

// seedDirDefault is the CWD-relative corpus directory that has always
// been the `-dir` default. It stays the first fallback candidate so a
// repo-local `off-by-one seed` behaves exactly as it did before.
const seedDirDefault = "./data"

// Indirect seams: the resolver needs the working directory and the
// executable path, and tests need to drive both without chdir/exec.
var (
	seedGetwd      = os.Getwd
	seedExecutable = os.Executable
)

// isCorpusRoot reports whether dir looks like a corpus data root, i.e.
// it holds an answers/ subdirectory (the only thing seed.Seed reads).
func isCorpusRoot(dir string) bool {
	if dir == "" {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, "answers"))
	return err == nil && info.IsDir()
}

// seedDirCandidates returns the default corpus candidates in precedence
// order, cleaned, deduplicated and with empty entries dropped:
//
//  1. <cwd>/data        — the historical repo-relative default
//  2. <exeDir>/data     — corpus shipped next to the binary
//  3. <exeDir>/../data  — bin/ layout (binary in <app>/bin, data in <app>/data)
//
// A missing cwd or executable path simply contributes no candidates.
func seedDirCandidates(cwd, exePath string) []string {
	raw := make([]string, 0, 3)
	if cwd != "" {
		raw = append(raw, filepath.Join(cwd, "data"))
	}
	if exePath != "" {
		exeDir := filepath.Dir(exePath)
		raw = append(raw, filepath.Join(exeDir, "data"), filepath.Join(exeDir, "..", "data"))
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, cand := range raw {
		if cand == "" {
			continue
		}
		clean := filepath.Clean(cand)
		if _, dup := seen[clean]; dup {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

// resolveSeedDir picks the corpus data directory for `off-by-one seed`.
//
// An explicit -dir always wins: when it is given, NO other candidate is
// considered — an invalid explicit path fails loudly (naming that path)
// instead of silently seeding from a different corpus. With no explicit
// -dir the first candidate from seedDirCandidates that holds an answers/
// subdirectory is used; if none does, the error lists every attempted
// path so the operator knows what to pass to -dir.
func resolveSeedDir(explicit, cwd, exePath string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		dir := filepath.Clean(explicit)
		if !isCorpusRoot(dir) {
			return "", fmt.Errorf("explicit -dir %s has no answers/ subdirectory (looked for %s); pass -dir DIR pointing at a corpus root — no fallback is attempted when -dir is given",
				dir, filepath.Join(dir, "answers"))
		}
		return dir, nil
	}

	candidates := seedDirCandidates(cwd, exePath)
	for _, dir := range candidates {
		if isCorpusRoot(dir) {
			return dir, nil
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("corpus directory not found: could not determine a working " +
			"directory or executable path; pass -dir DIR")
	}
	return "", fmt.Errorf("corpus directory not found: no answers/ subdirectory in any of %s; pass -dir DIR",
		strings.Join(candidates, ", "))
}

// runSeed implements `off-by-one seed` — a one-shot loader that merges
// the bundled flat answer corpus (data/answers/*.json) into the SQLite
// graph store. Fresh installs run it once after building so discovery
// works immediately instead of 404ing on an empty database (issue #1).
// Idempotent: re-running imports only the corpus delta.
func runSeed(args []string) {
	if err := seedRun(args); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

// seedRun is runSeed's testable core. It resolves the corpus directory
// BEFORE opening the graph store, so a missing default corpus cannot
// leave a misleading empty database behind.
func seedRun(args []string) error {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	dir := fs.String("dir", seedDirDefault, "Corpus data directory (contains answers/*.json); default ./data, else next to the executable")
	dbPath := fs.String("db", envString("OFF_BY_ONE_DB", "./off-by-one.db"), "SQLite database path")
	record := fs.String("record", envString(seedRecordEnv, ""), "Prometheus textfile-format completion record path (default <dir of -db>/"+seedRecordDefaultName+"; \"-\" disables)")
	fs.Usage = func() {
		_, _ = fmt.Fprintf(fs.Output(), "Usage: off-by-one seed [-dir DIR] [-db DB] [-record PATH]\n\n")
		_, _ = fmt.Fprintf(fs.Output(), "Loads the bundled flat answer corpus (DIR/answers/*.json) into the\n")
		_, _ = fmt.Fprintf(fs.Output(), "SQLite graph store so fresh installs start with a discoverable catalog.\n")
		_, _ = fmt.Fprintf(fs.Output(), "Idempotent — safe to re-run; only the corpus delta is imported.\n\n")
		_, _ = fmt.Fprintf(fs.Output(), "Without -dir the corpus is looked up relative to the working directory\n")
		_, _ = fmt.Fprintf(fs.Output(), "(./data), then relative to the executable (./data, ./../data).\n\n")
		_, _ = fmt.Fprintf(fs.Output(), "On completion the run writes a Prometheus textfile-format record holding\n")
		_, _ = fmt.Fprintf(fs.Output(), "its exit status, a timestamp and the classes/answers loaded, for a\n")
		_, _ = fmt.Fprintf(fs.Output(), "textfile collector to scrape (docs/observability-spec.md §5).\n\n")
		_, _ = fmt.Fprintf(fs.Output(), "Flags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	// Only an explicitly passed -dir counts as explicit; the flag's
	// default is a fallback candidate, not an override.
	explicitDir := ""
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "dir" {
			explicitDir = *dir
		}
	})

	cwd, _ := seedGetwd() // best-effort: no cwd simply drops that candidate
	exePath, _ := seedExecutable()

	// The completion record is written on BOTH outcomes: a failed seed
	// is exactly what an operator needs to see as a metric, and it is
	// the only durable trace a one-shot process leaves. The path is
	// resolved before the load so a record is written even when the
	// load never opens the DB.
	recordPath := resolveSeedRecordPath(*record, *dbPath)

	stats, err := seedLoad(explicitDir, *dbPath, cwd, exePath)

	rec := seedCompletion{ExitStatus: 0, FinishedAt: time.Now().UTC(), Stats: stats}
	if err != nil {
		rec.ExitStatus = 1
	}
	if werr := writeSeedCompletion(recordPath, rec); werr != nil {
		// A record-write failure must not fail a successful seed (the
		// corpus is already committed), but it must never be silent —
		// the operator's only signal is this line.
		log.Printf("seed: completion record %s not written: %v", recordPath, werr)
	}

	if err != nil {
		return err
	}
	log.Printf("seed complete: files=%d; classes=%d created / %d existing; answers=%d created / %d skipped; edges=%d created (db=%s, record=%s)",
		stats.FilesLoaded, stats.ClassesCreated, stats.ClassesExisting,
		stats.AnswersCreated, stats.AnswersSkipped, stats.EdgesCreated, *dbPath, recordPath)
	return nil
}

// seedLoad is the loading half of seedRun: resolve the corpus, open the
// graph store, run the seed. A failure returns nil statistics (the seed
// package does the same), so the completion record for a failed run
// carries zero counts plus a non-zero exit status.
func seedLoad(explicitDir, dbPath, cwd, exePath string) (*seed.Stats, error) {
	corpusDir, err := resolveSeedDir(explicitDir, cwd, exePath)
	if err != nil {
		return nil, err
	}

	store, err := graph.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open graph store: %w", err)
	}
	defer func() {
		if cerr := store.Close(); cerr != nil {
			log.Printf("seed: close graph store: %v", cerr)
		}
	}()

	return seed.Seed(context.Background(), store, corpusDir)
}
