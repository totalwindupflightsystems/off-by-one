package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// parseSeedRecord parses a completion record with the real Prometheus
// text parser: a record that does not parse is not a textfile, however
// good its numbers are.
func parseSeedRecord(t *testing.T, path string) map[string]*dto.MetricFamily {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open record %s: %v", path, err)
	}
	t.Cleanup(func() { _ = f.Close() })

	parser := expfmt.NewTextParser(model.LegacyValidation)
	fams, err := parser.TextToMetricFamilies(f)
	if err != nil {
		t.Fatalf("record %s is not parseable Prometheus text: %v", path, err)
	}
	if len(fams) == 0 {
		t.Fatalf("record %s parsed to zero families", path)
	}
	return fams
}

// seedRecordSample returns the single sample of a record family and
// asserts it carries the product/service const labels (the record is
// part of the obo_* series set, so OBS-OB-005 applies to it too).
func seedRecordSample(t *testing.T, fams map[string]*dto.MetricFamily, name string) float64 {
	t.Helper()
	fam, ok := fams[name]
	if !ok {
		t.Fatalf("record is missing family %s", name)
	}
	if len(fam.Metric) != 1 {
		t.Fatalf("family %s has %d samples, want exactly 1 (records are snapshots, never appended)", name, len(fam.Metric))
	}
	m := fam.Metric[0]
	labels := map[string]string{}
	for _, lp := range m.Label {
		labels[lp.GetName()] = lp.GetValue()
	}
	if labels["product"] != "off-by-one" || labels["service"] != "off-by-one" {
		t.Errorf("%s labels %v: want product=service=off-by-one", name, labels)
	}
	if m.Gauge == nil {
		t.Fatalf("family %s is not a gauge", name)
	}
	return m.Gauge.GetValue()
}

func TestResolveSeedRecordPath(t *testing.T) {
	for _, tc := range []struct {
		name     string
		explicit string
		dbPath   string
		want     string
	}{
		{
			name:   "default sits next to the database",
			dbPath: filepath.Join("/var/lib/off-by-one", "off-by-one.db"),
			want:   filepath.Join("/var/lib/off-by-one", seedRecordDefaultName),
		},
		{
			name:   "default db in the cwd writes into the cwd",
			dbPath: "off-by-one.db",
			want:   seedRecordDefaultName,
		},
		{
			name:   "empty db path falls back to the plain default name",
			dbPath: "",
			want:   seedRecordDefaultName,
		},
		{
			name:     "an explicit path wins",
			explicit: "/tmp/custom/obo.prom",
			dbPath:   "/var/lib/off-by-one/off-by-one.db",
			want:     "/tmp/custom/obo.prom",
		},
		{
			name:     "an explicit path is cleaned",
			explicit: "/tmp/../tmp/custom/obo.prom",
			dbPath:   "/var/lib/off-by-one/off-by-one.db",
			want:     "/tmp/custom/obo.prom",
		},
		{
			name:     "dash disables the record",
			explicit: "-",
			dbPath:   "/var/lib/off-by-one/off-by-one.db",
			want:     "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := resolveSeedRecordPath(tc.explicit, tc.dbPath); got != tc.want {
				t.Fatalf("resolveSeedRecordPath(%q, %q) = %q, want %q", tc.explicit, tc.dbPath, got, tc.want)
			}
		})
	}
}

