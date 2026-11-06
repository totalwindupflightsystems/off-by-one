package metrics

import (
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/solver"
)

const (
	testMiB = uint64(1) << 20
	testGiB = uint64(1) << 30
)

// TestSolveResourcesCumulativeTotals pins the per-solve resource
// accounting behind obo_solve_peak_rss_bytes / _killed_total /
// _over_budget_total / _rejected_total (OBS-OB-003): one histogram
// observation per recorded solve, cumulative bucket counts, and the
// kill/over-budget/rejection counters.
func TestSolveResourcesCumulativeTotals(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, nil)
	obs.RecordSolve(SolveRecord{SubmissionID: "s1", PeakRSSBytes: 100 * testMiB, Success: true})
	obs.RecordSolve(SolveRecord{SubmissionID: "s2", PeakRSSBytes: 600 * testMiB, Killed: true})
	obs.RecordSolve(SolveRecord{SubmissionID: "s3", PeakRSSBytes: 5 * testGiB, Rejected: true})

	r := obs.SolveResources()
	if r.PeakRSSCount != 3 {
		t.Fatalf("PeakRSSCount = %d, want 3 (one observation per solve)", r.PeakRSSCount)
	}
	wantSum := uint64(100*testMiB + 600*testMiB + 5*testGiB)
	if r.PeakRSSSum != wantSum {
		t.Errorf("PeakRSSSum = %d, want %d", r.PeakRSSSum, wantSum)
	}
	if len(r.PeakRSSBucketCounts) != len(SolvePeakRSSBuckets)+1 {
		t.Fatalf("bucket counts len = %d, want %d (+Inf slot)",
			len(r.PeakRSSBucketCounts), len(SolvePeakRSSBuckets)+1)
	}
	// Cumulative: 256MiB→1, 512MiB→1, 1GiB→2, 2.5GiB→2, 4GiB→2, +Inf→3.
	wantBuckets := []uint64{1, 1, 2, 2, 2, 3}
	for i, want := range wantBuckets {
		if got := r.PeakRSSBucketCounts[i]; got != want {
			t.Errorf("bucket[%d] = %d, want %d", i, got, want)
		}
	}
	// The +Inf slot is the number of observations — the invariant
	// acceptance criterion 3 checks on the wire.
	if inf := r.PeakRSSBucketCounts[len(r.PeakRSSBucketCounts)-1]; inf != uint64(r.PeakRSSCount) {
		t.Errorf("+Inf bucket = %d, solve count = %d — must match", inf, r.PeakRSSCount)
	}

	if r.Killed != 1 {
		t.Errorf("Killed = %d, want 1", r.Killed)
	}
	if r.OverBudget != 1 {
		t.Errorf("OverBudget = %d, want 1 (only the 5 GiB solve crossed 4 GiB)", r.OverBudget)
	}
	if r.Rejected != 1 {
		t.Errorf("Rejected = %d, want 1", r.Rejected)
	}
}

// TestSolveResourcesSurviveRecordWindowTrim is the regression guard for
// the trap this design avoids: Observer.records is a trimmable window,
// so any counter derived from it (like Snapshot().SolveCount) would go
// backwards after Recent(n). The cumulative resource totals must not.
func TestSolveResourcesSurviveRecordWindowTrim(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, nil)
	for i, peak := range []uint64{100 * testMiB, 200 * testMiB, 300 * testMiB} {
		obs.RecordSolve(SolveRecord{SubmissionID: string(rune('a' + i)), PeakRSSBytes: peak})
	}
	before := obs.SolveResources()

	if got := obs.Recent(1); len(got) != 1 {
		t.Fatalf("Recent(1) returned %d records, want 1", len(got))
	}
	after := obs.SolveResources()

	if after.PeakRSSCount != before.PeakRSSCount || after.PeakRSSCount != 3 {
		t.Errorf("PeakRSSCount after trim = %d, want 3 (unaffected by the record window)",
			after.PeakRSSCount)
	}
	if after.PeakRSSSum != before.PeakRSSSum {
		t.Errorf("PeakRSSSum after trim = %d, want %d", after.PeakRSSSum, before.PeakRSSSum)
	}
	if after.PeakRSSBucketCounts[len(after.PeakRSSBucketCounts)-1] != 3 {
		t.Errorf("+Inf bucket after trim = %d, want 3",
			after.PeakRSSBucketCounts[len(after.PeakRSSBucketCounts)-1])
	}
}

