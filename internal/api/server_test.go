package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestHealthReportsVersion pins the /health version contract the SPA
// version chip depends on (REVIEW-OB-021): web/js/app.js reads
// j.version from /health and overwrites the static v0.0.0 fallback in
// web/index.html. Without this field the chip ships a stale version
// forever. handleHealth only touches StartedAt + Version, so a bare
// Server (no store/queue) is enough here.
func TestHealthReportsVersion(t *testing.T) {
	s := &Server{
		Version:   "9.9.9-test",
		StartedAt: time.Now().Add(-90 * time.Second),
	}

	rr := do(t, s, "GET", "/health", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body["status"] != "ok" {
		t.Errorf("status field = %v, want ok", body["status"])
	}
	if got, want := body["version"], "9.9.9-test"; got != want {
		t.Errorf("version field = %v, want %q", got, want)
	}
	uptime, ok := body["uptime"].(string)
	if !ok || uptime == "" {
		t.Errorf("uptime field = %v, want a non-empty duration string", uptime)
	}
}

// ch:trace row=REVIEW-OB-022 spec=internal/api/server.go#readJSON test=internal/api/server_test.go::TestReadJSONRejectsTrailingDocument evidence=7ba22593c43eb65b61920e7ee1f438a1d063dd7d

// readJSONTarget is the minimal request shape for the readJSON unit
// tests below. It carries one tagged field plus a numeric field so the
// decoder flags that matter (unknown-field rejection, duplicate keys)
// can be exercised without a Server, store, or queue.
type readJSONTarget struct {
	ProblemClass string `json:"problem_class"`
	Count        int    `json:"count"`
}

func newReadJSONRequest(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/api/v1/problems/discover", strings.NewReader(body))
}

// TestReadJSONRejectsTrailingDocument pins REVIEW-OB-022: readJSON
// returned nil for {"problem_class":"unknown"}{"junk":1} because it
// decoded exactly once and never checked that the stream was exhausted,
// so a second concatenated document was silently dropped. Every body
// below decodes successfully on the first value and then carries
// trailing *data*, which must now be an error.
func TestReadJSONRejectsTrailingDocument(t *testing.T) {
	bodies := []string{
		`{"problem_class":"unknown"}{"junk":1}`,
		`{"problem_class":"unknown"} {"junk":1}`,
		"{\"problem_class\":\"unknown\"}\n{\"junk\":1}",
		`{"problem_class":"unknown"}null`,
		`{"problem_class":"unknown"}[]`,
		`{"problem_class":"unknown"}true`,
		`{"problem_class":"unknown"}42`,
		`{"problem_class":"unknown"},"count":1`,
	}
	for _, body := range bodies {
		var got readJSONTarget
		err := readJSON(newReadJSONRequest(body), &got)
		if err == nil {
			t.Errorf("readJSON(%s) = nil, want trailing-data error", body)
			continue
		}
		if !strings.Contains(err.Error(), "trailing data") {
			t.Errorf("readJSON(%s) error = %q, want it to mention \"trailing data\"", body, err)
		}
		if !strings.HasPrefix(err.Error(), "invalid JSON: ") {
			t.Errorf("readJSON(%s) error = %q, want the file's \"invalid JSON: \" prefix convention", body, err)
		}
	}
}

// TestReadJSONAcceptsTrailingWhitespace pins the other half of
// REVIEW-OB-022: a whitespace-only tail was accepted before the fix and
// must stay accepted. dec.Token() reports io.EOF once the only thing
// left in the stream is whitespace.
func TestReadJSONAcceptsTrailingWhitespace(t *testing.T) {
	bodies := []string{
		`{"problem_class":"unknown"}`,
		"{\"problem_class\":\"unknown\"}\n",
		"{\"problem_class\":\"unknown\"}   ",
		"{\"problem_class\":\"unknown\"}	\r\n  \n",
		"  \n{\"problem_class\":\"unknown\"}\n\n",
	}
	for _, body := range bodies {
		var got readJSONTarget
		if err := readJSON(newReadJSONRequest(body), &got); err != nil {
			t.Errorf("readJSON(%s) = %v, want nil", body, err)
			continue
		}
		if got.ProblemClass != "unknown" {
			t.Errorf("readJSON(%s) problem_class = %q, want unknown", body, got.ProblemClass)
		}
	}
}

// TestReadJSONDuplicateKeysLastWins keeps the decoder semantics callers
// relied on before REVIEW-OB-022: duplicate keys are one document (the
// last value wins), not trailing data. The new stream check must not
// start rejecting them.
func TestReadJSONDuplicateKeysLastWins(t *testing.T) {
	var got readJSONTarget
	if err := readJSON(newReadJSONRequest(`{"problem_class":"first","problem_class":"second","count":1,"count":2}`), &got); err != nil {
		t.Fatalf("readJSON = %v, want nil (duplicate keys are still valid JSON)", err)
	}
	if got.ProblemClass != "second" {
		t.Errorf("problem_class = %q, want the last value %q", got.ProblemClass, "second")
	}
	if got.Count != 2 {
		t.Errorf("count = %d, want the last value 2", got.Count)
	}
}

// TestReadJSONRejectsUnknownField pins that DisallowUnknownFields
// survived the REVIEW-OB-022 change.
func TestReadJSONRejectsUnknownField(t *testing.T) {
	var got readJSONTarget
	err := readJSON(newReadJSONRequest(`{"problem_class":"unknown","bogus":1}`), &got)
	if err == nil {
		t.Fatal("readJSON = nil, want an unknown-field error")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Errorf("error = %q, want it to mention the unknown field", err)
	}
}

// TestReadJSONKeepsSizeCap pins that the 1MiB MaxBytesReader cap is
// still in place after the fix.
func TestReadJSONKeepsSizeCap(t *testing.T) {
	body := `{"problem_class":"` + strings.Repeat("a", 2<<20) + `"}`
	var got readJSONTarget
	if err := readJSON(newReadJSONRequest(body), &got); err == nil {
		t.Fatal("readJSON = nil for a body over the 1MiB cap, want an error")
	}
}

// TestDiscoverRejectsTrailingDocumentAs400 pins the observable contract
// from the board row: the body that returned 200 on the lab now returns
// 400 invalid_request. handleDiscover maps every readJSON error to 400,
// and the trailing-document path returns before Store is read, so a
// bare Server is enough.
func TestDiscoverRejectsTrailingDocumentAs400(t *testing.T) {
	s := &Server{}
	rr := httptest.NewRecorder()
	s.handleDiscover(rr, newReadJSONRequest(`{"problem_class":"unknown"}{"junk":1}`))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] != "invalid_request" {
		t.Errorf("error code = %q, want invalid_request", body["error"])
	}
	if !strings.Contains(body["message"], "trailing data") {
		t.Errorf("message = %q, want it to mention trailing data", body["message"])
	}
}
