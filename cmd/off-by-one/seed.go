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

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/seed"
)

// runSeed implements `off-by-one seed` — a one-shot loader that merges
// the bundled flat answer corpus (data/answers/*.json) into the SQLite
// graph store. Fresh installs run it once after building so discovery
// works immediately instead of 404ing on an empty database (issue #1).
// Idempotent: re-running imports only the corpus delta.
func runSeed(args []string) {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	dir := fs.String("dir", "./data", "Corpus data directory (contains answers/*.json)")
	dbPath := fs.String("db", envString("OFF_BY_ONE_DB", "./off-by-one.db"), "SQLite database path")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: off-by-one seed [-dir DIR] [-db DB]\n\n")
		fmt.Fprintf(fs.Output(), "Loads the bundled flat answer corpus (DIR/answers/*.json) into the\n")
		fmt.Fprintf(fs.Output(), "SQLite graph store so fresh installs start with a discoverable catalog.\n")
		fmt.Fprintf(fs.Output(), "Idempotent — safe to re-run; only the corpus delta is imported.\n\n")
		fmt.Fprintf(fs.Output(), "Flags:\n")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}

	store, err := graph.Open(*dbPath)
	if err != nil {
		log.Fatalf("seed: open graph store: %v", err)
	}
	defer func() {
		if cerr := store.Close(); cerr != nil {
			log.Printf("seed: close graph store: %v", cerr)
		}
	}()

	stats, err := seed.Seed(context.Background(), store, *dir)
	if err != nil {
		log.Fatalf("seed: %v", err)
	}
	log.Printf("seed complete: files=%d; classes=%d created / %d existing; answers=%d created / %d skipped (db=%s)",
		stats.FilesLoaded, stats.ClassesCreated, stats.ClassesExisting,
		stats.AnswersCreated, stats.AnswersSkipped, *dbPath)
}
