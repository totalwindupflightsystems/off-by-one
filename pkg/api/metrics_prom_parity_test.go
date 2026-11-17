// ch:trace row=REVIEW-OB-024 spec=pkg/api/openapi.yaml#/metrics/prom wave=off-by-one-foreman-2026-11-16#slot-review test=pkg/api/metrics_prom_parity_test.go::TestOpenAPISpec_MetricsPromDocumented doc=pkg/api/openapi.yaml evidence=live:scratch-18772-metrics-prom-200
package api

import (
	"encoding/json"
	"reflect"
	"testing"
)

// metricsPromMediaType is the Prometheus text exposition content type in
// its OpenAPI media-type form (no charset parameter — the spec names the
// format, the server negotiates the charset at serve time).
const metricsPromMediaType = "text/plain; version=0.0.4"

// TestOpenAPISpec_MetricsPromDocumented pins REVIEW-OB-024: GET
// /metrics/prom shipped in OBS-OB-001 (commit e066e203) and answers on
// the running service, but the authoritative spec omitted it, so an
// OpenAPI consumer (Muster auto-config generates MCP tools from this
// document) could not discover the exposition surface at all.
//
// The assertions are exact-set on purpose: documenting the path as
// application/json, or leaving the content block out, would pass a bare
// "path present" check while still telling consumers the wrong thing.
func TestOpenAPISpec_MetricsPromDocumented(t *testing.T) {
	doc := servedSpec(t)

	paths := mustMap(t, doc, "paths")
	if _, ok := paths["/metrics/prom"]; !ok {
		t.Fatalf("served spec has no /metrics/prom path (%d paths documented)", len(paths))
	}

	op := mustMap(t, mustMap(t, paths, "/metrics/prom"), "get")
	if got := op["operationId"]; got != "getMetricsProm" {
		t.Errorf("GET /metrics/prom operationId = %v, want getMetricsProm", got)
	}
	if desc, _ := op["description"].(string); len(desc) < 40 {
		t.Errorf("GET /metrics/prom description = %q, want the exposition contract spelled out", desc)
	}

	responses := mustMap(t, op, "responses")
	if keys := sortedKeys(responses); !reflect.DeepEqual(keys, []string{"200"}) {
		t.Fatalf("GET /metrics/prom responses keys = %v, want [200] (the route never 404s: a nil Prom serves zeros)", keys)
	}

	content := mustMap(t, mustMap(t, responses, "200"), "content")
	if keys := sortedKeys(content); !reflect.DeepEqual(keys, []string{metricsPromMediaType}) {
		t.Fatalf("GET /metrics/prom content keys = %v, want [%q]", keys, metricsPromMediaType)
	}

	schema := mustMap(t, mustMap(t, content, metricsPromMediaType), "schema")
	if got := schema["type"]; got != "string" {
		t.Errorf("GET /metrics/prom 200 schema type = %v, want string (the body is the text exposition)", got)
	}
}

// TestOpenAPISpec_MetricsPromMediaTypeMatchesHandler closes the other half
// of the parity gap: the documented media type must be the one the handler
// actually writes. internal/api/handleMetricsProm delegates to
// (*metrics.Prom).Handler, which promhttp stamps as
// "text/plain; version=0.0.4; charset=utf-8" — so application/json here
// would be a documented lie no route-level test would catch.
func TestOpenAPISpec_MetricsPromMediaTypeMatchesHandler(t *testing.T) {
	doc := servedSpec(t)
	content := mustMap(t, mustMap(t, mustMap(t,
		mustMap(t, mustMap(t, doc, "paths"), "/metrics/prom"), "get"), "responses"), "200")
	contentTypes := mustMap(t, content, "content")

	if _, isJSON := contentTypes["application/json"]; isJSON {
		t.Errorf("/metrics/prom is documented as application/json — it serves the Prometheus text exposition")
	}
	if _, ok := contentTypes[metricsPromMediaType]; !ok {
		t.Errorf("/metrics/prom content types = %v, want %q", sortedKeys(contentTypes), metricsPromMediaType)
	}

	// /openapi.json is the serialized document consumers actually read, so
	// the path must survive the YAML→JSON conversion byte-visible.
	body, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var served map[string]any
	if err := json.Unmarshal(body, &served); err != nil {
		t.Fatalf("json.Unmarshal served document: %v", err)
	}
	if _, ok := mustMap(t, served, "paths")["/metrics/prom"]; !ok {
		t.Errorf("/openapi.json byte form does not name /metrics/prom")
	}
}
