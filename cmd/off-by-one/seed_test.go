package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
)

// seedCorpusJSON is one minimal corpus file — enough for seed.Seed to run
// without importing the ~1000-class real corpus. %q is the class title.
const seedCorpusJSON = `{
  "class_id": 1,
  "title": %q,
  "description": "seed fixture",
  "created_at": "2026-08-16 00:00:00",
  "answers": [
    {
      "answer_id": 1,
      "language": "go",
      "environment": "docker",
      "version": "latest",
      "solution": "fix: return the error",
      "evidence": "verified",
      "signatures": {},
      "status": "verified",
      "created_at": "2026-08-16 00:00:00"
    }
  ]
}`

// writeSeedFixture creates a tiny corpus dir (dir/answers/*.json) with a
// single minimal answer file.
func writeSeedFixture(t *testing.T) string {
	t.Helper()
	return writeCorpusRoot(t, t.TempDir())
}

// writeCorpusRoot creates a valid corpus root (root/answers/*.json) under
// root and returns root. The class title identifies which corpus a run
// actually loaded.
func writeCorpusRoot(t *testing.T, root string) string {
	t.Helper()
	return writeCorpusRootNamed(t, root, "test class")
}

func writeCorpusRootNamed(t *testing.T, root, title string) string {
	t.Helper()
	answersDir := filepath.Join(root, "answers")
	if err := os.MkdirAll(answersDir, 0o755); err != nil {
		t.Fatalf("mkdir answers dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(answersDir, "0001-test.json"), []byte(fmt.Sprintf(seedCorpusJSON, title)), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return root
}

// stubSeedPaths replaces the cwd/executable seams for one test.
func stubSeedPaths(t *testing.T, cwd, exe string) {
	t.Helper()
	origGetwd, origExe := seedGetwd, seedExecutable
	t.Cleanup(func() { seedGetwd, seedExecutable = origGetwd, origExe })
	seedGetwd = func() (string, error) { return cwd, nil }
	seedExecutable = func() (string, error) { return exe, nil }
}

// classExists reports whether the seeded database holds a problem class
// with the given title — proof of WHICH corpus was loaded.
func classExists(t *testing.T, dbPath, title string) bool {
	t.Helper()
	store, err := graph.Open(dbPath)
	if err != nil {
		t.Fatalf("open seeded db %s: %v", dbPath, err)
	}
	defer func() {
		if cerr := store.Close(); cerr != nil {
			t.Fatalf("close seeded db: %v", cerr)
		}
	}()
	class, err := store.GetProblemClassByTitle(context.Background(), title)
	if err != nil {
		return false
	}
	return class != nil
}

// TestRunSeedHonorsEnvVar proves that `seed` resolves the DB path from
// OFF_BY_ONE_DB when no -db flag is passed (OB-GAP-039 regression):
// the seeded database must land at the env path, not ./off-by-one.db.
func TestRunSeedHonorsEnvVar(t *testing.T) {
	fixture := writeSeedFixture(t)
	envDB := filepath.Join(t.TempDir(), "env-seeded.db")
	t.Setenv("OFF_BY_ONE_DB", envDB)

	// runSeed closes the graph store before returning, so the file is
	// safe to stat afterwards.
	runSeed([]string{"-dir", fixture})

	if st, err := os.Stat(envDB); err != nil {
		t.Fatalf("OFF_BY_ONE_DB path %s not created: %v", envDB, err)
	} else if st.Size() == 0 {
		t.Fatalf("OFF_BY_ONE_DB path %s created but empty", envDB)
	}

	// Old behavior seeded ./off-by-one.db in the cwd; make sure the
	// default was NOT used. (Clean any stray file from a failed run.)
	if err := os.Remove("./off-by-one.db"); err == nil {
		t.Fatal("default ./off-by-one.db was created — env var ignored")
	}
}

// TestRunSeedDBFlagOverridesEnvVar proves flag precedence: an explicit
// -db flag beats OFF_BY_ONE_DB (the env var is only the flag default).
func TestRunSeedDBFlagOverridesEnvVar(t *testing.T) {
	fixture := writeSeedFixture(t)
	envDB := filepath.Join(t.TempDir(), "env-ignored.db")
	flagDB := filepath.Join(t.TempDir(), "flag-seeded.db")
	t.Setenv("OFF_BY_ONE_DB", envDB)

	runSeed([]string{"-dir", fixture, "-db", flagDB})

	if st, err := os.Stat(flagDB); err != nil {
		t.Fatalf("-db path %s not created: %v", flagDB, err)
	} else if st.Size() == 0 {
		t.Fatalf("-db path %s created but empty", flagDB)
	}
	if _, err := os.Stat(envDB); err == nil {
		t.Fatalf("OFF_BY_ONE_DB path %s was created despite -db flag", envDB)
	}
}

// TestResolveSeedDirExplicitWins proves an explicit -dir is used verbatim
// and never falls back — including when it is invalid and a perfectly good
// corpus sits in the cwd (acceptance B).
func TestResolveSeedDirExplicitWins(t *testing.T) {
	cwd := filepath.Join(t.TempDir(), "cwd")
	writeCorpusRoot(t, filepath.Join(cwd, "data"))
	missing := filepath.Join(t.TempDir(), "nope")

	_, err := resolveSeedDir(missing, cwd, filepath.Join(cwd, "off-by-one"))
	if err == nil {
		t.Fatal("resolveSeedDir(explicit invalid): expected an error")
	}
	if !strings.Contains(err.Error(), missing) {
		t.Fatalf("error %q does not name the explicit dir %s", err, missing)
	}
	if strings.Contains(err.Error(), filepath.Join(cwd, "data")) {
		t.Fatalf("error %q leaked a fallback candidate — explicit -dir must not fall back", err)
	}

	explicitRoot := writeCorpusRoot(t, t.TempDir())
	got, err := resolveSeedDir(explicitRoot, cwd, filepath.Join(cwd, "off-by-one"))
	if err != nil {
		t.Fatalf("resolveSeedDir(explicit valid): unexpected error %v", err)
	}
	if got != explicitRoot {
		t.Fatalf("explicit -dir lost to cwd corpus: got %q want %q", got, explicitRoot)
	}
}

// TestResolveSeedDirPrefersCWD proves the historical ./data default still
// wins when it is a valid corpus root, ahead of any executable-relative
// candidate.
func TestResolveSeedDirPrefersCWD(t *testing.T) {
	cwd := filepath.Join(t.TempDir(), "cwd")
	want := writeCorpusRoot(t, filepath.Join(cwd, "data"))
	appRoot := t.TempDir()
	writeCorpusRoot(t, filepath.Join(appRoot, "data"))

	got, err := resolveSeedDir("", cwd, filepath.Join(appRoot, "off-by-one"))
	if err != nil {
		t.Fatalf("resolveSeedDir: unexpected error %v", err)
	}
	if got != want {
		t.Fatalf("cwd corpus not preferred: got %q want %q", got, want)
	}
}

// TestResolveSeedDirExecutableFallback proves a foreign cwd falls back to
// the corpus shipped next to the binary, and to <exeDir>/../data for a
// bin/ layout.
func TestResolveSeedDirExecutableFallback(t *testing.T) {
	foreignCWD := t.TempDir() // no data/ in here

	t.Run("next to executable", func(t *testing.T) {
		appRoot := t.TempDir()
		want := writeCorpusRoot(t, filepath.Join(appRoot, "data"))
		got, err := resolveSeedDir("", foreignCWD, filepath.Join(appRoot, "off-by-one"))
		if err != nil {
			t.Fatalf("resolveSeedDir: unexpected error %v", err)
		}
		if got != want {
			t.Fatalf("exe-adjacent corpus not found: got %q want %q", got, want)
		}
	})

	t.Run("bin layout", func(t *testing.T) {
		appRoot := t.TempDir()
		want := writeCorpusRoot(t, filepath.Join(appRoot, "data"))
		got, err := resolveSeedDir("", foreignCWD, filepath.Join(appRoot, "bin", "off-by-one"))
		if err != nil {
			t.Fatalf("resolveSeedDir: unexpected error %v", err)
		}
		if got != want {
			t.Fatalf("bin-layout corpus not found: got %q want %q", got, want)
		}
	})
}

// TestResolveSeedDirErrorListsAttempts proves the failure contract: the
// error names every attempted path and tells the operator to pass -dir.
func TestResolveSeedDirErrorListsAttempts(t *testing.T) {
	cwd, appRoot := t.TempDir(), t.TempDir()
	exe := filepath.Join(appRoot, "bin", "off-by-one")

	_, err := resolveSeedDir("", cwd, exe)
	if err == nil {
		t.Fatal("resolveSeedDir: expected error when no candidate is a corpus root")
	}
	for _, want := range []string{
		filepath.Join(cwd, "data"),
		filepath.Join(appRoot, "bin", "data"),
		filepath.Join(appRoot, "data"),
		"-dir",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
}

// TestResolveSeedDirCleansAndDedupes proves duplicate candidates collapse
// (cwd == exeDir) and paths are cleaned.
func TestResolveSeedDirCleansAndDedupes(t *testing.T) {
	cwd := t.TempDir()
	got := seedDirCandidates(cwd, filepath.Join(cwd, "off-by-one"))

	want := []string{filepath.Join(cwd, "data"), filepath.Join(filepath.Dir(cwd), "data")}
	if len(got) != len(want) {
		t.Fatalf("candidates not deduped: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("candidate %d: got %q want %q", i, got[i], want[i])
		}
	}
	for _, c := range got {
		if c != filepath.Clean(c) {
			t.Fatalf("candidate %q is not clean", c)
		}
	}
}

// TestSeedRunResolvesCorpusFromExecutableDir proves the resolution is
// wired into the command: with a foreign cwd and no -dir, the corpus next
// to the executable is loaded (acceptance A, minus the process spawn).
func TestSeedRunResolvesCorpusFromExecutableDir(t *testing.T) {
	appRoot := t.TempDir()
	fixture := writeCorpusRootNamed(t, filepath.Join(appRoot, "data"), "exe corpus class")
	stubSeedPaths(t, t.TempDir(), filepath.Join(appRoot, "bin", "off-by-one"))

	dbPath := filepath.Join(t.TempDir(), "exec-seeded.db")
	if err := seedRun([]string{"-db", dbPath}); err != nil {
		t.Fatalf("seedRun from foreign cwd: %v", err)
	}
	if st, err := os.Stat(dbPath); err != nil {
		t.Fatalf("db %s not created: %v", dbPath, err)
	} else if st.Size() == 0 {
		t.Fatalf("db %s created but empty", dbPath)
	}
	if !classExists(t, dbPath, "exe corpus class") {
		t.Fatalf("seeded db %s does not hold the exe-relative corpus (fixture %s)", dbPath, fixture)
	}
}

// TestSeedRunCWDCorpusBeatsExecutableCorpus proves precedence end to end:
// with two valid corpora, the cwd one is the one that lands in the DB.
func TestSeedRunCWDCorpusBeatsExecutableCorpus(t *testing.T) {
	cwdRoot := filepath.Join(t.TempDir(), "cwd")
	writeCorpusRootNamed(t, filepath.Join(cwdRoot, "data"), "cwd corpus class")
	appRoot := t.TempDir()
	writeCorpusRootNamed(t, filepath.Join(appRoot, "data"), "exe corpus class")
	stubSeedPaths(t, cwdRoot, filepath.Join(appRoot, "bin", "off-by-one"))

	dbPath := filepath.Join(t.TempDir(), "precedence.db")
	if err := seedRun([]string{"-db", dbPath}); err != nil {
		t.Fatalf("seedRun: %v", err)
	}
	if !classExists(t, dbPath, "cwd corpus class") {
		t.Fatalf("seeded db %s missing the cwd corpus", dbPath)
	}
	if classExists(t, dbPath, "exe corpus class") {
		t.Fatalf("seeded db %s holds the exe corpus — cwd corpus did not win", dbPath)
	}
}

// TestSeedRunMissingCorpusCreatesNoDB proves acceptance C: when no default
// candidate is a corpus root the command fails, names every attempted path,
// advises -dir, and leaves no misleading empty database behind.
func TestSeedRunMissingCorpusCreatesNoDB(t *testing.T) {
	cwd := t.TempDir()
	appRoot := t.TempDir()
	stubSeedPaths(t, cwd, filepath.Join(appRoot, "bin", "off-by-one"))

	dbPath := filepath.Join(t.TempDir(), "must-not-exist.db")
	err := seedRun([]string{"-db", dbPath})
	if err == nil {
		t.Fatal("seedRun: expected an error when no default corpus exists")
	}
	for _, want := range []string{
		filepath.Join(cwd, "data"),
		filepath.Join(appRoot, "bin", "data"),
		filepath.Join(appRoot, "data"),
		"-dir",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not mention %q", err, want)
		}
	}
	if _, statErr := os.Stat(dbPath); statErr == nil {
		t.Fatalf("database %s was created despite corpus resolution failure", dbPath)
	}
}

// TestSeedRunInvalidExplicitDirDoesNotFallBack proves acceptance B at the
// command level: an invalid -dir fails against THAT path instead of
// quietly seeding from the valid cwd corpus.
func TestSeedRunInvalidExplicitDirDoesNotFallBack(t *testing.T) {
	cwdRoot := filepath.Join(t.TempDir(), "cwd")
	writeCorpusRootNamed(t, filepath.Join(cwdRoot, "data"), "cwd corpus class")
	stubSeedPaths(t, cwdRoot, filepath.Join(cwdRoot, "off-by-one"))

	missing := filepath.Join(t.TempDir(), "does-not-exist")
	dbPath := filepath.Join(t.TempDir(), "must-not-exist.db")
	err := seedRun([]string{"-dir", missing, "-db", dbPath})
	if err == nil {
		t.Fatal("seedRun: expected an error for an invalid explicit -dir")
	}
	if !strings.Contains(err.Error(), filepath.Join(missing, "answers")) {
		t.Fatalf("error %q does not name the explicit dir %s", err, missing)
	}
	if _, statErr := os.Stat(dbPath); statErr == nil {
		t.Fatalf("database %s was created despite an invalid explicit -dir", dbPath)
	}
}