// TestSeedRunWritesCompletionRecordOnSuccess is the arm-2 gate: a
// successful seed leaves a parseable textfile record next to the DB
// carrying exit status 0, a timestamp and the loaded classes/answers.
func TestSeedRunWritesCompletionRecordOnSuccess(t *testing.T) {
	fixture := writeSeedFixture(t)
	dbPath := filepath.Join(t.TempDir(), "seeded.db")
	recordPath := filepath.Join(filepath.Dir(dbPath), seedRecordDefaultName)

	before := time.Now().Unix()
	if err := seedRun([]string{"-dir", fixture, "-db", dbPath}); err != nil {
		t.Fatalf("seedRun: %v", err)
	}

	fams := parseSeedRecord(t, recordPath)
	if got := seedRecordSample(t, fams, "obo_seed_last_exit_status"); got != 0 {
		t.Errorf("obo_seed_last_exit_status = %v, want 0", got)
	}
	if got := seedRecordSample(t, fams, "obo_seed_files_loaded"); got != 1 {
		t.Errorf("obo_seed_files_loaded = %v, want 1 (fixture corpus)", got)
	}
	if got := seedRecordSample(t, fams, "obo_seed_classes_loaded"); got != 1 {
		t.Errorf("obo_seed_classes_loaded = %v, want 1", got)
	}
	if got := seedRecordSample(t, fams, "obo_seed_answers_loaded"); got != 1 {
		t.Errorf("obo_seed_answers_loaded = %v, want 1", got)
	}
	if got := seedRecordSample(t, fams, "obo_seed_answers_created"); got != 1 {
		t.Errorf("obo_seed_answers_created = %v, want 1 on a fresh DB", got)
	}
	if got := seedRecordSample(t, fams, "obo_seed_answers_skipped"); got != 0 {
		t.Errorf("obo_seed_answers_skipped = %v, want 0 on a fresh DB", got)
	}
	ts := seedRecordSample(t, fams, "obo_seed_last_run_timestamp_seconds")
	if ts < float64(before) || ts > float64(time.Now().Unix()+5) {
		t.Errorf("obo_seed_last_run_timestamp_seconds = %v, want a completion time in [%d, now]", ts, before)
	}

	// A second, idempotent run must OVERWRITE the record: one sample per
	// family, counts reflecting the re-run (nothing created, one skipped).
	if err := seedRun([]string{"-dir", fixture, "-db", dbPath}); err != nil {
		t.Fatalf("second seedRun: %v", err)
	}
	fams = parseSeedRecord(t, recordPath)
	if got := seedRecordSample(t, fams, "obo_seed_last_exit_status"); got != 0 {
		t.Errorf("after re-run: obo_seed_last_exit_status = %v, want 0", got)
	}
	if got := seedRecordSample(t, fams, "obo_seed_answers_created"); got != 0 {
		t.Errorf("after re-run: obo_seed_answers_created = %v, want 0", got)
	}
	if got := seedRecordSample(t, fams, "obo_seed_answers_skipped"); got != 1 {
		t.Errorf("after re-run: obo_seed_answers_skipped = %v, want 1", got)
	}

	raw, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatalf("read record: %v", err)
	}
	if n := strings.Count(string(raw), "\nobo_seed_last_exit_status{"); n != 1 {
		t.Errorf("record holds %d obo_seed_last_exit_status lines, want 1 (rewritten, not appended)", n)
	}
	if !strings.HasSuffix(string(raw), "\n") {
		t.Error("record does not end with a newline (textfile format requirement)")
	}
}

// TestSeedRunWritesCompletionRecordOnFailure proves the record is
// written on the failing path too: a one-shot process that dies leaving
// no trace is exactly the observability hole OBS-OB-005 closes.
func TestSeedRunWritesCompletionRecordOnFailure(t *testing.T) {
	cwd := t.TempDir()
	stubSeedPaths(t, cwd, filepath.Join(t.TempDir(), "bin", "off-by-one"))

	dbPath := filepath.Join(t.TempDir(), "never-created.db")
	missing := filepath.Join(t.TempDir(), "no-such-corpus")

	err := seedRun([]string{"-dir", missing, "-db", dbPath})
	if err == nil {
		t.Fatal("seedRun: expected an error for a missing -dir")
	}
	if _, statErr := os.Stat(dbPath); statErr == nil {
		t.Fatalf("database %s created despite the failure", dbPath)
	}

	recordPath := filepath.Join(filepath.Dir(dbPath), seedRecordDefaultName)
	fams := parseSeedRecord(t, recordPath)
	if got := seedRecordSample(t, fams, "obo_seed_last_exit_status"); got != 1 {
		t.Errorf("obo_seed_last_exit_status = %v, want 1 on the failing path", got)
	}
	for _, name := range []string{
		"obo_seed_files_loaded",
		"obo_seed_classes_loaded",
		"obo_seed_answers_loaded",
		"obo_seed_answers_created",
		"obo_seed_answers_skipped",
		"obo_seed_edges_created",
	} {
		if got := seedRecordSample(t, fams, name); got != 0 {
			t.Errorf("%s = %v after a failed seed, want 0", name, got)
		}
	}
	if got := seedRecordSample(t, fams, "obo_seed_last_run_timestamp_seconds"); got <= 0 {
		t.Errorf("obo_seed_last_run_timestamp_seconds = %v, want a real completion timestamp", got)
	}
}

