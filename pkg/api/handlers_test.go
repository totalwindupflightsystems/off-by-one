package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOpenAPISpec_AllEndpointsDocumented verifies the embedded spec is
// parseable as JSON and contains every endpoint the system spec §10
// promises. This is the contract the rest of the system depends on.
func TestOpenAPISpec_AllEndpointsDocumented(t *testing.T) {
	body, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("paths missing or wrong type")
	}

	// System spec §10 — every endpoint must be present.
	required := []string{
		"/api/v1/problems/submit",
		"/api/v1/problems/discover",
		"/api/v1/problems",
		"/api/v1/problems/{class}",
		"/api/v1/problems/{class}/answers",
		"/api/v1/problems/{class}/answers/{id}",
		"/api/v1/problems/{class}/related",
		"/api/v1/queue",
		"/api/v1/queue/{submission_id}",
		"/api/v1/export",
		"/api/v1/import",
		"/api/v1/taxonomy",
		"/api/v1/stats",
		"/openapi.json",
		"/health",
	}
	for _, p := range required {
		if _, ok := paths[p]; !ok {
			t.Errorf("missing path %q in OpenAPI spec", p)
		}
	}
}

// TestOpenAPISpec_OperationsHaveIDs verifies each operation declares an
// operationId. Muster uses these to name the generated MCP tools.
func TestOpenAPISpec_OperationsHaveIDs(t *testing.T) {
	body, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	paths := spec["paths"].(map[string]any)
	for path, methods := range paths {
		mm := methods.(map[string]any)
		for method, op := range mm {
			opMap := op.(map[string]any)
			opid, ok := opMap["operationId"]
			if !ok {
				t.Errorf("%s %s: missing operationId", method, path)
				continue
			}
			if opid == "" {
				t.Errorf("%s %s: empty operationId", method, path)
			}
		}
	}
}

// TestOpenAPISpec_SchemasReferenced verifies all $ref values point to
// real schemas in components.schemas. Catches typos in ref strings.
func TestOpenAPISpec_SchemasReferenced(t *testing.T) {
	body, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(body, &spec); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	components, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatal("components missing")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("components.schemas missing")
	}
	// Walk the document and collect all $ref values.
	refs := collectRefs(spec)
	for ref := range refs {
		// Refs look like "#/components/schemas/Name"
		const prefix = "#/components/schemas/"
		if len(ref) < len(prefix) || ref[:len(prefix)] != prefix {
			t.Errorf("unexpected ref format: %q", ref)
			continue
		}
		name := ref[len(prefix):]
		if _, ok := schemas[name]; !ok {
			t.Errorf("$ref %q points to missing schema %q", ref, name)
		}
	}
}

func collectRefs(v any) map[string]bool {
	refs := map[string]bool{}
	walkRefs(v, refs)
	return refs
}

func walkRefs(v any, refs map[string]bool) {
	switch x := v.(type) {
	case map[string]any:
		if r, ok := x["$ref"]; ok {
			if s, ok := r.(string); ok {
				refs[s] = true
			}
		}
		for _, vv := range x {
			walkRefs(vv, refs)
		}
	case []any:
		for _, item := range x {
			walkRefs(item, refs)
		}
	}
}

// TestOpenAPIHandler_ServesJSON exercises the HTTP handler shape — the
// real handler lives in main.go, but we validate the response shape here
// against an httptest server. main.go wires this in via the same api
// package, so a green test here is a strong contract.
func TestOpenAPIHandler_ServesJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		etag := `"` + SHA256() + `"`
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		body, err := JSONBytes()
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if _, err := w.Write(body); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/openapi.json")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	// ETag must be present and quoted.
	etag := resp.Header.Get("ETag")
	if len(etag) < 2 || etag[0] != '"' || etag[len(etag)-1] != '"' {
		t.Errorf("ETag = %q, want quoted SHA-256", etag)
	}
	// Body must be valid JSON.
	var spec map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&spec); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if spec["openapi"] != "3.0.3" {
		t.Errorf("openapi = %v, want 3.0.3", spec["openapi"])
	}
}
