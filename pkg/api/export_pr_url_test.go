package api

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestOpenAPISpec_ExportResponsePRURLIsReserved pins the DF-OFF-BY-ONE-40
// contract: pr_url is a RESERVED field. No code path can ever assign it
// (internal/api builds exportResponse with only CommitSHA and FilesChanged,
// and internal/export has no pull-request creation), so the served spec must
// describe it as reserved and never populated — it must not suggest that a
// PR URL can appear under some conditions.
func TestOpenAPISpec_ExportResponsePRURLIsReserved(t *testing.T) {
	raw, err := JSONBytes()
	if err != nil {
		t.Fatalf("JSONBytes: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}

	schema := mustMap(t, mustMap(t, mustMap(t, doc, "components"), "schemas"), "ExportResponse")
	props := mustMap(t, schema, "properties")

	// The field stays in the wire schema (least breaking contract) but must
	// be documented as reserved.
	prURL, ok := props["pr_url"].(map[string]any)
	if !ok {
		t.Fatalf("ExportResponse.properties.pr_url = %#v (%T), want a mapping", props["pr_url"], props["pr_url"])
	}
	if got := prURL["type"]; got != "string" {
		t.Errorf("pr_url.type = %#v, want %q", got, "string")
	}
	desc, _ := prURL["description"].(string)
	descLower := strings.ToLower(desc)
	if !strings.Contains(descLower, "reserved") {
		t.Errorf("pr_url.description = %q, want it to state the field is reserved", desc)
	}
	if !strings.Contains(descLower, "never") {
		t.Errorf("pr_url.description = %q, want it to state the field is never populated", desc)
	}

	// A reserved field cannot be required.
	if required, ok := schema["required"].([]any); ok {
		for _, r := range required {
			if r == "pr_url" {
				t.Errorf("pr_url must stay optional, but it is in required: %v", required)
			}
		}
	}
}

// TestDocs_ExportPRURLReservedWording keeps docs/api-reference.md honest about
// pr_url: the old sentence claimed the field "is omitted when the repo does
// not support pull requests", implying a populated PR URL is possible. No
// code can assign it (DF-OFF-BY-ONE-40), so the living documentation must
// state the field is reserved and never populated, and the misleading
// conditional-population wording must not return.
func TestDocs_ExportPRURLReservedWording(t *testing.T) {
	raw, err := os.ReadFile("../../docs/api-reference.md")
	if err != nil {
		t.Fatalf("read docs/api-reference.md: %v", err)
	}
	doc := string(raw)

	// Scope to the export endpoint section.
	const heading = "### `POST /api/v1/export`"
	start := strings.Index(doc, heading)
	if start < 0 {
		t.Fatalf("docs/api-reference.md: heading %q not found", heading)
	}
	section := doc[start:]
	if next := strings.Index(section, "\n### "); next >= 0 {
		section = section[:next]
	}

	// Negative control: the misleading conditional-population claim is gone.
	if strings.Contains(section, "does not support pull requests") {
		t.Errorf("docs/api-reference.md export section still claims pr_url is conditionally populated: %q", section)
	}

	// Positive control: the section states the field is reserved and never
	// populated.
	lower := strings.ToLower(section)
	if !strings.Contains(lower, "reserved") {
		t.Errorf("docs/api-reference.md export section does not state pr_url is reserved")
	}
	if !strings.Contains(lower, "never populated") {
		t.Errorf("docs/api-reference.md export section does not state pr_url is never populated")
	}
}
