package api

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"sort"
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

// TestYAMLParsing_QuotedKeysAndFlowSequences pins the parser itself (not just
// the shipped spec): a quoted mapping key must arrive WITHOUT its quotes and a
// flow collection must arrive as a JSON array/mapping, never as one string.
func TestYAMLParsing_QuotedKeysAndFlowSequences(t *testing.T) {
	yaml := `
paths:
  /health:
    get:
      responses:
        '200':
          description: ok
        "404":
          description: missing
        default:
          description: fallback
components:
  schemas:
    QueueEntry:
      type: object
      properties:
        status:
          type: string
          enum: [pending, in_progress, complete, failed]
      required: [status]
      tags: [queue]
edges:
  nested: [[a], [b]]
  quoted: ["a, b", c]
  empty: []
  unclosed: [a, b
  flowmap: {a: b, '200': ok}
  flowmap_empty: {}
  'a: b': colon-inside-quoted-key
  'it''s': literal-apostrophe-key
`
	var doc map[string]any
	if err := decodeYAMLInto([]byte(yaml), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// (1) Quoted keys: '200' and "404" must land as 200 and 404.
	paths := mustMap(t, doc, "paths")
	responses := mustMap(t, mustMap(t, mustMap(t, paths, "/health"), "get"), "responses")
	for _, want := range []string{"200", "404", "default"} {
		if _, ok := responses[want]; !ok {
			t.Errorf("responses missing key %q; got keys %v", want, sortedKeys(responses))
		}
	}
	for _, bad := range []string{"'200'", `"404"`} {
		if _, ok := responses[bad]; ok {
			t.Errorf("responses kept a quoted key %q (values: %v)", bad, responses[bad])
		}
	}

	// (2) Flow sequences become arrays.
	entry := mustMap(t, mustMap(t, mustMap(t, doc, "components"), "schemas"), "QueueEntry")
	props := mustMap(t, entry, "properties")
	statusProp := mustMap(t, props, "status")
	gotEnum := statusProp["enum"]
	wantEnum := []any{"pending", "in_progress", "complete", "failed"}
	if !reflect.DeepEqual(gotEnum, wantEnum) {
		t.Errorf("enum = %#v (%T), want %#v", gotEnum, gotEnum, wantEnum)
	}
	if !reflect.DeepEqual(entry["required"], []any{"status"}) {
		t.Errorf("required = %#v, want [status]", entry["required"])
	}
	if !reflect.DeepEqual(entry["tags"], []any{"queue"}) {
		t.Errorf("tags = %#v, want [queue]", entry["tags"])
	}

	// (3) Nested, quoted-comma, empty and malformed flow collections.
	edges := mustMap(t, doc, "edges")
	if !reflect.DeepEqual(edges["nested"], []any{[]any{"a"}, []any{"b"}}) {
		t.Errorf("nested = %#v, want [[a] [b]] as arrays", edges["nested"])
	}
	if !reflect.DeepEqual(edges["quoted"], []any{"a, b", "c"}) {
		t.Errorf("quoted = %#v, want [\"a, b\" c]", edges["quoted"])
	}
	empty, ok := edges["empty"].([]any)
	if !ok || len(empty) != 0 {
		t.Errorf("empty = %#v (%T), want an empty array", edges["empty"], edges["empty"])
	}
	if got, want := edges["unclosed"], "[a, b"; got != want {
		t.Errorf("unclosed = %#v, want the plain string %q (never a panic or error)", got, want)
	}
	if !reflect.DeepEqual(edges["flowmap"], map[string]any{"a": "b", "200": "ok"}) {
		t.Errorf("flowmap = %#v, want a mapping with an unquoted 200 key", edges["flowmap"])
	}
	flowEmpty, ok := edges["flowmap_empty"].(map[string]any)
	if !ok || len(flowEmpty) != 0 {
		t.Errorf("flowmap_empty = %#v (%T), want an empty mapping", edges["flowmap_empty"], edges["flowmap_empty"])
	}
	if got, want := edges["a: b"], "colon-inside-quoted-key"; got != want {
		t.Errorf("key 'a: b' = %#v, want %q (a colon inside a quoted key is not the separator)", got, want)
	}
	if got, want := edges["it's"], "literal-apostrophe-key"; got != want {
		t.Errorf("key with doubled apostrophe = %#v, want %q", got, want)
	}
}

// TestOpenAPISpec_ResponsesKeysAndEnumsAreMachineReadable walks the WHOLE
// served document so no response key or enum anywhere can regress to the
// quoted-string / stringified-array forms.
func TestOpenAPISpec_ResponsesKeysAndEnumsAreMachineReadable(t *testing.T) {
	raw, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	responseKeyRE := regexp.MustCompile(`^(default|[1-5][0-9]{2})$`)
	var responseKeys []string
	var enums []any
	var walk func(v any, at string)
	walk = func(v any, at string) {
		switch node := v.(type) {
		case map[string]any:
			if responses, ok := node["responses"].(map[string]any); ok {
				for k := range responses {
					responseKeys = append(responseKeys, k)
				}
			}
			if e, ok := node["enum"]; ok {
				enums = append(enums, e)
			}
			for k, child := range node {
				walk(child, at+"/"+k)
			}
		case []any:
			for i, child := range node {
				walk(child, fmt.Sprintf("%s/%d", at, i))
			}
		}
	}
	walk(doc, "")

	// Non-vacuity guard: the served document really does carry these
	// constructs, so an empty walk cannot pass as green.
	if len(responseKeys) < 21 {
		t.Fatalf("walked only %d response keys, want at least 21", len(responseKeys))
	}
	for _, k := range responseKeys {
		if !responseKeyRE.MatchString(k) {
			t.Errorf("response key %q is not machine-readable (want 'default' or a 3-digit code, no quotes)", k)
		}
	}
	// Five enum flow sequences reach the served document. openapi.yaml also
	// carries `enum: [...]` lines inside sequence-item inline mappings
	// (parameter schemas) whose continuation keys this parser drops — those
	// never reach the document and are tracked as a separate defect.
	if len(enums) < 5 {
		t.Fatalf("walked only %d enum values, want at least 5", len(enums))
	}
	for i, e := range enums {
		arr, ok := e.([]any)
		if !ok {
			t.Errorf("enum #%d is %T (%v), want a JSON array", i, e, e)
			continue
		}
		if len(arr) == 0 {
			t.Errorf("enum #%d is an empty array", i)
		}
	}

	// The acceptance case, spelled out: /health's responses and
	// QueueEntry.status.enum.
	health := mustMap(t, mustMap(t, mustMap(t, mustMap(t, doc, "paths"), "/health"), "get"), "responses")
	if keys := sortedKeys(health); !reflect.DeepEqual(keys, []string{"200"}) {
		t.Errorf("/health responses keys = %v, want [200]", keys)
	}
	status := mustMap(t, mustMap(t, mustMap(t, mustMap(t, mustMap(t, doc, "components"), "schemas"), "QueueEntry"), "properties"), "status")
	got := status["enum"]
	want := []any{"pending", "in_progress", "complete", "failed"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("QueueEntry.status.enum = %#v (%T), want %#v", got, got, want)
	}
}

// mustMap returns doc[key] as a mapping, failing the test when it is missing
// or of another type.
func mustMap(t *testing.T, doc map[string]any, key string) map[string]any {
	t.Helper()
	child, ok := doc[key].(map[string]any)
	if !ok {
		t.Fatalf("key %q = %#v (%T), want a mapping", key, doc[key], doc[key])
	}
	return child
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
