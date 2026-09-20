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
	// Seven enum flow sequences reach the served document. openapi.yaml also
	// carried `enum: [...]` lines inside sequence-item inline mappings
	// (parameter schemas) that this parser used to drop — see
	// TestOpenAPISpec_EverySourceEnumReachesJSON, which requires the served
	// count to match the source exactly.
	if len(enums) < 7 {
		t.Fatalf("walked only %d enum values, want at least 7", len(enums))
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

// TestYAMLParsing_SequenceItemInlineMappingContinuations pins the parser-level
// defect: an inline mapping item inside a block sequence (`- name: q`) followed
// by DEEPER continuation lines (`in: query`, `required: true`, `schema:` with
// its own `enum:` flow array) must keep every continuation key. The bug was
// that the synthetic mapping built for the inline item used the SEQUENCE's
// base indent, so parseYAMLMapping consumed only the first key and the
// continuation lines — already advanced past by the collection loop — were
// silently dropped.
func TestYAMLParsing_SequenceItemInlineMappingContinuations(t *testing.T) {
	yaml := `
paths:
  /api/v1/problems:
    get:
      parameters:
        - name: q
          in: query
          required: true
          description: Full-text search query
          schema:
            type: string
            enum: [pending, verified, failed, ci_passed]
        - name: limit
          in: query
          schema:
            type: integer
            default: 20
      responses:
        '200':
          description: ok
servers:
  - url: http://localhost:8766
    description: Local dev server
items:
  - first
  - second
`
	var doc map[string]any
	if err := decodeYAMLInto([]byte(yaml), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}

	paramsAny := mustMap(t, mustMap(t, mustMap(t, doc, "paths"), "/api/v1/problems"), "get")["parameters"]
	params, ok := paramsAny.([]any)
	if !ok {
		t.Fatalf("parameters = %#v (%T), want a sequence of mappings", paramsAny, paramsAny)
	}
	if len(params) != 2 {
		t.Fatalf("parameters length = %d, want 2", len(params))
	}

	// (1) Every continuation key of the first parameter survives, and no
	// extra key appears: the key SET is asserted, not just non-emptiness.
	first, ok := params[0].(map[string]any)
	if !ok {
		t.Fatalf("parameters[0] = %#v (%T), want a mapping", params[0], params[0])
	}
	if keys := sortedKeys(first); !reflect.DeepEqual(keys, []string{"description", "in", "name", "required", "schema"}) {
		t.Errorf("parameters[0] keys = %v, want [description in name required schema]", keys)
	}
	for key, want := range map[string]any{
		"name":        "q",
		"in":          "query",
		"required":    true,
		"description": "Full-text search query",
	} {
		if got := first[key]; got != want {
			t.Errorf("parameters[0].%s = %#v, want %#v", key, got, want)
		}
	}

	// (2) The nested schema keeps its own continuation keys, including the
	// flow-sequence enum as a real array.
	schema := mustMap(t, first, "schema")
	if keys := sortedKeys(schema); !reflect.DeepEqual(keys, []string{"enum", "type"}) {
		t.Errorf("parameters[0].schema keys = %v, want [enum type]", keys)
	}
	if schema["type"] != "string" {
		t.Errorf("parameters[0].schema.type = %#v, want %q", schema["type"], "string")
	}
	if got, want := schema["enum"], []any{"pending", "verified", "failed", "ci_passed"}; !reflect.DeepEqual(got, want) {
		t.Errorf("parameters[0].schema.enum = %#v (%T), want %#v", got, got, want)
	}

	// (3) The second item's continuations survive independently: its schema
	// has no enum and carries the integer default.
	if keys := sortedKeys(params[1].(map[string]any)); !reflect.DeepEqual(keys, []string{"in", "name", "schema"}) {
		t.Errorf("parameters[1] keys = %v, want [in name schema]", keys)
	}
	second := mustMap(t, params[1].(map[string]any), "schema")
	if keys := sortedKeys(second); !reflect.DeepEqual(keys, []string{"default", "type"}) {
		t.Errorf("parameters[1].schema keys = %v, want [default type]", keys)
	}
	if got := second["type"]; got != "integer" {
		t.Errorf("parameters[1].schema.type = %#v, want %q", got, "integer")
	}
	if got := second["default"]; got != float64(20) {
		t.Errorf("parameters[1].schema.default = %#v, want 20", got)
	}
	if _, ok := params[1].(map[string]any)["required"]; ok {
		t.Errorf("parameters[1] unexpectedly carries a required key")
	}

	// (4) Non-parameter sequence items keep their continuations too.
	servers, ok := doc["servers"].([]any)
	if !ok || len(servers) != 1 {
		t.Fatalf("servers = %#v (%T), want one item", doc["servers"], doc["servers"])
	}
	serverMap, ok := servers[0].(map[string]any)
	if !ok {
		t.Fatalf("servers[0] = %#v (%T), want a mapping", servers[0], servers[0])
	}
	if keys := sortedKeys(serverMap); !reflect.DeepEqual(keys, []string{"description", "url"}) {
		t.Errorf("servers[0] keys = %v, want [description url]", keys)
	}
	if serverMap["description"] != "Local dev server" {
		t.Errorf("servers[0].description = %#v, want %q", serverMap["description"], "Local dev server")
	}

	// (5) The collection loop still advanced past the whole continuation
	// block: sibling blocks after it are intact and plain scalar items did
	// not get swallowed by the deeper mapping.
	responses := mustMap(t, mustMap(t, mustMap(t, mustMap(t, doc, "paths"), "/api/v1/problems"), "get"), "responses")
	if keys := sortedKeys(responses); !reflect.DeepEqual(keys, []string{"200"}) {
		t.Errorf("responses keys = %v, want [200]", keys)
	}
	if got := mustMap(t, responses, "200")["description"]; got != "ok" {
		t.Errorf("responses.200.description = %#v, want %q", got, "ok")
	}
	items, ok := doc["items"].([]any)
	if !ok || !reflect.DeepEqual(items, []any{"first", "second"}) {
		t.Errorf("items = %#v (%T), want [first second]", doc["items"], doc["items"])
	}
}

// TestYAMLParsing_SequenceItemBlockScalarContinuation pins the second half of
// the indent rule: the synthetic mapping's base indent must stay at the inline
// mapping's own column, so a DEEPER non-key line — block-scalar content under
// `description: |` — is still read as content of that key. Resolving the base
// indent to the shallowest CONTINUATION line instead of the mapping column
// would move the mapping past its own block scalar and lose the text (and the
// following keys with it, since the content line has no ':').
func TestYAMLParsing_SequenceItemBlockScalarContinuation(t *testing.T) {
	yaml := `
docs:
  - summary: inline
    description: |
      line one
      line two
    note: keep
  - summary: second
    description: plain text
`
	var doc map[string]any
	if err := decodeYAMLInto([]byte(yaml), &doc); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, ok := doc["docs"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("docs = %#v (%T), want two items", doc["docs"], doc["docs"])
	}
	first, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("docs[0] = %#v (%T), want a mapping", items[0], items[0])
	}
	if keys := sortedKeys(first); !reflect.DeepEqual(keys, []string{"description", "note", "summary"}) {
		t.Errorf("docs[0] keys = %v, want [description note summary]", keys)
	}
	if got, want := first["description"], "line one\nline two"; got != want {
		t.Errorf("docs[0].description = %#v, want %q", got, want)
	}
	if first["note"] != "keep" {
		t.Errorf("docs[0].note = %#v, want %q", first["note"], "keep")
	}
	second, ok := items[1].(map[string]any)
	if !ok {
		t.Fatalf("docs[1] = %#v (%T), want a mapping", items[1], items[1])
	}
	if second["description"] != "plain text" {
		t.Errorf("docs[1].description = %#v, want %q", second["description"], "plain text")
	}
}

