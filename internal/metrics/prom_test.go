package metrics

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
)

// serveProm performs a scrape against p's handler and returns the
// response body as text.
func serveProm(t *testing.T, p *Prom) string {
	t.Helper()
	rr := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/metrics/prom", nil)
	p.Handler().ServeHTTP(rr, req)
	if rr.Code != 200 {
		t.Fatalf("GET /metrics/prom: status %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Fatalf("Content-Type %q, want prefix text/plain; version=0.0.4", ct)
	}
	body, err := io.ReadAll(rr.Result().Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// parseProm runs the full expfmt.TextParser over the exposition: any
// malformed line is a parse error.
func parseProm(t *testing.T, body string) map[string]*dto.MetricFamily {
	t.Helper()
	// The zero TextParser is unusable in prometheus/common v0.70+ (its
	// name-validation scheme is unset and panics); LegacyValidation is
	// the scheme promhttp emits.
	parser := expfmt.NewTextParser(model.LegacyValidation)
	fams, err := parser.TextToMetricFamilies(strings.NewReader(body))
	if err != nil {
		t.Fatalf("exposition has malformed lines: %v", err)
	}
	return fams
}

// sampleValue returns the value of the single (or labelled) sample of
// a gauge/counter family, matching on the given label values.
func sampleValue(t *testing.T, fam *dto.MetricFamily, labels map[string]string) float64 {
	t.Helper()
	for _, m := range fam.Metric {
		match := true
		got := map[string]string{}
		for _, lp := range m.Label {
			got[lp.GetName()] = lp.GetValue()
		}
		for k, v := range labels {
			if got[k] != v {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		if m.Gauge != nil {
			return m.Gauge.GetValue()
		}
		if m.Counter != nil {
			return m.Counter.GetValue()
		}
		t.Fatalf("family %s: matched metric has neither gauge nor counter", fam.GetName())
	}
	t.Fatalf("family %s: no sample with labels %v", fam.GetName(), labels)
	return 0
}

func TestPromExpositionFamiliesAndPartition(t *testing.T) {
	p := NewProm(PromConfig{
		LoopRunning: func() bool { return true },
		SolveCounts: func() PromSolveCounts {
			// Failed includes Rejected (cron.Metrics semantics): the
			// collector must partition to succeeded=5, failed=2,
			// rejected=1 — sum 8 == attempts.
			return PromSolveCounts{Success: 5, Failed: 3, Rejected: 1}
		},
		QueuePending: func(ctx context.Context) (int64, error) { return 7, nil },
	})
	p.ObserveSolveDuration("succeeded", 95*time.Second)

	body := serveProm(t, p)
	for _, want := range []string{
		"# HELP obo_http_up",
		"obo_http_up",
		"obo_cron_loop_running",
		"obo_solves_total",
		"obo_queue_pending",
		"obo_solve_duration_seconds_bucket",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("exposition missing %q", want)
		}
	}

	fams := parseProm(t, body)

	if v := sampleValue(t, fams["obo_http_up"], nil); v != 1 {
		t.Errorf("obo_http_up = %v, want 1", v)
	}
	if v := sampleValue(t, fams["obo_cron_loop_running"], nil); v != 1 {
		t.Errorf("obo_cron_loop_running = %v, want 1", v)
	}
	if v := sampleValue(t, fams["obo_queue_pending"], nil); v != 7 {
		t.Errorf("obo_queue_pending = %v, want 7", v)
	}

	succ := sampleValue(t, fams["obo_solves_total"], map[string]string{"outcome": "succeeded"})
	fail := sampleValue(t, fams["obo_solves_total"], map[string]string{"outcome": "failed"})
	rej := sampleValue(t, fams["obo_solves_total"], map[string]string{"outcome": "rejected"})
	if succ != 5 || fail != 2 || rej != 1 {
		t.Errorf("partition: succeeded=%v failed=%v rejected=%v, want 5/2/1", succ, fail, rej)
	}
	if succ+fail+rej != 8 {
		t.Errorf("sum(obo_solves_total) = %v, want 8 (== SolveAttempts)", succ+fail+rej)
	}

	dur := fams["obo_solve_duration_seconds"]
	if dur == nil {
		t.Fatal("obo_solve_duration_seconds family missing")
	}
	var succHist, failHist, rejHist *dto.Metric
	for _, m := range dur.Metric {
		for _, lp := range m.Label {
			if lp.GetName() != "outcome" {
				continue
			}
			switch lp.GetValue() {
			case "succeeded":
				succHist = m
			case "failed":
				failHist = m
			case "rejected":
				rejHist = m
			}
		}
	}
	if succHist == nil || failHist == nil || rejHist == nil {
		t.Fatalf("histogram children: succeeded=%v failed=%v rejected=%v present, want all three",
			succHist != nil, failHist != nil, rejHist != nil)
	}
	h := succHist.GetHistogram()
	if h.GetSampleCount() != 1 || h.GetSampleSum() != 95 {
		t.Errorf("succeeded histogram: count=%d sum=%v, want 1/95", h.GetSampleCount(), h.GetSampleSum())
	}
	// 95s lands in the le=120 bucket but not le=60.
	var le60, le120 uint64
	for _, b := range h.Bucket {
		switch b.GetUpperBound() {
		case 60:
			le60 = b.GetCumulativeCount()
		case 120:
			le120 = b.GetCumulativeCount()
		}
	}
	if le60 != 0 || le120 != 1 {
		t.Errorf("buckets: le=60 count=%d (want 0), le=120 count=%d (want 1)", le60, le120)
	}
	if c := failHist.GetHistogram().GetSampleCount(); c != 0 {
		t.Errorf("failed histogram count=%d, want 0 (pre-created child, no observations)", c)
	}
}

func TestPromNilSafeZeroExposition(t *testing.T) {
	var p *Prom // never constructed: must still serve a valid exposition
	body := serveProm(t, p)
	if strings.HasPrefix(body, "<!DOCTYPE") {
		t.Fatal("exposition is HTML, not text format")
	}
	fams := parseProm(t, body)
	for _, name := range []string{
		"obo_http_up", "obo_cron_loop_running", "obo_solves_total",
		"obo_queue_pending", "obo_solve_duration_seconds",
	} {
		if fams[name] == nil {
			t.Errorf("nil-source exposition missing family %s", name)
		}
	}
	if v := sampleValue(t, fams["obo_http_up"], nil); v != 1 {
		t.Errorf("obo_http_up = %v, want 1 even on the zero registry", v)
	}
	if v := sampleValue(t, fams["obo_cron_loop_running"], nil); v != 0 {
		t.Errorf("obo_cron_loop_running = %v, want 0 with no source", v)
	}
	for _, outcome := range []string{"succeeded", "failed", "rejected"} {
		if v := sampleValue(t, fams["obo_solves_total"], map[string]string{"outcome": outcome}); v != 0 {
			t.Errorf("obo_solves_total{outcome=%s} = %v, want 0", outcome, v)
		}
	}
}

func TestPromQueuePendingSourceErrorOmitsSeries(t *testing.T) {
	p := NewProm(PromConfig{
		QueuePending: func(ctx context.Context) (int64, error) { return 0, errors.New("db wedged") },
	})
	fams := parseProm(t, serveProm(t, p))
	fam := fams["obo_queue_pending"]
	if fam != nil && len(fam.Metric) != 0 {
		t.Errorf("errored queue source emitted %d samples; want the series omitted, not a fabricated 0", len(fam.Metric))
	}
}

func TestPromObserveNilSafe(t *testing.T) {
	var p *Prom
	p.ObserveSolveDuration("succeeded", time.Second) // must not panic
}
