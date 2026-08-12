package muster

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// validSpec is a minimal Muster-compatible OpenAPI spec.
func validSpec() map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"paths": map[string]any{
			"/api/v1/problems/submit": map[string]any{
				"post": map[string]any{
					"operationId": "submitProblem",
					"summary":     "Submit problem to queue",
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{"$ref": "#/components/schemas/SubmitProblemRequest"},
							},
						},
					},
				},
			},
			"/api/v1/problems/discover": map[string]any{
				"post": map[string]any{
					"operationId": "discoverSolution",
					"summary":     "Query for answers",
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{},
						},
					},
				},
			},
			"/api/v1/problems": map[string]any{
				"get": map[string]any{
					"operationId": "listProblems",
					"summary":     "Browse taxonomy",
				},
			},
			"/api/v1/queue/{submission_id}": map[string]any{
				"get": map[string]any{
					"operationId": "getQueueStatus",
					"summary":     "Check status",
				},
			},
			"/api/v1/problems/{class}/related": map[string]any{
				"get": map[string]any{
					"operationId": "getRelated",
					"summary":     "Related problems",
				},
			},
			"/api/v1/queue": map[string]any{
				"get": map[string]any{
					"operationId": "listQueue",
					"summary":     "List queue",
				},
			},
			"/api/v1/export": map[string]any{
				"post": map[string]any{
					"operationId": "exportToGit",
					"summary":     "Export to git",
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{},
						},
					},
				},
			},
			"/api/v1/import": map[string]any{
				"post": map[string]any{
					"operationId": "importFromGit",
					"summary":     "Import from git",
					"requestBody": map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{},
						},
					},
				},
			},
			"/api/v1/taxonomy": map[string]any{
				"get": map[string]any{
					"operationId": "getTaxonomy",
					"summary":     "Browse taxonomy",
				},
			},
			"/api/v1/stats": map[string]any{
				"get": map[string]any{
					"operationId": "getStats",
					"summary":     "Stats",
				},
			},
		},
	}
}

func TestValidateSpecDoc_ValidSpec(t *testing.T) {
	if err := ValidateSpecDoc(validSpec()); err != nil {
		t.Fatalf("valid spec failed: %v", err)
	}
}

func TestValidateSpecDoc_MissingOperationId(t *testing.T) {
	spec := validSpec()
	delete(spec["paths"].(map[string]any)["/api/v1/problems/submit"].(map[string]any)["post"].(map[string]any), "operationId")

	err := ValidateSpecDoc(spec)
	if err == nil {
		t.Fatal("expected error for missing operationId")
	}
	if !strings.Contains(err.Error(), "missing operationId") {
		t.Errorf("error should mention missing operationId: %v", err)
	}
}

func TestValidateSpecDoc_MissingRequestBody(t *testing.T) {
	spec := validSpec()
	delete(spec["paths"].(map[string]any)["/api/v1/problems/submit"].(map[string]any)["post"].(map[string]any), "requestBody")

	err := ValidateSpecDoc(spec)
	if err == nil {
		t.Fatal("expected error for missing requestBody")
	}
	if !strings.Contains(err.Error(), "missing requestBody") {
		t.Errorf("error should mention missing requestBody: %v", err)
	}
}

func TestValidateSpecDoc_RequestBodyMissingJSON(t *testing.T) {
	spec := validSpec()
	content := spec["paths"].(map[string]any)["/api/v1/problems/submit"].(map[string]any)["post"].(map[string]any)["requestBody"].(map[string]any)["content"].(map[string]any)
	delete(content, "application/json")
	content["text/plain"] = map[string]any{}

	err := ValidateSpecDoc(spec)
	if err == nil {
		t.Fatal("expected error for missing application/json")
	}
}

func TestValidateSpecDoc_MissingCoreTool(t *testing.T) {
	spec := validSpec()
	delete(spec["paths"].(map[string]any), "/api/v1/problems")

	err := ValidateSpecDoc(spec)
	if err == nil {
		t.Fatal("expected error for missing listProblems")
	}
	if !strings.Contains(err.Error(), "listProblems") {
		t.Errorf("error should mention listProblems: %v", err)
	}
}

func TestValidateSpecDoc_NoPaths(t *testing.T) {
	spec := map[string]any{
		"openapi": "3.0.3",
		"paths":   map[string]any{},
	}
	err := ValidateSpecDoc(spec)
	if err == nil {
		t.Fatal("expected error for empty paths")
	}
}

func TestValidateSpecDoc_NoPathsObject(t *testing.T) {
	spec := map[string]any{
		"openapi": "3.0.3",
	}
	err := ValidateSpecDoc(spec)
	if err == nil {
		t.Fatal("expected error for missing paths")
	}
}

