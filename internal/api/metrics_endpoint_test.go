package api

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/metrics"
)

// TestHandleMetricsReturnsSnapshot proves GET /metrics answers 200
// with the Observer's snapshot as JSON: records a solve + a skip, then
// asserts the served body carries them (judge failure #6: no test
// covered the endpoint at all).
func TestHandleMetricsReturnsSnapshot(t *testing.T) {
	s := newMetricsTestServer(t)

	// Populate the observer: one solve (2 GiB, over the 1 GiB test
	// threshold) and one host-pressure skip.
	obs := s.Observer
	obs.RecordSolve(metrics.SolveRecord{
		SubmissionID: "sub-metrics-1",
		ProblemClass: "go-test",
		PeakRSSBytes: 2 << 30,
		Success:      true,
	})
	obs.RecordSkip(metrics.HostSnapshot{Load1: 9.5, Reason: "load"})

	rr := do(t, s, "GET", "/metrics", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want 200", rr.Code)
	}
	var snap metrics.Snapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatalf("body is not valid JSON: %v\n%s", err, rr.Body.String())
	}
	if snap.SolveCount != 1 {
		t.Errorf("solve_count = %d, want 1", snap.SolveCount)
	}
	if snap.SkippedHostPressure != 1 {
		t.Errorf("skipped_host_pressure = %d, want 1", snap.SkippedHostPressure)
	}
	if !snap.LastSolve.OverBudget || snap.LastSolve.PeakRSSBytes != 2<<30 {
		t.Errorf("last_solve = %+v, want over-budget 2GiB solve", snap.LastSolve)
	}
	if snap.LastSkipReason != "load" {
		t.Errorf("last_skip_reason = %q, want %q", snap.LastSkipReason, "load")
	}
}

// TestHandleMetricsEmptyObserver — a fresh observer answers 200 with a
// zeroed snapshot (the endpoint must not 404/500 before the first
// solve).
func TestHandleMetricsEmptyObserver(t *testing.T) {
	s := newMetricsTestServer(t)
	rr := do(t, s, "GET", "/metrics", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want 200", rr.Code)
	}
	var snap metrics.Snapshot
	if err := json.Unmarshal(rr.Body.Bytes(), &snap); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if snap.SolveCount != 0 || snap.SkippedHostPressure != 0 {
		t.Errorf("fresh snapshot = %+v, want zeroed", snap)
	}
}

// TestHandleMetricsNilObserver — metrics disabled answers the explicit
// 404 error shape, not a panic and not the SPA.
func TestHandleMetricsNilObserver(t *testing.T) {
	s := newMetricsTestServer(t)
	s.Observer = nil
	rr := do(t, s, "GET", "/metrics", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /metrics (nil observer) status = %d, want 404", rr.Code)
	}
	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("404 body is not the error envelope: %v", err)
	}
	if body.Error != "metrics_disabled" {
		t.Errorf("404 error = %q, want %q", body.Error, "metrics_disabled")
	}
}

// TestMetricsRouterDispatch documents the wiring the rework had to
// fix: the API mux registers GET /metrics, and this file's server goes
// through Handler() — the same route table the production router
// dispatches to. The dispatch itself is proven live by
// cmd/off-by-one's router (see the /metrics case in the composed
// handler); this test pins the API side so a route regression cannot
// land silently.
func TestMetricsRouterDispatch(t *testing.T) {
	s := newMetricsTestServer(t)
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /metrics via Handler() = %d, want 200 (route must stay registered)", resp.StatusCode)
	}
}

