package api

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/metrics"
)

// promGauge scans a Prometheus text exposition for an unlabelled
// sample line ("name value") and returns its value.
func promGauge(t *testing.T, body, name string) float64 {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, name+" ") {
			v, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimPrefix(line, name)), 64)
			if err != nil {
				t.Fatalf("parse %q line %q: %v", name, line, err)
			}
			return v
		}
	}
	t.Fatalf("no sample line for %s in exposition", name)
	return 0
}

// TestMetricsPromEndpoint is the HTTP-level contract for OBS-OB-001.
// The status code alone proves nothing — the SPA catch-all also
// answers /metrics/prom with 200 (DF-OFF-BY-ONE-32 trap) — so the
// assertions are on Content-Type and body content.
func TestMetricsPromEndpoint(t *testing.T) {
	srv, _, _ := newTestServer(t)

	rr := do(t, srv, "GET", "/metrics/prom", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	ct := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Fatalf("Content-Type %q, want prefix text/plain; version=0.0.4 (HTML here means the SPA catch-all answered, not the API)", ct)
	}
	body := rr.Body.String()
	if strings.HasPrefix(body, "<!DOCTYPE") {
		t.Fatal("body is HTML: the SPA catch-all answered /metrics/prom, not the API handler")
	}
	for _, want := range []string{
		"# HELP obo_",
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
	if v := promGauge(t, body, "obo_http_up"); v != 1 {
		t.Errorf("obo_http_up = %v, want 1", v)
	}

	// obo_queue_pending must agree with /api/v1/stats queue_depth —
	// both read queue.Depth over the same store.
	statsRR := do(t, srv, "GET", "/api/v1/stats", nil)
	if statsRR.Code != 200 {
		t.Fatalf("/api/v1/stats: status %d", statsRR.Code)
	}
	var stats struct {
		QueueDepth int `json:"queue_depth"`
	}
	if err := json.Unmarshal(statsRR.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if v := promGauge(t, body, "obo_queue_pending"); v != float64(stats.QueueDepth) {
		t.Errorf("obo_queue_pending = %v, /api/v1/stats queue_depth = %d — must agree", v, stats.QueueDepth)
	}
}

// TestMetricsPromNilSafe covers direct Server construction (no New,
// no queue, no Prom, no Observer): /metrics/prom must still answer
// 200 with a parseable exposition and every family present, while the
// JSON /metrics keeps its existing metrics_disabled 404.
func TestMetricsPromNilSafe(t *testing.T) {
	srv := &Server{}

	rr := do(t, srv, "GET", "/metrics/prom", nil)
	if rr.Code != 200 {
		t.Fatalf("status %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Fatalf("Content-Type %q, want text/plain; version=0.0.4", ct)
	}
	body := rr.Body.String()
	for _, want := range []string{
		"obo_http_up 1",
		"obo_cron_loop_running 0",
		`obo_solves_total{outcome="succeeded"} 0`,
		"obo_queue_pending 0",
		"obo_solve_duration_seconds_bucket",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("nil-safe exposition missing %q", want)
		}
	}

	jsonRR := do(t, srv, "GET", "/metrics", nil)
	if jsonRR.Code != 404 {
		t.Errorf("JSON /metrics with nil Observer: status %d, want 404 metrics_disabled (unchanged behaviour)", jsonRR.Code)
	}
}

// TestMetricsPromReadOnlyPassthrough proves sanitizeReadOnlyJSON lets
// the text/plain exposition through untouched in read-only (public
// catalog) mode.
func TestMetricsPromReadOnlyPassthrough(t *testing.T) {
	srv, _, _ := newTestServer(t)
	srv.ReadOnly = true

	rr := do(t, srv, "GET", "/metrics/prom", nil)
	if rr.Code != 200 {
		t.Fatalf("read-only /metrics/prom: status %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Fatalf("read-only Content-Type %q — sanitizer mangled the exposition", ct)
	}
	if !strings.Contains(rr.Body.String(), "obo_http_up 1") {
		t.Error("read-only exposition missing obo_http_up 1")
	}
}

// TestMetricsJSONUnchangedAfterProm proves the JSON /metrics contract
// is byte-shape compatible after adding the prom surface: same keys,
// same types.
func TestMetricsJSONUnchangedAfterProm(t *testing.T) {
	srv, _, _ := newTestServer(t)
	srv.Observer = metrics.New(0, nil)

	rr := do(t, srv, "GET", "/metrics", nil)
	if rr.Code != 200 {
		t.Fatalf("JSON /metrics: status %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("JSON /metrics Content-Type %q, want application/json", ct)
	}
	var snap map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatalf("decode JSON /metrics: %v", err)
	}
	v, ok := snap["skipped_host_pressure"]
	if !ok {
		t.Fatal("JSON /metrics missing skipped_host_pressure")
	}
	if _, isNum := v.(float64); !isNum {
		t.Errorf("skipped_host_pressure = %v (%T), want a number", v, v)
	}

	// Both surfaces coexist on one server.
	promRR := do(t, srv, "GET", "/metrics/prom", nil)
	if promRR.Code != 200 || !strings.Contains(promRR.Body.String(), "obo_http_up") {
		t.Errorf("/metrics/prom alongside JSON /metrics: status %d", promRR.Code)
	}
}
