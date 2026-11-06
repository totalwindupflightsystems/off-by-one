package metrics

import (
	"strings"
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/solver"
)

// TestPromSolveResourceFamilies is the exposition contract for
// OBS-OB-003 (docs/observability-spec.md §3.3): the five solve
// resource/failure families are served from the Observer's cumulative
// totals, the peak-RSS histogram's +Inf bucket equals the solve count,
// and every one of the seven cause labels is present.
func TestPromSolveResourceFamilies(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, nil)
	obs.RecordSolve(SolveRecord{SubmissionID: "ok", PeakRSSBytes: 300 * testMiB, Success: true})
	obs.RecordSolve(SolveRecord{SubmissionID: "dead", PeakRSSBytes: 200 * testMiB, Killed: true})
	obs.RecordSolve(SolveRecord{SubmissionID: "rej", PeakRSSBytes: 5 * testGiB, Rejected: true})
	obs.RecordFailureCause(solver.CauseTimeout, 1)
	obs.RecordFailureCause(solver.CauseSolverExit, 1)
	obs.RecordFailureCause(solver.CauseOther, 1)

	p := NewProm(PromConfig{SolveResources: obs.SolveResources})
	body := serveProm(t, p)

	// Acceptance criterion 1 shape: the families are on the wire.
	for _, want := range []string{
		"# HELP obo_solve_peak_rss_bytes",
		"obo_solve_peak_rss_bytes_bucket",
		"obo_solve_peak_rss_bytes_sum",
		"obo_solve_peak_rss_bytes_count",
		"# HELP obo_solve_killed_total",
		"# HELP obo_solve_over_budget_total",
		"# HELP obo_solve_rejected_total",
		"# HELP obo_solve_failure_total",
		`obo_solve_failure_total{cause="timeout"}`,
		`obo_solve_failure_total{cause="toolchain_missing"}`,
		`obo_solve_failure_total{cause="stale_reaped"}`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("exposition missing %q", want)
		}
	}

	fams := parseProm(t, body)

	hist := fams["obo_solve_peak_rss_bytes"]
	if hist == nil || len(hist.Metric) != 1 {
		t.Fatalf("obo_solve_peak_rss_bytes: family %v with %d metrics, want exactly 1",
			hist != nil, len(hist.GetMetric()))
	}
	h := hist.Metric[0].GetHistogram()
	// The +Inf bucket is the sample count: one observation per solve.
	if h.GetSampleCount() != 3 {
		t.Errorf("histogram sample count = %d, want 3 (+Inf must equal the solve count)", h.GetSampleCount())
	}
	if want := float64(300*testMiB + 200*testMiB + 5*testGiB); h.GetSampleSum() != want {
		t.Errorf("histogram sample sum = %v, want %v", h.GetSampleSum(), want)
	}
	// 200MiB lands under 256MiB; 300MiB under 512MiB; the 5GiB solve is
	// above the last finite bucket and therefore only in +Inf.
	wantBuckets := map[float64]uint64{
		float64(256 * testMiB): 1,
		float64(512 * testMiB): 2,
		float64(1 << 30):       2,
		float64(2684354560):    2,
		float64(4 << 30):       2,
	}
	for _, b := range h.Bucket {
		if b.GetUpperBound() > float64(4<<30) { // the +Inf bucket
			continue
		}
		want, ok := wantBuckets[b.GetUpperBound()]
		if !ok {
			t.Errorf("unexpected bucket le=%v", b.GetUpperBound())
			continue
		}
		if b.GetCumulativeCount() != want {
			t.Errorf("bucket le=%v count = %d, want %d", b.GetUpperBound(), b.GetCumulativeCount(), want)
		}
	}
	// +Inf is synthesised by the exporter and must equal the count.
	var inf float64
	var infCount uint64
	for _, b := range h.Bucket {
		if b.GetUpperBound() > float64(4<<30) {
			inf = b.GetUpperBound()
			infCount = b.GetCumulativeCount()
		}
	}
	if inf == 0 || infCount != 3 {
		t.Errorf("+Inf bucket (le=%v) = %d, want 3", inf, infCount)
	}

	if v := sampleValue(t, fams["obo_solve_killed_total"], nil); v != 1 {
		t.Errorf("obo_solve_killed_total = %v, want 1", v)
	}
	if v := sampleValue(t, fams["obo_solve_over_budget_total"], nil); v != 1 {
		t.Errorf("obo_solve_over_budget_total = %v, want 1", v)
	}
	if v := sampleValue(t, fams["obo_solve_rejected_total"], nil); v != 1 {
		t.Errorf("obo_solve_rejected_total = %v, want 1", v)
	}

	// The cause family carries the closed set: three populated, four at
	// zero — never a partial set and never an unknown label.
	failure := fams["obo_solve_failure_total"]
	if failure == nil {
		t.Fatal("obo_solve_failure_total family missing")
	}
	if len(failure.Metric) != len(solver.FailureCauses) {
		t.Errorf("obo_solve_failure_total has %d series, want %d (closed set)", len(failure.Metric), len(solver.FailureCauses))
	}
	for _, cause := range solver.FailureCauses {
		want := 0.0
		switch cause {
		case solver.CauseTimeout, solver.CauseSolverExit, solver.CauseOther:
			want = 1
		}
		if got := sampleValue(t, failure, map[string]string{"cause": cause}); got != want {
			t.Errorf("obo_solve_failure_total{cause=%q} = %v, want %v", cause, got, want)
		}
	}
}