// TestOpenAPISpec_ParametersCarryContinuationKeys walks the served document and
// asserts that OpenAPI parameter metadata — the reason this defect mattered —
// reaches JSON, not just that some parameters parse.
func TestOpenAPISpec_ParametersCarryContinuationKeys(t *testing.T) {
	raw, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	var params []map[string]any
	var walk func(v any)
	walk = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			if list, ok := node["parameters"].([]any); ok {
				for _, item := range list {
					if m, ok := item.(map[string]any); ok {
						params = append(params, m)
					}
				}
			}
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(doc)

	// Non-vacuity guard: the spec declares 17 parameters across its paths
	// (15 before GET /api/v1/queue documented its limit/offset query pair).
	if len(params) != 17 {
		t.Fatalf("walked %d parameters, want 17", len(params))
	}
	for i, p := range params {
		name, _ := p["name"].(string)
		loc, _ := p["in"].(string)
		if name == "" {
			t.Errorf("parameter #%d has no name: %#v", i, p)
		}
		if loc != "query" && loc != "path" && loc != "header" && loc != "cookie" {
			t.Errorf("parameter %q has in=%#v, want a valid location (continuation key dropped?)", name, p["in"])
		}
		if _, ok := p["schema"].(map[string]any); !ok {
			t.Errorf("parameter %q has no schema mapping: %#v", name, p["schema"])
		}
	}

	find := func(path, method, name string) map[string]any {
		t.Helper()
		op := mustMap(t, mustMap(t, mustMap(t, doc, "paths"), path), method)
		list, ok := op["parameters"].([]any)
		if !ok {
			t.Fatalf("%s %s has no parameters", method, path)
		}
		for _, item := range list {
			m := item.(map[string]any)
			if m["name"] == name {
				return m
			}
		}
		t.Fatalf("parameter %q not found on %s %s", name, method, path)
		return nil
	}

	q := find("/api/v1/problems", "get", "q")
	if q["in"] != "query" {
		t.Errorf("q.in = %#v, want query", q["in"])
	}
	if q["description"] != "Full-text search query" {
		t.Errorf("q.description = %#v, want %q", q["description"], "Full-text search query")
	}
	if got := mustMap(t, q, "schema")["type"]; got != "string" {
		t.Errorf("q.schema.type = %#v, want string", got)
	}

	status := find("/api/v1/problems", "get", "status")
	statusEnum := mustMap(t, status, "schema")["enum"]
	if got, want := statusEnum, []any{"pending", "verified", "failed", "ci_passed"}; !reflect.DeepEqual(got, want) {
		t.Errorf("status.schema.enum = %#v (%T), want %#v", got, got, want)
	}

	limit := find("/api/v1/problems", "get", "limit")
	limitSchema := mustMap(t, limit, "schema")
	if got := limitSchema["default"]; got != float64(20) {
		t.Errorf("limit.schema.default = %#v, want 20", got)
	}
	if got := limitSchema["maximum"]; got != float64(100) {
		t.Errorf("limit.schema.maximum = %#v, want 100", got)
	}

	id := find("/api/v1/problems/{class}/answers/{id}", "get", "id")
	if id["in"] != "path" || id["required"] != true {
		t.Errorf("id param = %#v, want in=path required=true", id)
	}
}

