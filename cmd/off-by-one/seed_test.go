package main

import (
	"os"
	"path/filepath"
	"testing"
)

// writeSeedFixture creates a tiny corpus dir (dir/answers/*.json) with a
// single minimal answer file — enough for seed.Seed to run without
// importing the ~1000-class real corpus.
func writeSeedFixture(t *testing.T) string {
	t.Helper()
	fixture := t.TempDir()
	answersDir := filepath.Join(fixture, "answers")
	if err := os.MkdirAll(answersDir, 0o755); err != nil {
		t.Fatalf("mkdir answers dir: %v", err)
	}
	corpus := `{
  "class_id": 1,
  "title": "test class",
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
	if err := os.WriteFile(filepath.Join(answersDir, "0001-test.json"), []byte(corpus), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return fixture
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