// TestSeedRunRecordsFailureAfterASuccess proves the record is a
// snapshot of the LAST run, not a failure counter that leaves the
// previous success visible: a success followed by a failure must read
// exit status 1.
func TestSeedRunRecordsFailureAfterASuccess(t *testing.T) {
	fixture := writeSeedFixture(t)
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "seeded.db")
	recordPath := filepath.Join(dir, seedRecordDefaultName)

	if err := seedRun([]string{"-dir", fixture, "-db", dbPath}); err != nil {
		t.Fatalf("seedRun: %v", err)
	}
	if got := seedRecordSample(t, parseSeedRecord(t, recordPath), "obo_seed_last_exit_status"); got != 0 {
		t.Fatalf("first run: obo_seed_last_exit_status = %v, want 0", got)
	}

	if err := seedRun([]string{"-dir", filepath.Join(dir, "gone"), "-db", dbPath}); err == nil {
		t.Fatal("seedRun: expected an error for a missing -dir")
	}
	if got := seedRecordSample(t, parseSeedRecord(t, recordPath), "obo_seed_last_exit_status"); got != 1 {
		t.Fatalf("second run: obo_seed_last_exit_status = %v, want 1 (record must follow the last run)", got)
	}
}

// TestSeedRunRecordPathOverrideAndDisable covers the two operator
// controls: OFF_BY_ONE_SEED_RECORD (the -record flag default) moves the
// record, and "-" turns it off without writing anything.
func TestSeedRunRecordPathOverrideAndDisable(t *testing.T) {
	fixture := writeSeedFixture(t)

	t.Run("env override", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "seeded.db")
		custom := filepath.Join(t.TempDir(), "custom", "obo.prom")
		if err := os.MkdirAll(filepath.Dir(custom), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		t.Setenv(seedRecordEnv, custom)

		if err := seedRun([]string{"-dir", fixture, "-db", dbPath}); err != nil {
			t.Fatalf("seedRun: %v", err)
		}
		if got := seedRecordSample(t, parseSeedRecord(t, custom), "obo_seed_last_exit_status"); got != 0 {
			t.Errorf("env-path record: obo_seed_last_exit_status = %v, want 0", got)
		}
		if _, err := os.Stat(filepath.Join(dir, seedRecordDefaultName)); err == nil {
			t.Errorf("default record %s written despite %s override", seedRecordDefaultName, seedRecordEnv)
		}
	})

	t.Run("explicit flag wins over env", func(t *testing.T) {
		dir := t.TempDir()
		flagPath := filepath.Join(dir, "flag.prom")
		envPath := filepath.Join(dir, "env.prom")
		t.Setenv(seedRecordEnv, envPath)

		if err := seedRun([]string{"-dir", fixture, "-db", filepath.Join(dir, "seeded.db"), "-record", flagPath}); err != nil {
			t.Fatalf("seedRun: %v", err)
		}
		if got := seedRecordSample(t, parseSeedRecord(t, flagPath), "obo_seed_last_exit_status"); got != 0 {
			t.Errorf("-record path: obo_seed_last_exit_status = %v, want 0", got)
		}
		if _, err := os.Stat(envPath); err == nil {
			t.Errorf("-record did not win: %s still written", envPath)
		}
	})

	t.Run("dash disables", func(t *testing.T) {
		dir := t.TempDir()
		dbPath := filepath.Join(dir, "seeded.db")

		if err := seedRun([]string{"-dir", fixture, "-db", dbPath, "-record", "-"}); err != nil {
			t.Fatalf("seedRun: %v", err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read dir: %v", err)
		}
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".prom") {
				t.Errorf("record %s written despite -record -", e.Name())
			}
		}
	})
}
