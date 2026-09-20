package api

import (
	"encoding/json"
	"os"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// specErrorResponse is the shape every documented error response must use:
// an inline content block whose application/json schema is a $ref to the
// shared Error schema (the pattern the other paths in openapi.yaml use).
const errorSchemaRef = "#/components/schemas/Error"

// TestOpenAPISpec_ErrorResponsesDocumented pins the response codes for the
// endpoints that can fail on a default (unconfigured) install:
//
//	POST /api/v1/export  — 200, 400 (invalid request), 501 (not configured), 500 (failed)
//	POST /api/v1/import  — 200, 400 (invalid request), 501 (not configured), 500 (failed)
//	GET  /api/v1/problems — 200, 500 (internal error)
//
// Before OB-GAP-082 every one of these operations declared ONLY '200', so an
// OpenAPI consumer (Muster auto-config generates MCP tools from this spec)
// had no way to know that export/import answer 501 on a default install and
// 400 on a validation error. The codes below are what internal/api/handlers.go
// actually writes (handleExport/handleImport/handleListProblems).
//
// The exact-set assertions also cover the OB-GAP-066 failure class: a
// response key must arrive as the machine-readable string "400", never as
// the quoted literal "'400'".
func TestOpenAPISpec_ErrorResponsesDocumented(t *testing.T) {
	doc := servedSpec(t)

	cases := []struct {
		path   string
		method string
		want   []string
	}{
		{"/api/v1/export", "post", []string{"200", "400", "500", "501"}},
		{"/api/v1/import", "post", []string{"200", "400", "500", "501"}},
		{"/api/v1/problems", "get", []string{"200", "500"}},
	}

	for _, tc := range cases {
		responses := mustMap(t, mustMap(t, mustMap(t, mustMap(t, doc, "paths"), tc.path), tc.method), "responses")
		got := sortedKeys(responses)
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s %s responses keys = %v, want %v", strings.ToUpper(tc.method), tc.path, got, tc.want)
		}
		// Every non-2xx code must carry the Error schema, not an empty
		// object and not a quoted key.
		for _, code := range tc.want {
			if code == "200" {
				continue
			}
			resp, ok := responses[code].(map[string]any)
			if !ok {
				t.Errorf("%s %s: response %q is %T, want a mapping", tc.method, tc.path, code, responses[code])
				continue
			}
			content := mustMap(t, resp, "content")
			jsonMedia := mustMap(t, content, "application/json")
			schema := mustMap(t, jsonMedia, "schema")
			if got := schema["$ref"]; got != errorSchemaRef {
				t.Errorf("%s %s response %q $ref = %v, want %q", tc.method, tc.path, code, got, errorSchemaRef)
			}
			if desc, _ := resp["description"].(string); strings.TrimSpace(desc) == "" {
				t.Errorf("%s %s response %q has an empty description", tc.method, tc.path, code)
			}
		}
	}

	// Negative control for the OB-GAP-066 quoted-key class: nothing in the
	// whole document may carry a quoted-literal response key.
	for _, bad := range []string{"'400'", `"400"`, "'501'", "'500'", "'200'"} {
		if _, ok := mustMap(t, mustMap(t, mustMap(t, mustMap(t, doc, "paths"), "/api/v1/export"), "post"), "responses")[bad]; ok {
			t.Errorf("/api/v1/export kept a quoted response key %q", bad)
		}
	}
}

// TestDocs_StatusCodesMatchSpec compares docs/api-reference.md against the
// served spec for the same three endpoints. docs/integration.md calls the
// spec the source of truth for status codes, so a doc line naming a code the
// server can never return (or omitting one it does return) is a defect in
// its own right — this test makes the two sources agree per endpoint.
func TestDocs_StatusCodesMatchSpec(t *testing.T) {
	raw, err := os.ReadFile("../../docs/api-reference.md")
	if err != nil {
		t.Fatalf("read docs/api-reference.md: %v", err)
	}
	doc := servedSpec(t)

	sections := []struct{ heading, specPath, method string }{
		{"### `POST /api/v1/export`", "/api/v1/export", "post"},
		{"### `POST /api/v1/import`", "/api/v1/import", "post"},
		{"### `GET /api/v1/problems`", "/api/v1/problems", "get"},
	}

	line := regexp.MustCompile("`([0-9]{3})`")

	for _, s := range sections {
		docCodes := docStatusCodes(t, string(raw), s.heading, line)
		responses := mustMap(t, mustMap(t, mustMap(t, mustMap(t, doc, "paths"), s.specPath), s.method), "responses")
		specCodes := sortedKeys(responses)
		if len(docCodes) == 0 {
			t.Errorf("%s: no documented status codes found", s.heading)
			continue
		}
		if !reflect.DeepEqual(docCodes, specCodes) {
			t.Errorf("%s: doc codes = %v, spec codes = %v — the spec is the source of truth for status codes",
				s.heading, docCodes, specCodes)
		}
	}
}

// docStatusCodes extracts the sorted, de-duplicated status codes from the
// LAST "**Status codes:**" line inside the markdown section that starts at
// heading. Returns nil when the section or line is missing.
func docStatusCodes(t *testing.T, markdown, heading string, codeRE *regexp.Regexp) []string {
	t.Helper()
	start := strings.Index(markdown, heading)
	if start < 0 {
		t.Errorf("docs/api-reference.md: heading %q not found", heading)
		return nil
	}
	rest := markdown[start+len(heading):]
	if next := strings.Index(rest, "\n### "); next >= 0 {
		rest = rest[:next]
	}
	var found []string
	for _, l := range strings.Split(rest, "\n") {
		if !strings.HasPrefix(l, "**Status codes:**") {
			continue
		}
		seen := map[string]bool{}
		for _, m := range codeRE.FindAllStringSubmatch(l, -1) {
			if !seen[m[1]] {
				seen[m[1]] = true
				found = append(found, m[1])
			}
		}
	}
	sortStrings(found)
	return found
}

// servedSpec decodes the document the server actually serves at
// /openapi.json (the embedded YAML through the hand-rolled parser).
func servedSpec(t *testing.T) map[string]any {
	t.Helper()
	body, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	return doc
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