func TestNewBridge(t *testing.T) {
	b := NewBridge("http://localhost:8766/")
	if b.baseURL != "http://localhost:8766" {
		t.Errorf("baseURL = %q, want trailing slash removed", b.baseURL)
	}
	if b.client == nil {
		t.Error("client is nil")
	}
	if b.maxCallLog != 100 {
		t.Errorf("maxCallLog = %d, want 100", b.maxCallLog)
	}
}

func TestBridge_MarkConnected(t *testing.T) {
	b := NewBridge("http://localhost:8766")
	if b.IsConnected() {
		t.Error("new bridge should not be connected")
	}
	b.MarkConnected(true)
	if !b.IsConnected() {
		t.Error("should be connected after MarkConnected(true)")
	}
	b.MarkConnected(false)
	if b.IsConnected() {
		t.Error("should not be connected after MarkConnected(false)")
	}
}

func TestBridge_LogToolCall(t *testing.T) {
	b := NewBridge("http://localhost:8766")

	b.LogToolCall(ToolCall{Tool: "submit_problem", Method: "POST", Path: "/api/v1/problems/submit"})
	b.LogToolCall(ToolCall{
		Tool:      "discover_solution",
		Method:    "POST",
		Path:      "/api/v1/problems/discover",
		Timestamp: time.Now(),
		Duration:  "1ms",
		Status:    200,
	})

	if b.TotalCalls() != 2 {
		t.Errorf("TotalCalls = %d, want 2", b.TotalCalls())
	}

	recent := b.RecentCalls(1)
	if len(recent) != 1 {
		t.Fatalf("RecentCalls(1) = %d items, want 1", len(recent))
	}
	if recent[0].Tool != "discover_solution" {
		t.Errorf("most recent tool = %q, want discover_solution", recent[0].Tool)
	}

	all := b.RecentCalls(0)
	if len(all) != 2 {
		t.Errorf("RecentCalls(0) = %d, want 2", len(all))
	}
}

func TestBridge_CallLogRingBuffer(t *testing.T) {
	b := NewBridge("http://localhost:8766")
	b.maxCallLog = 3

	for i := 0; i < 5; i++ {
		b.LogToolCall(ToolCall{Tool: "tool", Method: "GET", Path: "/test"})
	}

	recent := b.RecentCalls(10)
	if len(recent) != 3 {
		t.Errorf("ring buffer has %d entries, want max 3", len(recent))
	}
	if b.TotalCalls() != 5 {
		t.Errorf("TotalCalls = %d, want 5", b.TotalCalls())
	}
}

func TestMusterToolNames(t *testing.T) {
	// The full MCP tool surface Muster auto-generates from the spec.
	// Keep in lockstep with pkg/api/openapi.yaml operationIds and
	// muster-config.yaml tools.
	want := []string{
		"submit_problem", "discover_solution", "list_problems",
		"get_queue_status", "export_to_git", "import_from_git",
		"get_taxonomy", "get_stats", "get_related", "list_queue",
	}
	if len(musterToolNames) != len(want) {
		t.Fatalf("musterToolNames has %d tools, want %d", len(musterToolNames), len(want))
	}
	for i, name := range want {
		if musterToolNames[i] != name {
			t.Errorf("musterToolNames[%d] = %q, want %q", i, musterToolNames[i], name)
		}
	}
}

func TestBridge_HealthCheck_ServerDown(t *testing.T) {
	b := NewBridge("http://localhost:1") // port 1 will fail to connect
	result := b.HealthCheck(context.Background())
	if result.ServerUp {
		t.Error("ServerUp should be false for unreachable server")
	}
	if result.Error == "" {
		t.Error("Error should be set when server is unreachable")
	}
}

func TestBridge_HealthCheck_ServerUp(t *testing.T) {
	// Serve a health endpoint and a valid spec.
	spec := validSpec()
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(spec); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	b := NewBridge(srv.URL)
	result := b.HealthCheck(context.Background())
	if !result.ServerUp {
		t.Error("ServerUp should be true")
	}
	if !result.SpecValid {
		t.Errorf("SpecValid should be true, error: %s", result.Error)
	}
}

func TestBridge_HealthCheck_SpecInvalid(t *testing.T) {
	// Serve health but an invalid spec (missing operationId).
	spec := validSpec()
	delete(spec["paths"].(map[string]any)["/api/v1/problems/submit"].(map[string]any)["post"].(map[string]any), "operationId")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(spec); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	b := NewBridge(srv.URL)
	result := b.HealthCheck(context.Background())
	if !result.ServerUp {
		t.Error("ServerUp should be true")
	}
	if result.SpecValid {
		t.Error("SpecValid should be false for invalid spec")
	}
}

func TestBridge_ValidateSpec_Remote(t *testing.T) {
	spec := validSpec()
	mux := http.NewServeMux()
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(spec); err != nil {
			http.Error(w, err.Error(), 500)
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	b := NewBridge(srv.URL)
	if err := b.ValidateSpec(context.Background()); err != nil {
		t.Fatalf("ValidateSpec failed: %v", err)
	}
}
