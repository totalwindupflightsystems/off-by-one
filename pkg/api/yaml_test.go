package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestJSONBytes_MatchesYAMLOpenAPISpec(t *testing.T) {
	got, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(got, &spec); err != nil {
		t.Fatalf("JSON unmarshal: %v", err)
	}
	if got := spec["openapi"]; got != "3.0.3" {
		t.Errorf("openapi version = %v, want 3.0.3", got)
	}
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatalf("paths not present or wrong type: %T", spec["paths"])
	}
	// Spec must include every system-spec §10 endpoint.
	wantPaths := []string{
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
	for _, p := range wantPaths {
		if _, ok := paths[p]; !ok {
			t.Errorf("missing path %q in spec", p)
		}
	}
}

func TestYAMLBytes_NonEmpty(t *testing.T) {
	if len(YAMLBytes()) == 0 {
		t.Fatal("YAMLBytes returned empty")
	}
}

func TestSHA256_Stable(t *testing.T) {
	a := SHA256()
	if a == "" {
		t.Fatal("SHA256 returned empty")
	}
	if a != SHA256() {
		t.Error("SHA256 not stable across calls")
	}
	// SHA-256 hex = 64 chars.
	if len(a) != 64 {
		t.Errorf("SHA256 length = %d, want 64", len(a))
	}
}

func TestYAMLParsing_BasicTypes(t *testing.T) {
	// We use decodeYAMLInto + a target struct. This catches round-trip
	// behaviour for scalars, sequences, and mappings.
	cases := []struct {
		name    string
		yaml    string
		want    any
		checkFn func(t *testing.T, v any)
	}{
		{
			name: "string",
			yaml: "hello world\n",
			checkFn: func(t *testing.T, v any) {
				if v != "hello world" {
					t.Errorf("got %v (%T), want %q", v, v, "hello world")
				}
			},
		},
		{
			name: "integer",
			yaml: "42\n",
			checkFn: func(t *testing.T, v any) {
				if v != float64(42) {
					t.Errorf("got %v (%T), want 42", v, v)
				}
			},
		},
		{
			name: "float",
			yaml: "3.14\n",
			checkFn: func(t *testing.T, v any) {
				if v != 3.14 {
					t.Errorf("got %v (%T), want 3.14", v, v)
				}
			},
		},
		{
			name: "bool true",
			yaml: "true\n",
			checkFn: func(t *testing.T, v any) {
				if v != true {
					t.Errorf("got %v, want true", v)
				}
			},
		},
		{
			name: "bool false (no)",
			yaml: "no\n",
			checkFn: func(t *testing.T, v any) {
				if v != false {
					t.Errorf("got %v, want false", v)
				}
			},
		},
		{
			name: "null",
			yaml: "~\n",
			checkFn: func(t *testing.T, v any) {
				if v != nil {
					t.Errorf("got %v, want nil", v)
				}
			},
		},
		{
			name: "double quoted string with escape",
			yaml: "\"hello \\\"world\\\"\"\n",
			checkFn: func(t *testing.T, v any) {
				if v != "hello \"world\"" {
					t.Errorf("got %q, want %q", v, "hello \"world\"")
				}
			},
		},
		{
			name: "single quoted string",
			yaml: "'literal text'\n",
			checkFn: func(t *testing.T, v any) {
				if v != "literal text" {
					t.Errorf("got %q", v)
				}
			},
		},
		{
			name: "sequence",
			yaml: "- 1\n- 2\n- 3\n",
			checkFn: func(t *testing.T, v any) {
				got, ok := v.([]any)
				if !ok {
					t.Fatalf("got %T, want []any", v)
				}
				if len(got) != 3 || got[0] != float64(1) || got[1] != float64(2) || got[2] != float64(3) {
					t.Errorf("got %v", got)
				}
			},
		},
		{
			name: "mapping",
			yaml: "key1: hello\nkey2: 42\n",
			checkFn: func(t *testing.T, v any) {
				got, ok := v.(map[string]any)
				if !ok {
					t.Fatalf("got %T, want map[string]any", v)
				}
				if got["key1"] != "hello" {
					t.Errorf("key1 = %v", got["key1"])
				}
				if got["key2"] != float64(42) {
					t.Errorf("key2 = %v", got["key2"])
				}
			},
		},
		{
			name: "nested mapping with sequence",
			yaml: "outer:\n  inner:\n    - a\n    - b\n",
			checkFn: func(t *testing.T, v any) {
				got, ok := v.(map[string]any)
				if !ok {
					t.Fatalf("got %T", v)
				}
				outer, ok := got["outer"].(map[string]any)
				if !ok {
					t.Fatalf("outer = %T", got["outer"])
				}
				inner, ok := outer["inner"].([]any)
				if !ok {
					t.Fatalf("inner = %T", outer["inner"])
				}
				if len(inner) != 2 || inner[0] != "a" || inner[1] != "b" {
					t.Errorf("inner = %v", inner)
				}
			},
		},
		{
			name: "inline comment stripped",
			yaml: "key: value # this is a comment\n",
			checkFn: func(t *testing.T, v any) {
				got := v.(map[string]any)
				if got["key"] != "value" {
					t.Errorf("got %v, want value", got["key"])
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			if err := decodeYAMLInto([]byte(tc.yaml), &v); err != nil {
				t.Fatalf("decodeYAMLInto: %v", err)
			}
			tc.checkFn(t, v)
		})
	}
}

func TestYAMLParsing_OpenAPISpecSubset(t *testing.T) {
	// A focused integration test: parse a realistic OpenAPI-shaped document
	// and assert the structure survives the YAML→JSON round-trip.
	yaml := `
openapi: 3.0.3
info:
  title: Test API
  version: 0.1.0
paths:
  /api/v1/problems:
    get:
      operationId: listProblems
      summary: List problems
  /api/v1/problems/{class}/answers/{id}:
    get:
      operationId: getAnswer
      summary: Get answer
`
	var doc map[string]any
	if err := decodeYAMLInto([]byte(yaml), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if doc["openapi"] != "3.0.3" {
		t.Errorf("openapi = %v", doc["openapi"])
	}
	info := doc["info"].(map[string]any)
	if info["title"] != "Test API" {
		t.Errorf("title = %v", info["title"])
	}
	paths := doc["paths"].(map[string]any)
	if len(paths) != 2 {
		t.Errorf("paths len = %d, want 2", len(paths))
	}
	// Path with two path params.
	answers := paths["/api/v1/problems/{class}/answers/{id}"].(map[string]any)
	get := answers["get"].(map[string]any)
	if get["operationId"] != "getAnswer" {
		t.Errorf("operationId = %v", get["operationId"])
	}
}

func TestYAMLParsing_TabInIndentErrors(t *testing.T) {
	yaml := "key:\n\tvalue: 1\n"
	var doc any
	err := decodeYAMLInto([]byte(yaml), &doc)
	if err == nil {
		t.Fatal("expected error on tab indentation, got nil")
	}
	if !strings.Contains(err.Error(), "tab") {
		t.Errorf("error %q does not mention tab", err.Error())
	}
}

func TestJSONBytes_RoundTrip(t *testing.T) {
	// We can re-parse JSONBytes output through json.Unmarshal. This proves
	// the YAML→JSON conversion produces a well-formed JSON document.
	first, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var v any
	if err := json.Unmarshal(first, &v); err != nil {
		t.Fatalf("re-unmarshal failed: %v\nfirst 200 bytes: %s", err, first[:min(200, len(first))])
	}
}
