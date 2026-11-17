// ch:trace row=REVIEW-OB-024 spec=pkg/api/openapi.yaml#/metrics/prom wave=off-by-one-foreman-2026-11-16#slot-review test=internal/api/openapi_metrics_prom_test.go::TestServedOpenAPISpec_DocumentsMetricsProm doc=pkg/api/openapi.yaml evidence=live:scratch-18772-metrics-prom-200
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	specapi "github.com/totalwindupflightsystems/off-by-one/pkg/api"
)

// TestServedOpenAPISpec_DocumentsMetricsProm pins REVIEW-OB-024 at the HTTP
// surface: main.go pre-loads pkg/api.JSONBytes() into Server.OpenAPISpec,
// so the document served at /openapi.json must advertise GET /metrics/prom
// — and the route it advertises must still answer with the Prometheus text
// exposition content type. This is the pair a spec-only or a route-only
// check would each miss.
func TestServedOpenAPISpec_DocumentsMetricsProm(t *testing.T) {
	srv, _, _ := newTestServer(t)
	spec, err := specapi.JSONBytes()
	if err != nil {
		t.Fatalf("specapi.JSONBytes: %v", err)
	}
	srv.OpenAPISpec = spec

	rr := do(t, srv, "GET", "/openapi.json", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET /openapi.json status = %d, want 200", rr.Code)
	}
	var doc map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &doc); err != nil {
		t.Fatalf("served /openapi.json is not JSON: %v", err)
	}
	paths, ok := doc["paths"].(map[string]any)
	if !ok {
		t.Fatalf("served document has no paths object (%T)", doc["paths"])
	}
	promPath, ok := paths["/metrics/prom"].(map[string]any)
	if !ok {
		t.Fatalf("served /openapi.json does not advertise /metrics/prom (%d paths served)", len(paths))
	}
	if _, ok := promPath["get"].(map[string]any); !ok {
		t.Fatalf("/metrics/prom is served without a GET operation")
	}

	// The advertised route must be live and answer with the exposition
	// content type the spec names — no 404 (a nil Prom serves zeros) and no
	// SPA HTML.
	promRR := do(t, srv, "GET", "/metrics/prom", nil)
	if promRR.Code != http.StatusOK {
		t.Fatalf("GET /metrics/prom status = %d, want 200", promRR.Code)
	}
	if ct := promRR.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Fatalf("GET /metrics/prom Content-Type = %q, want text/plain; version=0.0.4", ct)
	}
}