// TestPromSolveResourceZeroSource keeps the exposition well-formed with
// no wired source (unit-test Server, observer-less build): every family
// still exists, at zero, so dashboards and rate() alerts have a series
// to read instead of an absence they cannot distinguish from an outage.
func TestPromSolveResourceZeroSource(t *testing.T) {
	p := NewProm(PromConfig{})
	fams := parseProm(t, serveProm(t, p))

	for _, name := range []string{
		"obo_solve_peak_rss_bytes",
		"obo_solve_killed_total",
		"obo_solve_over_budget_total",
		"obo_solve_rejected_total",
		"obo_solve_failure_total",
	} {
		if fams[name] == nil {
			t.Errorf("zero-source exposition missing family %s", name)
		}
	}
	hist := fams["obo_solve_peak_rss_bytes"]
	if hist == nil || len(hist.Metric) != 1 {
		t.Fatal("obo_solve_peak_rss_bytes missing or multi-sample")
	}
	if c := hist.Metric[0].GetHistogram().GetSampleCount(); c != 0 {
		t.Errorf("zero-source histogram sample count = %d, want 0", c)
	}
	if got := len(hist.Metric[0].GetHistogram().Bucket); got != len(SolvePeakRSSBuckets)+1 {
		// +Inf is always emitted alongside the finite buckets.
		t.Errorf("zero-source histogram has %d bucket series, want %d (finite + +Inf)", got, len(SolvePeakRSSBuckets)+1)
	}
	for _, cause := range solver.FailureCauses {
		if v := sampleValue(t, fams["obo_solve_failure_total"], map[string]string{"cause": cause}); v != 0 {
			t.Errorf("zero-source failure_total{cause=%q} = %v, want 0", cause, v)
		}
	}
	for _, name := range []string{"obo_solve_killed_total", "obo_solve_over_budget_total", "obo_solve_rejected_total"} {
		if v := sampleValue(t, fams[name], nil); v != 0 {
			t.Errorf("zero-source %s = %v, want 0", name, v)
		}
	}
}

// TestPromSolveResourceConsistentAfterWindowTrim proves the exposition
// counters do not go backwards when the Observer's per-solve record
// window is trimmed — the failure mode of deriving counters from the
// bounded window.
func TestPromSolveResourceConsistentAfterWindowTrim(t *testing.T) {
	obs := New(DefaultRSSAlertBytes, nil)
	obs.RecordSolve(SolveRecord{PeakRSSBytes: 100 * testMiB})
	obs.RecordSolve(SolveRecord{PeakRSSBytes: 100 * testMiB})
	obs.Recent(1) // trim the window to one record

	fams := parseProm(t, serveProm(t, NewProm(PromConfig{SolveResources: obs.SolveResources})))
	hist := fams["obo_solve_peak_rss_bytes"].Metric[0].GetHistogram()
	if hist.GetSampleCount() != 2 {
		t.Errorf("sample count after window trim = %d, want 2", hist.GetSampleCount())
	}
}
