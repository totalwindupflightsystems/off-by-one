package api

import (
	"strings"
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/metrics"
	"github.com/totalwindupflightsystems/off-by-one/internal/solver"
)

// TestMetricsPromServesSolveResources is the HTTP-level contract for
// OBS-OB-003: the wired /metrics/prom handler serves the solve resource
// and failure-cause families with the Observer's live values, so a
// scraper sees a forced timeout as cause="timeout" and a memory kill as
// obo_solve_killed_total / the peak-RSS histogram — not just JSON.
func TestMetricsPromServesSolveResources(t *testing.T) {
	srv, _, _ := newTestServer(t)

	obs := metrics.New(metrics.DefaultRSSAlertBytes, nil)
	obs.RecordSolve(metrics.SolveRecord{SubmissionID: "ok", PeakRSSBytes: 300 << 20, Success: true})
	obs.RecordSolve(metrics.SolveRecord{SubmissionID: "dead", PeakRSSBytes: 600 << 20, Killed: true})
	obs.RecordFailureCause(solver.CauseTimeout, 3)
	obs.RecordFailureCause(solver.CauseSolverExit, 1)

	srv.Observer = obs
	srv.Prom = metrics.NewProm(metrics.PromConfig{SolveResources: obs.SolveResources})

	rr := do(t, srv, "GET", "/metrics/prom", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	for _, want := range []string{
		`obo_solve_failure_total{cause="timeout",` + promProductLabels + `} 3`,
		`obo_solve_failure_total{cause="solver_exit",` + promProductLabels + `} 1`,
		`obo_solve_failure_total{cause="toolchain_missing",` + promProductLabels + `} 0`,
		"obo_solve_killed_total{" + promProductLabels + "} 1",
		"obo_solve_peak_rss_bytes_count{" + promProductLabels + "} 2",
		// Histogram bucket lines carry the const labels first and the
		// generated le label LAST (expfmt appends it while writing).
		`obo_solve_peak_rss_bytes_bucket{` + promProductLabels + `,le="+Inf"} 2`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("exposition missing %q\nbody:\n%s", want, body)
		}
	}
}
