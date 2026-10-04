package metrics

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, log.Default())
	if obs == nil {
		t.Fatal("New returned nil")
	}
	if obs.alertBytes != DefaultRSSAlertBytes {
		t.Errorf("alertBytes = %d, want %d", obs.alertBytes, DefaultRSSAlertBytes)
	}
}

func TestRecordSolve(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, log.Default())
	rec := SolveRecord{
		SubmissionID: "test-1",
		ProblemClass: "go-test",
		PeakRSSBytes: 1 << 30, // 1 GiB
		WallTimeMS:   1000,
		Success:      true,
	}
	obs.RecordSolve(rec)

	snap := obs.Snapshot()
	if snap.SolveCount != 1 {
		t.Errorf("SolveCount = %d, want 1", snap.SolveCount)
	}
	if snap.LastSolve == nil {
		t.Fatal("LastSolve is nil after RecordSolve")
	}
	if snap.LastSolve.SubmissionID != "test-1" {
		t.Errorf("SubmissionID = %q, want %q", snap.LastSolve.SubmissionID, "test-1")
	}
}

func TestRecordSolveOverBudget(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "", 0)
	obs := New(DefaultRSSAlertBytes, logger)

	rec := SolveRecord{
		SubmissionID: "test-2",
		ProblemClass: "go-bbr-pacing",
		PeakRSSBytes: 5 << 30, // 5 GiB > 4 GiB threshold
		WallTimeMS:   2000,
		Success:      true,
	}
	obs.RecordSolve(rec)

	output := buf.String()
	if !strings.Contains(output, "solve ALERT") {
		t.Errorf("expected ALERT log, got %q", output)
	}
	if !strings.Contains(output, "peak RSS 5368709120") {
		t.Errorf("expected peak RSS in log, got %q", output)
	}
}

func TestRecordSkip(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, log.Default())
	obs.RecordSkip(HostSnapshot{
		Load1:  10.5,
		Reason: "load",
	})

	if obs.Skips() != 1 {
		t.Errorf("Skips = %d, want 1", obs.Skips())
	}
}

func TestRecordHost(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, log.Default())
	obs.RecordHost(HostSnapshot{
		Load1:           2.5,
		MemUsedFraction: 0.65,
	})

	snap := obs.Snapshot()
	if snap.LastHost == nil {
		t.Fatal("LastHost is nil after RecordHost")
	}
	if snap.LastHost.Load1 != 2.5 {
		t.Errorf("LastHost.Load1 = %f, want 2.5", snap.LastHost.Load1)
	}
	if snap.LastHost.MemUsedFraction != 0.65 {
		t.Errorf("LastHost.MemUsedFraction = %f, want 0.65", snap.LastHost.MemUsedFraction)
	}
}

func TestSnapshotOverBudget(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, log.Default())

	// Record one over-budget solve
	obs.RecordSolve(SolveRecord{
		SubmissionID: "over-1",
		PeakRSSBytes: 5 << 30, // 5 GiB > 4 GiB threshold
		OverBudget:   true,
	})

	snap := obs.Snapshot()
	if snap.OverBudget != 1 {
		t.Errorf("OverBudget = %d, want 1", snap.OverBudget)
	}
	if snap.PeakRSSBytesMax != 5<<30 {
		t.Errorf("PeakRSSBytesMax = %d, want %d", snap.PeakRSSBytesMax, 5<<30)
	}
}

func TestSnapshotSolveCount(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, log.Default())

	// Record 5 solves
	for i := 0; i < 5; i++ {
		obs.RecordSolve(SolveRecord{
			SubmissionID: "test",
			PeakRSSBytes: uint64(i) << 30,
		})
	}

	snap := obs.Snapshot()
	if snap.SolveCount != 5 {
		t.Errorf("SolveCount = %d, want 5", snap.SolveCount)
	}
}

func TestAlertThreshold(t *testing.T) {
	obs := New(8<<30, log.Default()) // 8 GiB
	if obs.AlertThreshold() != 8<<30 {
		t.Errorf("AlertThreshold = %d, want %d", obs.AlertThreshold(), 8<<30)
	}
}
