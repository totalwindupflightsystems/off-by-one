package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandler_SubmitJSDedup409IsSuccess asserts /js/submit.js treats
// an HTTP 409 response whose parsed body carries
// status:"deduplicated" as a SUCCESS (the API treats dedup as success
// — the submission already exists), rendering a pointer to the
// existing submission instead of "Submission failed (HTTP 409)".
// This is the UI twin of the CLI-side DF-OFF-BY-ONE-37.
//
// The assertions are structural (the repo has no JS test harness):
// the dedup-409 success detection must appear BEFORE the generic
// failure branch, and the dedup render must name the submission_id
// and queue position.
func TestHandler_SubmitJSDedup409IsSuccess(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/js/submit.js")
	if err != nil {
		t.Fatalf("GET /js/submit.js: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "application/javascript") && !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("content-type: got %q, want javascript…", ct)
	}

	raw, _ := io.ReadAll(resp.Body)
	body := string(raw)

	for _, needle := range []string{
		// Dedup-409 success detection, distinct from the failure branch.
		"res.status === 409",
		"'deduplicated'",
		// Pointer to the existing submission.
		"Already queued",
		"submission_id",
		// Form cleared so the user doesn't re-submit the same class.
		"state.form.reset()",
	} {
		if !strings.Contains(body, needle) {
			t.Errorf("submit.js body missing %q", needle)
		}
	}

	// The dedup-409 success detection must be evaluated BEFORE the
	// generic "Submission failed (HTTP …)" failure branch, otherwise
	// dedup responses still render as failures.
	idxDedup := strings.Index(body, "res.status === 409")
	idxFail := strings.Index(body, "Submission failed (HTTP")
	if idxDedup == -1 || idxFail == -1 {
		t.Fatalf("markers missing: dedup idx %d, failure idx %d", idxDedup, idxFail)
	}
	if idxDedup >= idxFail {
		t.Errorf("dedup-409 success detection (byte %d) must precede the failure branch (byte %d)", idxDedup, idxFail)
	}

	// The failure branch must still exist for non-dedup 4xx/5xx.
	if !strings.Contains(body, "renderError(area, msg)") {
		t.Errorf("submit.js lost the failure render path")
	}
}