// ch:trace row=REVIEW-OB-019 spec=internal/metrics/metrics.go#Observer wave=none:serial-slot test=internal/api/metrics_endpoint_test.go::TestMetricsDiscoveryCounters doc=none:reason evidence=commit
//
// TestMetricsDiscoveryCounters pins the additive discovery counters
// (REVIEW-OB-019): POST /api/v1/problems/discover records a hit when an
// exact answer resolves, a miss on the 404 not_found arm, and nothing
// for a 400 invalid request; GET /metrics then serves discovery_hits and
// discovery_misses as real JSON keys. The counters are request-path
// telemetry and must not shadow the catalog-wide /api/v1/stats hit_rate.
func TestMetricsDiscoveryCounters(t *testing.T) {
	s, store, _ := newTestServer(t)
	s.Observer = metrics.New(1<<30, log.New(testDiscard{}, "", 0))
	seedClass(t, store, "ob019-hit", "permissions", "docker", "go", "1.0", "use --user", graph.AnswerVerified)

	// Hit arm: an existing class resolves an exact answer.
	rr := do(t, s, "POST", "/api/v1/problems/discover", discoverRequest{
		ProblemClass: "ob019-hit",
		Environment:  "docker",
		Language:     "go",
		Version:      "1.0",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("discover (seeded class) status = %d, want 200, body = %s", rr.Code, rr.Body.String())
	}
	var found discoverResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &found); err != nil {
		t.Fatalf("decode discover: %v", err)
	}
	if !found.Found {
		t.Fatalf("discover found = false for the seeded class, want true")
	}

	// Miss arm: an absent class answers 404 not_found.
	rr = do(t, s, "POST", "/api/v1/problems/discover", discoverRequest{ProblemClass: "ob019-absent"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("discover (absent class) status = %d, want 404, body = %s", rr.Code, rr.Body.String())
	}

	// 400 arm: an invalid request records nothing.
	rr = do(t, s, "POST", "/api/v1/problems/discover", discoverRequest{})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("discover (invalid request) status = %d, want 400, body = %s", rr.Code, rr.Body.String())
	}

	rr = do(t, s, "GET", "/metrics", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want 200", rr.Code)
	}
	// Presence, not just value: decoding into Snapshot would silently
	// accept a zero when the JSON tag is missing, so read the raw keys.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("/metrics body is not a JSON object: %v\n%s", err, rr.Body.String())
	}
	for _, key := range []string{"discovery_hits", "discovery_misses"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("/metrics payload is missing the %q key; body = %s", key, rr.Body.String())
		}
	}
	if _, ok := raw["hit_rate"]; ok {
		t.Errorf("/metrics must not serve a hit_rate field — that ratio belongs to /api/v1/stats; body = %s", rr.Body.String())
	}
	var hits, misses int64
	if err := json.Unmarshal(raw["discovery_hits"], &hits); err != nil {
		t.Fatalf("decode discovery_hits: %v", err)
	}
	if err := json.Unmarshal(raw["discovery_misses"], &misses); err != nil {
		t.Fatalf("decode discovery_misses: %v", err)
	}
	if hits != 1 {
		t.Errorf("discovery_hits = %d, want 1 (one 200 discover; the 400 must not count)", hits)
	}
	if misses != 1 {
		t.Errorf("discovery_misses = %d, want 1 (one 404 not_found)", misses)
	}

	// Nil observer (metrics disabled) must not panic the discover path
	// with the nil-check the handler shares with handleMetrics.
	s.Observer = nil
	rr = do(t, s, "POST", "/api/v1/problems/discover", discoverRequest{
		ProblemClass: "ob019-hit",
		Environment:  "docker",
		Language:     "go",
		Version:      "1.0",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("discover with nil observer status = %d, want 200 (no panic), body = %s", rr.Code, rr.Body.String())
	}
	rr = do(t, s, "POST", "/api/v1/problems/discover", discoverRequest{ProblemClass: "ob019-absent"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("discover miss with nil observer status = %d, want 404 (no panic), body = %s", rr.Code, rr.Body.String())
	}
}

// newMetricsTestServer builds the minimal Server for /metrics tests:
// real handlers need a store+queue, so reuse the shared test factory
// and attach a fresh Observer.
func newMetricsTestServer(t *testing.T) *Server {
	t.Helper()
	s, _, _ := newTestServer(t)
	s.Observer = metrics.New(1<<30, log.New(testDiscard{}, "", 0))
	return s
}

// testDiscard swallows the observer's alert logs so test output stays
// clean (the 2 GiB solve above trips the 1 GiB threshold on purpose).
type testDiscard struct{}

func (testDiscard) Write(p []byte) (int, error) { return len(p), nil }