// TestOpenAPISpec_EverySourceEnumReachesJSON derives the expected enum arrays
// from the embedded source YAML (independent of the parser under test) and
// requires every one of them to appear as a non-empty JSON array in the served
// document. Two of the seven source `enum:` lines used to be dropped because
// they live inside a sequence-item inline mapping (a parameter's `schema:`).
func TestOpenAPISpec_EverySourceEnumReachesJSON(t *testing.T) {
	source := string(YAMLBytes())
	lines := regexp.MustCompile(`(?m)^[ \t]*enum:[ \t]*\[([^\]]*)\][ \t]*$`).FindAllStringSubmatch(source, -1)
	var want [][]any
	for _, m := range lines {
		var arr []any
		for _, part := range strings.Split(m[1], ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			arr = append(arr, strings.Trim(part, `"'`))
		}
		want = append(want, arr)
	}
	if len(want) != 7 {
		t.Fatalf("source openapi.yaml declares %d single-line enum arrays, want 7 (update this test with the spec)", len(want))
	}

	raw, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	var got [][]any
	var walk func(v any)
	walk = func(v any) {
		switch node := v.(type) {
		case map[string]any:
			if e, ok := node["enum"]; ok {
				arr, ok := e.([]any)
				if !ok {
					t.Errorf("enum %#v is %T, want a JSON array", e, e)
				} else {
					got = append(got, arr)
				}
			}
			for _, child := range node {
				walk(child)
			}
		case []any:
			for _, child := range node {
				walk(child)
			}
		}
	}
	walk(doc)

	if len(got) != len(want) {
		t.Errorf("served document carries %d enum arrays, source declares %d — continuation keys were dropped", len(got), len(want))
	}
	for i, arr := range got {
		if len(arr) == 0 {
			t.Errorf("served enum array #%d is empty", i)
		}
	}
	for _, w := range want {
		found := false
		for _, g := range got {
			if reflect.DeepEqual(g, w) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("source enum %v never reached the served JSON (got %v)", w, got)
		}
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