// TestRecordFailureCauseKeepsLabelSetClosed: the cause set is closed by
// contract (spec §3.3), so free text from a caller is coalesced into
// "other" rather than minting a new series.
func TestRecordFailureCauseKeepsLabelSetClosed(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, nil)
	obs.RecordFailureCause(solver.CauseTimeout, 1)
	obs.RecordFailureCause(solver.CauseTimeout, 2)
	obs.RecordFailureCause(solver.CauseCommit, 1)
	// Arbitrary free text must not widen the label set.
	obs.RecordFailureCause("pi exited with code 1", 4)
	obs.RecordFailureCause("", 1)
	// Non-positive counts are no-ops (the reap path passes 0 routinely).
	obs.RecordFailureCause(solver.CauseStaleReaped, 0)
	obs.RecordFailureCause(solver.CauseStaleReaped, -3)

	r := obs.SolveResources()
	if got := r.Failures[solver.CauseTimeout]; got != 3 {
		t.Errorf("timeout = %d, want 3 (counts accumulate)", got)
	}
	if got := r.Failures[solver.CauseCommit]; got != 1 {
		t.Errorf("commit = %d, want 1", got)
	}
	if got := r.Failures[solver.CauseOther]; got != 5 {
		t.Errorf("other = %d, want 5 (4 free-text + 1 empty)", got)
	}
	if got := r.Failures[solver.CauseStaleReaped]; got != 0 {
		t.Errorf("stale_reaped = %d, want 0 (non-positive n is a no-op)", got)
	}
	for cause := range r.Failures {
		if !solver.IsFailureCause(cause) {
			t.Errorf("Failures contains %q, outside the closed set", cause)
		}
	}
}

// TestRecordFailureCauseReapBatch proves the reaper's batch form: n
// reaped entries in one call count n failures, since a reaped entry
// never reaches the per-solve failure path.
func TestRecordFailureCauseReapBatch(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, nil)
	obs.RecordFailureCause(solver.CauseStaleReaped, 6)
	if got := obs.SolveResources().Failures[solver.CauseStaleReaped]; got != 6 {
		t.Errorf("stale_reaped = %d, want 6", got)
	}
}

// TestSolveResourcesNilSafe: a nil Observer yields a well-formed zero
// snapshot (full-length bucket slice) so the collector cannot emit a
// malformed histogram.
func TestSolveResourcesNilSafe(t *testing.T) {
	var obs *Observer
	r := obs.SolveResources()
	if r.PeakRSSCount != 0 || r.PeakRSSSum != 0 || r.Killed != 0 {
		t.Errorf("nil Observer resources = %+v, want zeros", r)
	}
	if len(r.PeakRSSBucketCounts) != len(SolvePeakRSSBuckets)+1 {
		t.Errorf("nil Observer bucket len = %d, want %d",
			len(r.PeakRSSBucketCounts), len(SolvePeakRSSBuckets)+1)
	}
	obs.RecordFailureCause(solver.CauseTimeout, 1)      // must not panic
	obs.RecordSolve(SolveRecord{PeakRSSBytes: testGiB}) // must not panic
}

// TestSolveResourcesIsACopy: mutating the returned snapshot must never
// corrupt the Observer's counters.
func TestSolveResourcesIsACopy(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, nil)
	obs.RecordSolve(SolveRecord{PeakRSSBytes: 10 * testMiB})
	obs.RecordFailureCause(solver.CauseTimeout, 1)

	r := obs.SolveResources()
	r.PeakRSSBucketCounts[0] = 99
	r.Failures[solver.CauseTimeout] = 99

	again := obs.SolveResources()
	if again.PeakRSSBucketCounts[0] != 1 {
		t.Errorf("bucket[0] = %d after caller mutation, want 1", again.PeakRSSBucketCounts[0])
	}
	if again.Failures[solver.CauseTimeout] != 1 {
		t.Errorf("timeout = %d after caller mutation, want 1", again.Failures[solver.CauseTimeout])
	}
}

// TestZeroValueObserverHistogram pins the zero-value safety arm: an
// Observer built by struct literal (no New) must still produce a
// consistent histogram instead of panicking on a nil bucket slice.
func TestZeroValueObserverHistogram(t *testing.T) {
	obs := &Observer{}
	obs.RecordSolve(SolveRecord{PeakRSSBytes: 100 * testMiB})
	r := obs.SolveResources()
	if r.PeakRSSCount != 1 {
		t.Fatalf("PeakRSSCount = %d, want 1", r.PeakRSSCount)
	}
	if r.PeakRSSBucketCounts[0] != 1 || r.PeakRSSBucketCounts[len(r.PeakRSSBucketCounts)-1] != 1 {
		t.Errorf("zero-value Observer buckets = %v, want first and +Inf at 1", r.PeakRSSBucketCounts)
	}
}
