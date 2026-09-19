package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
	schemasql "github.com/totalwindupflightsystems/off-by-one/sql/schema"
)

// newTestServer builds a Server backed by an in-memory SQLite with
// both graph + queue tables. Returns the server, the underlying
// store, the queue, and a cleanup func that closes the DB.
func newTestServer(t *testing.T) (*Server, *graph.Store, *ingest.Queue) {
	t.Helper()
	store, err := graph.OpenShared("api_test_" + t.Name() + "_" + time.Now().Format("150405.000000"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.ApplyExtra(schemasql.QueueSchema); err != nil {
		t.Fatalf("apply queue schema: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	queue, err := ingest.Open(store)
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	srv := New(store, queue, []byte("openapi: 3.0.3\ninfo: {title: test}\n"))
	srv.SolverAvailable = true
	return srv, store, queue
}

// do is a small helper that runs an HTTP request against the test
// server and returns the response.
func do(t *testing.T, s *Server, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	return rr
}

// seedClass inserts a problem class + an answer for use in tests
// that need pre-existing graph data.
func seedClass(t *testing.T, store *graph.Store, title, desc, env, lang, version, solution, status string) (int64, int64) {
	t.Helper()
	pc, _, err := store.UpsertProblemClass(context.Background(), title, desc)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	id, err := store.CreateAnswerNode(context.Background(), pc.ID, 0, env, lang, version, solution, "evidence: test", "{}")
	if err != nil {
		t.Fatalf("create answer: %v", err)
	}
	if err := store.UpdateAnswerStatus(context.Background(), id, status); err != nil {
		t.Fatalf("update status: %v", err)
	}
	return pc.ID, id
}

// seedClassWithSigs is seedClass with an explicit signatures JSON blob,
// for tests that need to exercise signature-aware aggregation
// (OB-GAP-060: failed-signature answers must not count as verified).
func seedClassWithSigs(t *testing.T, store *graph.Store, title, desc, env, lang, version, solution, status, signatures string) (int64, int64) {
	t.Helper()
	pc, _, err := store.UpsertProblemClass(context.Background(), title, desc)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	id, err := store.CreateAnswerNode(context.Background(), pc.ID, 0, env, lang, version, solution, "evidence: test", signatures)
	if err != nil {
		t.Fatalf("create answer: %v", err)
	}
	if err := store.UpdateAnswerStatus(context.Background(), id, status); err != nil {
		t.Fatalf("update status: %v", err)
	}
	return pc.ID, id
}

// --- Health + OpenAPI ----------------------------------------------------

func TestHealth(t *testing.T) {
	s, _, _ := newTestServer(t)
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
}

func TestOpenAPI(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "GET", "/openapi.json", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "openapi: 3.0.3") {
		t.Errorf("body = %q, expected openapi spec", rr.Body.String())
	}
}

// --- Submit --------------------------------------------------------------

func TestSubmit_Queued(t *testing.T) {
	s, _, _ := newTestServer(t)
	body := submitProblemRequest{
		ProblemClass: "docker-volume-permissions",
		Environment:  "docker",
		Language:     "go",
		Cadence:      ingest.CadencePrePhase,
	}
	rr := do(t, s, "POST", "/api/v1/problems/submit", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rr.Code, rr.Body.String())
	}
	var resp submitProblemResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "queued" {
		t.Errorf("status = %q, want queued", resp.Status)
	}
	if resp.ProblemClass != "docker-volume-permissions" {
		t.Errorf("problem_class = %q, want docker-volume-permissions", resp.ProblemClass)
	}
	if resp.SubmissionID == "" {
		t.Error("submission_id is empty")
	}
}

func TestSubmit_SolverUnavailable(t *testing.T) {
	s, _, _ := newTestServer(t)
	s.SolverAvailable = false
	body := submitProblemRequest{
		ProblemClass: "docker-volume-permissions",
		Environment:  "docker",
		Language:     "go",
		Cadence:      ingest.CadencePrePhase,
	}
	rr := do(t, s, "POST", "/api/v1/problems/submit", body)
	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503, body = %s", rr.Code, rr.Body.String())
	}
	var e struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if e.Error != "solver_unavailable" {
		t.Errorf("error = %q, want solver_unavailable", e.Error)
	}
	if e.Message == "" {
		t.Error("message is empty")
	}
	// Nothing may be enqueued — the rejection must happen before any
	// queue write so no pending row is stranded without a solver.
	depth, _ := s.Queue.Depth(context.Background())
	if depth != 0 {
		t.Errorf("queue depth = %d, want 0 (no submission enqueued)", depth)
	}
}

func TestSubmit_SlugifiesProblemClass(t *testing.T) {
	s, _, _ := newTestServer(t)
	body := submitProblemRequest{
		ProblemClass: "Docker Volume Permissions!!!",
		Cadence:      ingest.CadencePrePhase,
	}
	rr := do(t, s, "POST", "/api/v1/problems/submit", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp submitProblemResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.ProblemClass != "docker-volume-permissions" {
		t.Errorf("slug = %q, want docker-volume-permissions", resp.ProblemClass)
	}
}

func TestSubmit_DedupPending(t *testing.T) {
	s, _, q := newTestServer(t)
	body := submitProblemRequest{
		ProblemClass: "docker-perms",
		Cadence:      ingest.CadencePrePhase,
	}
	rr1 := do(t, s, "POST", "/api/v1/problems/submit", body)
	if rr1.Code != http.StatusOK {
		t.Fatalf("first submit: %d", rr1.Code)
	}
	// Second submit with same class + same (empty) env/lang/version
	// should dedup against the pending entry.
	rr2 := do(t, s, "POST", "/api/v1/problems/submit", body)
	if rr2.Code != http.StatusConflict {
		t.Fatalf("dedup status = %d, want 409", rr2.Code)
	}
	var resp submitProblemResponse
	_ = json.Unmarshal(rr2.Body.Bytes(), &resp)
	if resp.Status != "deduplicated" {
		t.Errorf("status = %q, want deduplicated", resp.Status)
	}
	if resp.SubmissionID == "" {
		t.Error("dedup response should include the existing submission_id")
	}
	// Verify the queue only has 1 entry.
	depth, _ := q.Depth(context.Background())
	if depth != 1 {
		t.Errorf("queue depth = %d, want 1", depth)
	}
}

func TestSubmit_DedupVerifiedAnswer(t *testing.T) {
	s, store, _ := newTestServer(t)
	// Pre-seed a verified answer for "docker-perms".
	seedClass(t, store, "docker-perms", "desc", "docker", "go", "1.0", "use --user flag", graph.AnswerVerified)

	body := submitProblemRequest{
		ProblemClass: "docker-perms",
		Environment:  "docker",
		Language:     "go",
		Version:      "1.0",
		Cadence:      ingest.CadencePrePhase,
	}
	rr := do(t, s, "POST", "/api/v1/problems/submit", body)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 (verified dedup)", rr.Code)
	}
	var resp submitProblemResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Status != "deduplicated" {
		t.Errorf("status = %q, want deduplicated", resp.Status)
	}
	if resp.ExistingSolutions < 1 {
		t.Errorf("existing_solutions = %d, want >= 1", resp.ExistingSolutions)
	}
}

func TestSubmit_InvalidCadence(t *testing.T) {
	s, _, _ := newTestServer(t)
	body := submitProblemRequest{
		ProblemClass: "anything",
		Cadence:      "totally-not-valid",
	}
	rr := do(t, s, "POST", "/api/v1/problems/submit", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestSubmit_MissingProblemClass(t *testing.T) {
	s, _, _ := newTestServer(t)
	body := submitProblemRequest{Cadence: ingest.CadencePrePhase}
	rr := do(t, s, "POST", "/api/v1/problems/submit", body)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// TestSubmit_RequiredTools verifies that the submit endpoint stores
// required_tools in the queue entry (AC1 — SBOX-002).
func TestSubmit_RequiredTools(t *testing.T) {
	s, _, q := newTestServer(t)
	body := submitProblemRequest{
		ProblemClass:  "jq-parsing-error",
		Environment:   "linux",
		Language:      "bash",
		Cadence:       ingest.CadencePrePhase,
		RequiredTools: []string{"jq", "parallel"},
	}
	rr := do(t, s, "POST", "/api/v1/problems/submit", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rr.Code, rr.Body.String())
	}
	var resp submitProblemResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != "queued" {
		t.Errorf("status = %q, want queued", resp.Status)
	}
	// Verify the queue entry carries the required_tools.
	entry, err := q.Get(context.Background(), resp.SubmissionID)
	if err != nil {
		t.Fatalf("queue.Get: %v", err)
	}
	if len(entry.RequiredTools) != 2 {
		t.Fatalf("RequiredTools = %v, want 2 items", entry.RequiredTools)
	}
	if entry.RequiredTools[0] != "jq" || entry.RequiredTools[1] != "parallel" {
		t.Errorf("RequiredTools = %v, want [jq parallel]", entry.RequiredTools)
	}
}

// TestSubmit_RequiredTools_Empty verifies that omitting required_tools
// produces an empty (not nil) slice in the entry — the column has a
// DEFAULT '[]'.
func TestSubmit_RequiredTools_Empty(t *testing.T) {
	s, _, q := newTestServer(t)
	body := submitProblemRequest{
		ProblemClass: "no-tools-needed",
		Cadence:      ingest.CadencePrePhase,
	}
	rr := do(t, s, "POST", "/api/v1/problems/submit", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp submitProblemResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	entry, err := q.Get(context.Background(), resp.SubmissionID)
	if err != nil {
		t.Fatalf("queue.Get: %v", err)
	}
	if len(entry.RequiredTools) != 0 {
		t.Errorf("RequiredTools = %v, want empty", entry.RequiredTools)
	}
}

func TestSubmit_InvalidJSON(t *testing.T) {
	s, _, _ := newTestServer(t)
	req := httptest.NewRequest("POST", "/api/v1/problems/submit", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// --- Discover ------------------------------------------------------------

func TestDiscover_Found(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "docker-perms", "permissions", "docker", "go", "1.0", "use --user", graph.AnswerVerified)
	body := discoverRequest{
		ProblemClass: "docker-perms",
		Environment:  "docker",
		Language:     "go",
		Version:      "1.0",
	}
	rr := do(t, s, "POST", "/api/v1/problems/discover", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rr.Code, rr.Body.String())
	}
	var resp discoverResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Found {
		t.Error("found = false, want true")
	}
	if resp.Answer == nil {
		t.Fatal("answer is nil")
	}
	if resp.Answer.Solution != "use --user" {
		t.Errorf("solution = %q", resp.Answer.Solution)
	}
}

// boolPtr returns a pointer to b, for the optional include_related
// request field (a *bool so "absent" is distinguishable from false).
func boolPtr(b bool) *bool { return &b }

// TestDiscover_EmptyArraysPresent locks the published DiscoverResponse
// contract: a successful discover always carries `related` and
// `version_warnings` as JSON arrays, even when the class has no edges
// and no version warnings (OB-GAP-058).
func TestDiscover_EmptyArraysPresent(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "no-edges", "no edges", "docker", "go", "1.0", "sol", graph.AnswerVerified)
	body := discoverRequest{
		ProblemClass: "no-edges",
		Environment:  "docker",
		Language:     "go",
		Version:      "1.0",
	}
	rr := do(t, s, "POST", "/api/v1/problems/discover", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rr.Code, rr.Body.String())
	}

	// Raw-body check: both keys must exist and be literal empty arrays.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v, body = %s", err, rr.Body.String())
	}
	for _, key := range []string{"related", "version_warnings"} {
		v, ok := raw[key]
		if !ok {
			t.Errorf("body = %s: key %q missing, want an array", rr.Body.String(), key)
			continue
		}
		if got := strings.TrimSpace(string(v)); got != "[]" {
			t.Errorf("%s = %s, want [] (empty array, not omitted/null)", key, got)
		}
	}

	// Typed check: decoding must not yield nil slices (the wire value
	// was null) — the empty-array contract is what clients depend on.
	var resp discoverResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode typed: %v", err)
	}
	if resp.Related == nil {
		t.Error("related decoded nil; want non-nil empty slice")
	}
	if resp.VersionWarnings == nil {
		t.Error("version_warnings decoded nil; want non-nil empty slice")
	}
}

// TestDiscover_RelatedEdgeHonored verifies populated related edges
// still surface, and that include_related=false yields an EMPTY ARRAY
// (not an omitted key) rather than dropping the field.
func TestDiscover_RelatedEdgeHonored(t *testing.T) {
	s, store, _ := newTestServer(t)
	classA, _ := seedClass(t, store, "class-a", "a", "docker", "go", "1.0", "solA", graph.AnswerVerified)
	classB, _ := seedClass(t, store, "class-b", "b", "docker", "go", "1.0", "solB", graph.AnswerVerified)
	if _, err := store.CreateEdge(context.Background(), classA, classB, graph.EdgeSameRootCause, 0.8); err != nil {
		t.Fatalf("create edge: %v", err)
	}

	// include_related=true -> the seeded edge is returned.
	rr := do(t, s, "POST", "/api/v1/problems/discover", discoverRequest{
		ProblemClass:   "class-a",
		Environment:    "docker",
		Language:       "go",
		Version:        "1.0",
		IncludeRelated: boolPtr(true),
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("include_related=true: status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var withEdges discoverResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &withEdges); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(withEdges.Related) != 1 {
		t.Fatalf("include_related=true: related len = %d, want 1, body = %s", len(withEdges.Related), rr.Body.String())
	}
	if withEdges.Related[0].ProblemClass != "class-b" {
		t.Errorf("related[0].problem_class = %q, want class-b", withEdges.Related[0].ProblemClass)
	}
	if withEdges.Related[0].Relationship != graph.EdgeSameRootCause {
		t.Errorf("related[0].relationship = %q, want %q", withEdges.Related[0].Relationship, graph.EdgeSameRootCause)
	}

	// include_related=false -> key present, array empty.
	rr = do(t, s, "POST", "/api/v1/problems/discover", discoverRequest{
		ProblemClass:   "class-a",
		Environment:    "docker",
		Language:       "go",
		Version:        "1.0",
		IncludeRelated: boolPtr(false),
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("include_related=false: status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	v, ok := raw["related"]
	if !ok {
		t.Fatalf("include_related=false: body = %s, want key \"related\" present", rr.Body.String())
	}
	if got := strings.TrimSpace(string(v)); got != "[]" {
		t.Errorf("include_related=false: related = %s, want []", got)
	}
	var withoutEdges discoverResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &withoutEdges); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(withoutEdges.Related) != 0 {
		t.Errorf("include_related=false: related len = %d, want 0", len(withoutEdges.Related))
	}
	if withoutEdges.Related == nil {
		t.Error("include_related=false: related decoded nil; want non-nil empty slice")
	}
}

// TestDiscover_VersionWarning verifies a seeded superseded_by edge to a
// class whose verified answer is a different version produces a
// non-empty version_warnings array.
func TestDiscover_VersionWarning(t *testing.T) {
	s, store, _ := newTestServer(t)
	classA, _ := seedClass(t, store, "class-a", "a", "docker", "go", "1.0", "solA", graph.AnswerVerified)
	classB, _ := seedClass(t, store, "class-b", "b", "docker", "go", "2.0", "solB", graph.AnswerVerified)
	if _, err := store.CreateEdge(context.Background(), classA, classB, graph.EdgeSupersededBy, 0.9); err != nil {
		t.Fatalf("create edge: %v", err)
	}
	rr := do(t, s, "POST", "/api/v1/problems/discover", discoverRequest{
		ProblemClass: "class-a",
		Environment:  "docker",
		Language:     "go",
		Version:      "1.0",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var resp discoverResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.VersionWarnings) == 0 {
		t.Fatalf("version_warnings = %v, want at least one warning, body = %s", resp.VersionWarnings, rr.Body.String())
	}
	if !strings.Contains(resp.VersionWarnings[0], "2.0") {
		t.Errorf("version_warnings[0] = %q, want it to mention the newer version 2.0", resp.VersionWarnings[0])
	}
}

// TestDiscover_VersionWarningsEmptyPresent verifies version_warnings is
// still an empty array (not omitted, not null) when the superseded_by
// target carries no differing version.
func TestDiscover_VersionWarningsEmptyPresent(t *testing.T) {
	s, store, _ := newTestServer(t)
	classA, _ := seedClass(t, store, "class-a", "a", "docker", "go", "1.0", "solA", graph.AnswerVerified)
	classB, _ := seedClass(t, store, "class-b", "b", "docker", "go", "1.0", "solB", graph.AnswerVerified)
	if _, err := store.CreateEdge(context.Background(), classA, classB, graph.EdgeSupersededBy, 0.9); err != nil {
		t.Fatalf("create edge: %v", err)
	}
	rr := do(t, s, "POST", "/api/v1/problems/discover", discoverRequest{
		ProblemClass: "class-a",
		Environment:  "docker",
		Language:     "go",
		Version:      "1.0",
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw: %v", err)
	}
	v, ok := raw["version_warnings"]
	if !ok {
		t.Fatalf("body = %s, want key \"version_warnings\" present", rr.Body.String())
	}
	if got := strings.TrimSpace(string(v)); got != "[]" {
		t.Errorf("version_warnings = %s, want [] (same version on both sides -> no warning)", got)
	}
}

func TestDiscover_NotFound(t *testing.T) {
	s, _, _ := newTestServer(t)
	body := discoverRequest{ProblemClass: "no-such-class"}
	rr := do(t, s, "POST", "/api/v1/problems/discover", body)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// TestDiscover_PlaceholderClass_NotFound verifies that a placeholder
// (self-test) class with a verified answer in the DB is served exactly
// like an unknown class: 404 not_found (OB-GAP-061). Regression: a normal
// class still discovers 200 found:true (covered by TestDiscover_Found).
func TestDiscover_PlaceholderClass_NotFound(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "off-by-one-self-test", "probe", "", "", "", "placeholder solution", graph.AnswerVerified)
	rr := do(t, s, "POST", "/api/v1/problems/discover", discoverRequest{ProblemClass: "off-by-one-self-test"})
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", rr.Code, rr.Body.String())
	}
	var errBody map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if errBody["error"] != "not_found" {
		t.Errorf("error = %v, want not_found", errBody["error"])
	}
}

// --- Read-only catalog mode (OB-GAP-020) ---------------------------------

// Discovery is a pure read, so it must keep working in read-only catalog
// mode — the agent-discovery workflow depends on it.
func TestReadOnly_DiscoverAllowed(t *testing.T) {
	s, store, _ := newTestServer(t)
	s.ReadOnly = true
	seedClass(t, store, "docker-perms", "permissions", "docker", "go", "1.0", "use --user", graph.AnswerVerified)
	body := discoverRequest{
		ProblemClass: "docker-perms",
		Environment:  "docker",
		Language:     "go",
		Version:      "1.0",
	}
	rr := do(t, s, "POST", "/api/v1/problems/discover", body)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (discover is a pure read), body = %s", rr.Code, rr.Body.String())
	}
	var resp discoverResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Found {
		t.Error("found = false, want true")
	}
}

// Every other POST endpoint is a write and stays blocked in read-only
// mode, along with the AI chat WebSocket.
func TestReadOnly_MutatingEndpointsBlocked(t *testing.T) {
	s, _, _ := newTestServer(t)
	s.ReadOnly = true
	for _, path := range []string{
		"/api/v1/problems/submit",
		"/api/v1/export",
		"/api/v1/import",
		"/ws/chat",
	} {
		rr := do(t, s, "POST", path, map[string]any{"problem_class": "x"})
		if rr.Code != http.StatusForbidden {
			t.Errorf("POST %s: status = %d, want 403", path, rr.Code)
			continue
		}
		var body map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Fatalf("POST %s: decode: %v", path, err)
		}
		if body["error"] != "read_only" {
			t.Errorf("POST %s: error = %v, want read_only", path, body["error"])
		}
	}
}

// --- List problems -------------------------------------------------------

func TestListProblems_Empty(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "GET", "/api/v1/problems", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp listProblemsResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Total)
	}
	if resp.Problems == nil {
		t.Errorf("problems is nil; want non-nil empty slice (serializes as null)")
	}
	if !strings.Contains(rr.Body.String(), `"problems":[]`) {
		t.Errorf("body = %s, want it to contain `\"problems\":[]`", rr.Body.String())
	}
}

func TestListProblems_WithData(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "docker-perms", "desc1", "docker", "go", "1.0", "sol1", graph.AnswerVerified)
	seedClass(t, store, "chown-dockerfile", "desc2", "docker", "dockerfile", "1.0", "sol2", graph.AnswerPending)
	rr := do(t, s, "GET", "/api/v1/problems", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp listProblemsResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Total)
	}
}

func TestListProblems_SearchQuery(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "docker-permissions", "fixing permissions in containers", "docker", "go", "1.0", "use user flag", graph.AnswerVerified)
	seedClass(t, store, "chown-dockerfile", "chown in dockerfile", "docker", "dockerfile", "1.0", "add USER", graph.AnswerPending)
	rr := do(t, s, "GET", "/api/v1/problems?q=docker", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp listProblemsResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Total < 1 {
		t.Errorf("total = %d, want >= 1 (FTS5 match)", resp.Total)
	}
	// OB-GAP-050: search results must carry the same per-class metadata
	// as the plain list/detail endpoints — real derived status (not the
	// status filter echoed back), description, created_at, answer_count.
	found := false
	for _, p := range resp.Problems {
		if p.Status == "" {
			t.Errorf("problem %q: status empty, want derived status", p.Title)
		}
		if p.CreatedAt == "" {
			t.Errorf("problem %q: created_at empty, want RFC3339 timestamp", p.Title)
		}
		if p.Title == "docker-permissions" {
			found = true
			if p.Status != "verified" {
				t.Errorf("docker-permissions status = %q, want verified", p.Status)
			}
			if p.Description != "fixing permissions in containers" {
				t.Errorf("docker-permissions description = %q, want seeded description", p.Description)
			}
			if p.AnswerCount != 1 {
				t.Errorf("docker-permissions answer_count = %d, want 1", p.AnswerCount)
			}
		}
	}
	if !found {
		t.Errorf("docker-permissions not in search results: %+v", resp.Problems)
	}
}

// A search with no matches must return "problems":[] (not null) so
// agent clients can iterate the field without a nil check (OB-GAP-046).
func TestListProblems_SearchNoMatch(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "docker-permissions", "fixing permissions in containers", "docker", "go", "1.0", "use user flag", graph.AnswerVerified)
	rr := do(t, s, "GET", "/api/v1/problems?q=zzzznomatchxyz", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp listProblemsResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Total != 0 {
		t.Errorf("total = %d, want 0", resp.Total)
	}
	if len(resp.Problems) != 0 {
		t.Errorf("problems len = %d, want 0", len(resp.Problems))
	}
	if resp.Problems == nil {
		t.Errorf("problems is nil; want non-nil empty slice (serializes as null)")
	}
	if !strings.Contains(rr.Body.String(), `"problems":[]`) {
		t.Errorf("body = %s, want it to contain `\"problems\":[]`", rr.Body.String())
	}
}

// OB-GAP-080: env= and lang= must filter the NON-search branch of
// handleListProblems exactly like the q branch — before the fix the non-q
// branch ignored both params, so ?env=<nonsense> returned the whole
// unfiltered catalog (the shipped SPA hits this shape when a filter chip
// is clicked with an empty query box).
func TestListProblems_EnvLangFiltersWithoutQuery(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "docker-perms", "desc1", "docker", "go", "1.0", "sol1", graph.AnswerVerified)
	seedClass(t, store, "kube-pods", "desc2", "kubernetes", "python", "1.0", "sol2", graph.AnswerPending)

	get := func(path string) listProblemsResponse {
		t.Helper()
		rr := do(t, s, "GET", path, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200", path, rr.Code)
		}
		var resp listProblemsResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("GET %s: decode: %v", path, err)
		}
		return resp
	}

	// Nonsense env with no q → total 0 (the OB-GAP-080 live repro).
	if resp := get("/api/v1/problems?env=zzz-no-such-env-xyz"); resp.Total != 0 {
		t.Errorf("env=<nonsense> no q: total = %d, want 0 (env must filter the non-search branch)", resp.Total)
	}
	// Nonsense lang with no q → total 0.
	if resp := get("/api/v1/problems?lang=zzz-no-such-lang-xyz"); resp.Total != 0 {
		t.Errorf("lang=<nonsense> no q: total = %d, want 0 (lang must filter the non-search branch)", resp.Total)
	}
	// Real env → only matching classes.
	resp := get("/api/v1/problems?env=docker")
	if resp.Total != 1 || len(resp.Problems) != 1 || resp.Problems[0].Title != "docker-perms" {
		t.Errorf("env=docker: total = %d problems = %+v, want exactly docker-perms", resp.Total, resp.Problems)
	}
	// Real lang → only matching classes.
	resp = get("/api/v1/problems?lang=python")
	if resp.Total != 1 || len(resp.Problems) != 1 || resp.Problems[0].Title != "kube-pods" {
		t.Errorf("lang=python: total = %d problems = %+v, want exactly kube-pods", resp.Total, resp.Problems)
	}
	// Combined env+lang must intersect (no class has both docker+python).
	if resp := get("/api/v1/problems?env=docker&lang=python"); resp.Total != 0 {
		t.Errorf("env=docker&lang=python: total = %d, want 0 (filters intersect)", resp.Total)
	}
	// Empty env/lang keeps the unfiltered total.
	if resp := get("/api/v1/problems"); resp.Total != 2 {
		t.Errorf("no filters: total = %d, want 2 (unfiltered catalog unchanged)", resp.Total)
	}
}

// The list endpoint's limit parameter clamps over-max values to the
// documented max of 100 instead of silently falling back to the default
// 20 (OB-GAP-081): a client asking for a large page would otherwise get
// a short page with no error. 0 / negative and non-numeric values keep
// the documented default of 20, and the response shape is unchanged.
func TestListProblems_LimitBoundaries(t *testing.T) {
	s, store, _ := newTestServer(t)
	const total = 101
	for i := 0; i < total; i++ {
		seedClass(t, store, fmt.Sprintf("limit-class-%03d", i), "desc", "docker", "go", "1.0", "sol", graph.AnswerVerified)
	}

	get := func(path string) listProblemsResponse {
		t.Helper()
		rr := do(t, s, "GET", path, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200", path, rr.Code)
		}
		var resp listProblemsResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("GET %s: decode: %v", path, err)
		}
		return resp
	}

	cases := []struct {
		name  string
		path  string
		wantN int
	}{
		{"max accepted", "/api/v1/problems?limit=100", 100},
		{"over max clamps", "/api/v1/problems?limit=101", 100},
		{"far over max clamps", "/api/v1/problems?limit=200", 100},
		{"zero uses default", "/api/v1/problems?limit=0", 20},
		{"negative uses default", "/api/v1/problems?limit=-3", 20},
		{"non-numeric uses default", "/api/v1/problems?limit=abc", 20},
		{"omitted uses default", "/api/v1/problems", 20},
	}
	for _, tc := range cases {
		resp := get(tc.path)
		if len(resp.Problems) != tc.wantN {
			t.Errorf("%s (%s): got %d problems, want %d", tc.name, tc.path, len(resp.Problems), tc.wantN)
		}
		if resp.Total != total {
			t.Errorf("%s (%s): total = %d, want %d (limit must not affect total)", tc.name, tc.path, resp.Total, total)
		}
	}

	// offset still paginates past the clamp boundary.
	first := get("/api/v1/problems?limit=100&offset=0")
	rest := get("/api/v1/problems?limit=200&offset=100")
	if len(first.Problems) != 100 || len(rest.Problems) != 1 {
		t.Errorf("offset pagination: first page = %d items, second = %d, want 100 + 1", len(first.Problems), len(rest.Problems))
	}
}

func TestGetProblemClass(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "docker-perms", "permissions", "docker", "go", "1.0", "use --user", graph.AnswerVerified)
	rr := do(t, s, "GET", "/api/v1/problems/docker-perms", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var pc problemClassWire
	_ = json.Unmarshal(rr.Body.Bytes(), &pc)
	if pc.Title != "docker-perms" {
		t.Errorf("title = %q", pc.Title)
	}
	if pc.AnswerCount != 1 {
		t.Errorf("answer_count = %d, want 1", pc.AnswerCount)
	}
}

func TestGetProblemClass_NotFound(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "GET", "/api/v1/problems/nope", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// The detail endpoint must report the same derived status as the list
// endpoint for the same class: ci_passed > verified > pending > failed,
// 'pending' when the class has no answers (OB-GAP-024).
func TestGetProblemClass_StatusMatchesList(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "docker-perms", "permissions", "docker", "go", "1.0", "use --user", graph.AnswerVerified)
	if _, _, err := store.UpsertProblemClass(context.Background(), "no-answers-class", "desc"); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	listStatus := func(title string) string {
		t.Helper()
		rr := do(t, s, "GET", "/api/v1/problems?limit=100", nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("list: status = %d, want 200", rr.Code)
		}
		var resp listProblemsResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode list: %v", err)
		}
		for _, p := range resp.Problems {
			if p.Title == title {
				return p.Status
			}
		}
		t.Fatalf("class %q not in list response", title)
		return ""
	}

	for _, tc := range []struct {
		class string
		want  string
	}{
		{"docker-perms", "verified"},
		{"no-answers-class", "pending"},
	} {
		rr := do(t, s, "GET", "/api/v1/problems/"+tc.class, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("GET %s: status = %d, want 200", tc.class, rr.Code)
		}
		var pc problemClassWire
		if err := json.Unmarshal(rr.Body.Bytes(), &pc); err != nil {
			t.Fatalf("GET %s: decode: %v", tc.class, err)
		}
		if pc.Status == "" {
			t.Errorf("GET %s: status is empty, want %q", tc.class, tc.want)
		}
		if pc.Status != tc.want {
			t.Errorf("GET %s: status = %q, want %q", tc.class, pc.Status, tc.want)
		}
		if ls := listStatus(tc.class); ls != pc.Status {
			t.Errorf("%s: detail status %q != list status %q", tc.class, pc.Status, ls)
		}
	}
}

// --- Answers -------------------------------------------------------------

func TestListAnswers(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "docker-perms", "perms", "docker", "go", "1.0", "use --user", graph.AnswerVerified)
	rr := do(t, s, "GET", "/api/v1/problems/docker-perms/answers", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp listAnswersResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Total != 1 {
		t.Errorf("total = %d, want 1", resp.Total)
	}
	if len(resp.Answers) != 1 {
		t.Fatalf("answers len = %d, want 1", len(resp.Answers))
	}
	if resp.Answers[0].Env != "docker" {
		t.Errorf("env = %q, want docker", resp.Answers[0].Env)
	}
}

func TestGetAnswer(t *testing.T) {
	s, store, _ := newTestServer(t)
	_, aid := seedClass(t, store, "docker-perms", "perms", "docker", "go", "1.0", "use --user", graph.AnswerVerified)
	path := "/api/v1/problems/docker-perms/answers/" + intToString(aid)
	rr := do(t, s, "GET", path, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var a answerWire
	_ = json.Unmarshal(rr.Body.Bytes(), &a)
	if a.Solution != "use --user" {
		t.Errorf("solution = %q", a.Solution)
	}
}

func TestGetAnswer_CrossClassReturns404(t *testing.T) {
	s, store, _ := newTestServer(t)
	_, aid := seedClass(t, store, "class-a", "perms", "docker", "go", "1.0", "solA", graph.AnswerVerified)
	// Look up aid under the wrong class.
	rr := do(t, s, "GET", "/api/v1/problems/class-b/answers/"+intToString(aid), nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (cross-class)", rr.Code)
	}
}

func TestGetAnswer_InvalidID(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "GET", "/api/v1/problems/class-a/answers/abc", nil)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

// --- Related -------------------------------------------------------------

func TestGetRelated_Empty(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "docker-perms", "perms", "docker", "go", "1.0", "sol", graph.AnswerVerified)
	rr := do(t, s, "GET", "/api/v1/problems/docker-perms/related", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp relatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Related) != 0 {
		t.Errorf("related len = %d, want 0", len(resp.Related))
	}
	if resp.Related == nil {
		t.Errorf("related is nil; want non-nil empty slice (serializes as null)")
	}
	if !strings.Contains(rr.Body.String(), `"related":[]`) {
		t.Errorf("body = %s, want it to contain `\"related\":[]`", rr.Body.String())
	}
}

func TestGetRelated_WithEdge(t *testing.T) {
	s, store, _ := newTestServer(t)
	classA, _ := seedClass(t, store, "class-a", "a", "docker", "go", "1.0", "solA", graph.AnswerVerified)
	classB, _ := seedClass(t, store, "class-b", "b", "docker", "go", "1.0", "solB", graph.AnswerVerified)
	if _, err := store.CreateEdge(context.Background(), classA, classB, graph.EdgeSameRootCause, 0.8); err != nil {
		t.Fatalf("create edge: %v", err)
	}
	rr := do(t, s, "GET", "/api/v1/problems/class-a/related", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp relatedResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if len(resp.Related) != 1 {
		t.Fatalf("related len = %d, want 1", len(resp.Related))
	}
	if resp.Related[0].ProblemClass != "class-b" {
		t.Errorf("related[0].problem_class = %q, want class-b", resp.Related[0].ProblemClass)
	}
	if resp.Related[0].Relationship != graph.EdgeSameRootCause {
		t.Errorf("relationship = %q", resp.Related[0].Relationship)
	}
}

// --- Queue endpoints -----------------------------------------------------

func TestListQueue(t *testing.T) {
	s, _, _ := newTestServer(t)
	// Submit two problems.
	for _, cls := range []string{"class-a", "class-b"} {
		body := submitProblemRequest{ProblemClass: cls, Cadence: ingest.CadencePrePhase}
		rr := do(t, s, "POST", "/api/v1/problems/submit", body)
		if rr.Code != http.StatusOK {
			t.Fatalf("submit %s: %d", cls, rr.Code)
		}
	}
	rr := do(t, s, "GET", "/api/v1/queue", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp queueListResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2", resp.Total)
	}
}

// TestListQueue_ExcludesPlaceholderClasses verifies placeholder-class
// entries never appear in GET /api/v1/queue, non-placeholder entries are
// unaffected, and Total/positions reflect the filtered set (OB-GAP-061).
func TestListQueue_ExcludesPlaceholderClasses(t *testing.T) {
	s, _, _ := newTestServer(t)
	for _, cls := range []string{"class-a", "off-by-one-self-test", "class-b"} {
		body := submitProblemRequest{ProblemClass: cls, Cadence: ingest.CadencePrePhase}
		rr := do(t, s, "POST", "/api/v1/problems/submit", body)
		if rr.Code != http.StatusOK {
			t.Fatalf("submit %s: %d, body = %s", cls, rr.Code, rr.Body.String())
		}
	}
	rr := do(t, s, "GET", "/api/v1/queue", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var resp queueListResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	if resp.Total != 2 {
		t.Errorf("total = %d, want 2 (placeholder filtered)", resp.Total)
	}
	for _, e := range resp.Entries {
		if graph.IsPlaceholderClass(e.ProblemClass) {
			t.Errorf("placeholder class %q leaked into queue listing", e.ProblemClass)
		}
	}
	// Positions contiguous from 1 for the filtered set.
	for i, e := range resp.Entries {
		if e.Position != i+1 {
			t.Errorf("entries[%d].position = %d, want %d", i, e.Position, i+1)
		}
	}
}

func TestGetQueueStatus(t *testing.T) {
	s, _, _ := newTestServer(t)
	body := submitProblemRequest{ProblemClass: "class-a", Cadence: ingest.CadencePrePhase}
	subResp := do(t, s, "POST", "/api/v1/problems/submit", body)
	var sub submitProblemResponse
	_ = json.Unmarshal(subResp.Body.Bytes(), &sub)

	rr := do(t, s, "GET", "/api/v1/queue/"+sub.SubmissionID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var entry queueEntryWire
	_ = json.Unmarshal(rr.Body.Bytes(), &entry)
	if entry.SubmissionID != sub.SubmissionID {
		t.Errorf("id = %q, want %q", entry.SubmissionID, sub.SubmissionID)
	}
	if entry.Status != ingest.StatusPending {
		t.Errorf("status = %q, want pending", entry.Status)
	}
}

// queueEntryObject decodes a queue response body — either the list shape
// ({"entries":[...],"total":n}) or the single-entry shape — and returns
// the raw JSON object for the first entry. Tests assert on this rather
// than on the typed struct so a tag typo or a dropped key cannot hide
// behind the decoder.
func queueEntryObject(t *testing.T, label, body string) map[string]any {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		t.Fatalf("%s: decode body: %v (body = %s)", label, err, body)
	}
	entries, listed := raw["entries"]
	if !listed {
		return raw
	}
	list, ok := entries.([]any)
	if !ok || len(list) == 0 {
		t.Fatalf("%s: list body carries no entries: %s", label, body)
	}
	obj, ok := list[0].(map[string]any)
	if !ok {
		t.Fatalf("%s: first entry is not a JSON object: %s", label, body)
	}
	return obj
}

// TestFormatStoreTimestamp pins the store→wire timestamp conversion: the
// queue columns hold SQLite TEXT ("2006-01-02 15:04:05", UTC, no offset)
// while every documented API timestamp is RFC 3339 (OB-GAP-068). Empty
// stays empty, an already-RFC3339 value passes through, and an
// unrecognised value is returned unchanged rather than dropped.
func TestFormatStoreTimestamp(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty stays empty", "", ""},
		{"sqlite layout becomes RFC3339", "2026-08-15 02:13:43", "2026-08-15T02:13:43Z"},
		{"sqlite layout at midnight", "2026-01-02 00:00:00", "2026-01-02T00:00:00Z"},
		{"already RFC3339 passes through", "2026-09-04T15:35:26Z", "2026-09-04T15:35:26Z"},
		{"RFC3339 with offset passes through", "2026-09-04T15:35:26-05:00", "2026-09-04T15:35:26-05:00"},
		{"RFC3339Nano passes through", "2026-09-04T15:35:26.123456789Z", "2026-09-04T15:35:26.123456789Z"},
		{"date-only layout is returned unchanged", "2026-08-15", "2026-08-15"},
		{"free text is returned unchanged", "not-a-timestamp", "not-a-timestamp"},
	}
	for _, tc := range cases {
		if got := formatStoreTimestamp(tc.in); got != tc.want {
			t.Errorf("%s: formatStoreTimestamp(%q) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}

	// Idempotent: re-running the helper on its own output is a no-op, so
	// it stays safe if a caller converts an already-converted value.
	converted := formatStoreTimestamp("2026-08-15 02:13:43")
	if got := formatStoreTimestamp(converted); got != converted {
		t.Errorf("formatStoreTimestamp is not idempotent: %q -> %q", converted, got)
	}
}

// TestQueueTimestamps_RFC3339 is the OB-GAP-068 regression test: the
// queue endpoints must return started_at/completed_at as RFC 3339, never
// the raw SQLite TEXT the columns hold, and a pending entry must carry
// both keys as "" exactly as docs/api-reference.md documents.
//
// The fixture inserts the raw store layout directly, the way
// seedObservedSolveTime and the store's own CURRENT_TIMESTAMP writes do.
// Both endpoints go through entryToWire, so both are exercised. Every
// assertion reads the RAW response body — a tag typo or a missing key
// must not be able to pass behind the decoder.
func TestQueueTimestamps_RFC3339(t *testing.T) {
	s, store, _ := newTestServer(t)
	const completeID = "sub_ts_complete"
	if _, err := store.DB().Exec(`INSERT INTO queue_entries
		(id, problem_class, status, stage, started_at, completed_at)
		VALUES (?, 'cls-ts', 'complete', 'done',
			'2026-08-15 02:13:43', '2026-08-15 02:14:16')`, completeID); err != nil {
		t.Fatalf("insert completed entry: %v", err)
	}
	const pendingID = "sub_ts_pending"
	if _, err := store.DB().Exec(`INSERT INTO queue_entries
		(id, problem_class, status, stage) VALUES (?, 'cls-ts', 'pending', 'queued')`,
		pendingID); err != nil {
		t.Fatalf("insert pending entry: %v", err)
	}

	// A completed entry: both keys present, RFC 3339, both parseable.
	for _, tc := range []struct{ name, path string }{
		{"list", "/api/v1/queue?status=complete&limit=1"},
		{"detail", "/api/v1/queue/" + completeID},
	} {
		rr := do(t, s, "GET", tc.path, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", tc.name, rr.Code, rr.Body.String())
		}
		body := rr.Body.String()
		// The exact wire form, asserted on the raw bytes.
		for _, want := range []string{
			`"started_at":"2026-08-15T02:13:43Z"`,
			`"completed_at":"2026-08-15T02:14:16Z"`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: raw body missing %s: %s", tc.name, want, body)
			}
		}
		obj := queueEntryObject(t, tc.name, body)
		if obj["submission_id"] != completeID {
			t.Errorf("%s: submission_id = %v, want %q", tc.name, obj["submission_id"], completeID)
		}
		for _, field := range []string{"started_at", "completed_at"} {
			got, present := obj[field]
			if !present {
				t.Errorf("%s: %q missing from the response", tc.name, field)
				continue
			}
			str, ok := got.(string)
			if !ok {
				t.Errorf("%s: %q = %v (%T), want a JSON string", tc.name, field, got, got)
				continue
			}
			if str == "" {
				t.Errorf("%s: %q is empty for a completed entry", tc.name, field)
			}
			if _, err := time.Parse(time.RFC3339, str); err != nil {
				t.Errorf("%s: %q = %q does not parse as RFC3339: %v", tc.name, field, str, err)
			}
			if strings.Contains(str, " ") {
				t.Errorf("%s: %q = %q still carries the raw store layout (space separator)", tc.name, field, str)
			}
		}
	}

	// A pending entry has no timing yet: both keys present and empty,
	// matching the documented pending-entry example.
	for _, tc := range []struct{ name, path string }{
		{"pending detail", "/api/v1/queue/" + pendingID},
		{"pending list", "/api/v1/queue?status=pending&limit=1"},
	} {
		rr := do(t, s, "GET", tc.path, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", tc.name, rr.Code, rr.Body.String())
		}
		body := rr.Body.String()
		for _, want := range []string{`"started_at":""`, `"completed_at":""`} {
			if !strings.Contains(body, want) {
				t.Errorf("%s: raw body missing %s (omitempty defeats the documented empty-string shape): %s",
					tc.name, want, body)
			}
		}
		obj := queueEntryObject(t, tc.name, body)
		if obj["submission_id"] != pendingID {
			t.Errorf("%s: submission_id = %v, want %q", tc.name, obj["submission_id"], pendingID)
		}
		for _, field := range []string{"started_at", "completed_at"} {
			got, present := obj[field]
			if !present {
				t.Errorf("%s: %q missing from a pending entry, want \"\"", tc.name, field)
				continue
			}
			if got != "" {
				t.Errorf("%s: %q = %v, want \"\"", tc.name, field, got)
			}
		}
	}
}

// TestGetQueueStatus_FailureReason guards DF-OFF-BY-ONE-4 at the API
// layer: a failed solve must tell the submitter WHY over
// GET /api/v1/queue/{submission_id}, not just status='failed'. Pending
// entries must NOT carry the field at all.
func TestGetQueueStatus_FailureReason(t *testing.T) {
	s, _, queue := newTestServer(t)
	ctx := context.Background()
	body := submitProblemRequest{ProblemClass: "class-fail", Cadence: ingest.CadencePrePhase}
	subResp := do(t, s, "POST", "/api/v1/problems/submit", body)
	var sub submitProblemResponse
	_ = json.Unmarshal(subResp.Body.Bytes(), &sub)
	if sub.SubmissionID == "" {
		t.Fatalf("submit did not return an id: %s", subResp.Body.String())
	}

	// Pending: no failure_reason key on the wire.
	rr := do(t, s, "GET", "/api/v1/queue/"+sub.SubmissionID, nil)
	var pending map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &pending)
	if _, ok := pending["failure_reason"]; ok {
		t.Errorf("pending entry carries failure_reason: %v", pending)
	}

	const reason = "solver: pi agent exited 1: sandbox timeout after 300s"
	if _, err := queue.Dequeue(ctx); err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if err := queue.MarkFailed(ctx, sub.SubmissionID, reason); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}

	rr = do(t, s, "GET", "/api/v1/queue/"+sub.SubmissionID, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var entry queueEntryWire
	if err := json.Unmarshal(rr.Body.Bytes(), &entry); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if entry.Status != ingest.StatusFailed {
		t.Fatalf("status = %q, want failed", entry.Status)
	}
	if entry.FailureReason == "" {
		t.Fatal("failure_reason empty on a failed entry")
	}
	if entry.FailureReason != reason {
		t.Errorf("failure_reason = %q, want %q", entry.FailureReason, reason)
	}
	// Raw-JSON check: the field must be on the wire, not only in the
	// decoded struct (a tag typo would pass the struct comparison).
	var raw map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &raw)
	if raw["failure_reason"] != reason {
		t.Errorf("raw failure_reason = %v, want %q", raw["failure_reason"], reason)
	}

	// The listing endpoint shares entryToWire — it must expose the
	// reason too, so an agent polling the list is not left blind.
	lr := do(t, s, "GET", "/api/v1/queue?status=failed", nil)
	var list queueListResponse
	if err := json.Unmarshal(lr.Body.Bytes(), &list); err != nil {
		t.Fatalf("unmarshal list: %v", err)
	}
	if len(list.Entries) != 1 {
		t.Fatalf("failed entries = %d, want 1", len(list.Entries))
	}
	if list.Entries[0].FailureReason != reason {
		t.Errorf("list failure_reason = %q, want %q", list.Entries[0].FailureReason, reason)
	}
}

func TestGetQueueStatus_NotFound(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "GET", "/api/v1/queue/sub_nonexistent", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

// seedObservedSolveTime inserts one completed queue entry whose solve took
// 133s (10:00:00 -> 10:02:13), so Queue.AvgSolveTime reports 2m13s — the
// same fixture TestStats_AvgSolveTime and
// TestSubmit_EstimatedTimeFromObservedAverage use.
func seedObservedSolveTime(t *testing.T, store *graph.Store, id string) {
	t.Helper()
	if _, err := store.DB().Exec(`INSERT INTO queue_entries
		(id, problem_class, status, stage, started_at, completed_at)
		VALUES (?, 'cls-observed', 'complete', 'done',
			'2026-08-21 10:00:00', '2026-08-21 10:02:13')`, id); err != nil {
		t.Fatalf("insert completed entry: %v", err)
	}
}

// submitForTest posts a pre-phase submission and returns the decoded body.
func submitForTest(t *testing.T, s *Server, class string) submitProblemResponse {
	t.Helper()
	rr := do(t, s, "POST", "/api/v1/problems/submit", submitProblemRequest{
		ProblemClass: class,
		Environment:  "docker",
		Language:     "go",
		Cadence:      ingest.CadencePrePhase,
	})
	if rr.Code != http.StatusOK {
		t.Fatalf("submit %s: status = %d, body = %s", class, rr.Code, rr.Body.String())
	}
	var resp submitProblemResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode submit response: %v", err)
	}
	if resp.SubmissionID == "" {
		t.Fatalf("submit %s returned no submission_id: %s", class, rr.Body.String())
	}
	return resp
}

// getQueueEntry GETs one queue entry and returns both the raw JSON object
// (so a missing or mis-tagged key is caught) and the decoded struct.
func getQueueEntry(t *testing.T, s *Server, id string) (map[string]any, queueEntryWire) {
	t.Helper()
	rr := do(t, s, "GET", "/api/v1/queue/"+id, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET queue/%s: status = %d, body = %s", id, rr.Code, rr.Body.String())
	}
	var raw map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw queue/%s: %v", id, err)
	}
	var entry queueEntryWire
	if err := json.Unmarshal(rr.Body.Bytes(), &entry); err != nil {
		t.Fatalf("decode queue/%s: %v", id, err)
	}
	return raw, entry
}

// TestGetQueueStatus_PositionAndEstimatedTime guards DF-OFF-BY-ONE-11 at the
// API layer: GET /api/v1/queue/{submission_id} must fill the two fields the
// spec declares on QueueEntry instead of returning position 0 and an empty
// estimated_time.
//
// Fixture: one completed solve of 2m13s is the observed per-job cost, then
// three pending rows in a deterministic order (the extra rows get created_at
// values in the future so they cannot tie with the submitted row's now()).
// A pending entry reports its 1-based place in the pending queue and
// estimateTime(place, observed mean); a terminal entry reports 0 and "";
// an in_progress entry reports 0 and one job's wait.
func TestGetQueueStatus_PositionAndEstimatedTime(t *testing.T) {
	s, store, queue := newTestServer(t)
	ctx := context.Background()
	seedObservedSolveTime(t, store, "sub_eta_done")

	// Real submit path first: its position/ETA must agree with the detail
	// endpoint (pre-fix the submit said position 1 / 2m13s while the detail
	// endpoint said 0 / "").
	submitted := submitForTest(t, s, "cls-eta-submitted")
	if submitted.Position != 1 {
		t.Fatalf("submit position = %d, want 1 (only pending row)", submitted.Position)
	}
	if submitted.EstimatedTime == "" {
		t.Fatalf("submit estimated_time empty: %+v", submitted)
	}

	// Two more pending rows, queued behind the submitted one.
	for i, id := range []string{"sub_eta_p2", "sub_eta_p3"} {
		if _, err := store.DB().Exec(`INSERT INTO queue_entries
			(id, problem_class, status, stage, created_at)
			VALUES (?, 'cls-eta-queued', 'pending', 'queued', ?)`,
			id, []string{"2030-01-01 00:00:01", "2030-01-01 00:00:02"}[i]); err != nil {
			t.Fatalf("insert pending %s: %v", id, err)
		}
	}

	perJob, err := queue.AvgSolveTime(ctx)
	if err != nil {
		t.Fatalf("AvgSolveTime: %v", err)
	}
	if perJob <= 0 {
		t.Fatalf("fixture observed average = %s, want > 0", perJob)
	}

	// Pending: 1-based pending position + ETA from the observed mean.
	for _, tc := range []struct {
		id  string
		pos int
	}{
		{submitted.SubmissionID, 1},
		{"sub_eta_p2", 2},
		{"sub_eta_p3", 3},
	} {
		raw, entry := getQueueEntry(t, s, tc.id)
		wantETA := estimateTime(tc.pos, perJob)
		if entry.Status != ingest.StatusPending {
			t.Fatalf("%s status = %q, want pending", tc.id, entry.Status)
		}
		if entry.Position != tc.pos {
			t.Errorf("%s position = %d, want %d", tc.id, entry.Position, tc.pos)
		}
		if entry.EstimatedTime == "" {
			t.Errorf("%s estimated_time is empty for a waiting entry", tc.id)
		}
		if entry.EstimatedTime != wantETA {
			t.Errorf("%s estimated_time = %q, want %q", tc.id, entry.EstimatedTime, wantETA)
		}
		// Raw-JSON key checks: a struct-only assertion misses a json tag
		// typo or a field the handler never writes.
		rawPos, ok := raw["position"]
		if !ok {
			t.Fatalf("%s: no position key on the wire: %v", tc.id, raw)
		}
		if got, isNum := rawPos.(float64); !isNum || int(got) != tc.pos {
			t.Errorf("%s raw position = %v, want %d", tc.id, rawPos, tc.pos)
		}
		rawETA, ok := raw["estimated_time"]
		if !ok {
			t.Fatalf("%s: no estimated_time key on the wire: %v", tc.id, raw)
		}
		if rawETA != wantETA {
			t.Errorf("%s raw estimated_time = %v, want %q", tc.id, rawETA, wantETA)
		}
	}

	// Terminal (complete): not waiting, so no position and no ETA.
	raw, entry := getQueueEntry(t, s, "sub_eta_done")
	if entry.Status != ingest.StatusComplete {
		t.Fatalf("sub_eta_done status = %q, want complete", entry.Status)
	}
	if entry.Position != 0 {
		t.Errorf("complete entry position = %d, want 0", entry.Position)
	}
	if entry.EstimatedTime != "" {
		t.Errorf("complete entry estimated_time = %q, want empty", entry.EstimatedTime)
	}
	if raw["position"] != float64(0) {
		t.Errorf("complete entry raw position = %v, want 0", raw["position"])
	}
	if got, ok := raw["estimated_time"]; !ok || got != "" {
		t.Errorf("complete entry raw estimated_time = %v (present=%v), want \"\"", got, ok)
	}

	// Terminal (failed): same rule as complete.
	if err := queue.MarkFailed(ctx, "sub_eta_p3", "solver: pi agent exited 1"); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	for _, id := range []string{"sub_eta_p3"} {
		raw, entry := getQueueEntry(t, s, id)
		if entry.Status != ingest.StatusFailed {
			t.Fatalf("%s status = %q, want failed", id, entry.Status)
		}
		if entry.Position != 0 {
			t.Errorf("%s (failed) position = %d, want 0", id, entry.Position)
		}
		if entry.EstimatedTime != "" {
			t.Errorf("%s (failed) estimated_time = %q, want empty", id, entry.EstimatedTime)
		}
		if got, ok := raw["estimated_time"]; !ok || got != "" {
			t.Errorf("%s (failed) raw estimated_time = %v (present=%v), want \"\"", id, got, ok)
		}
	}

	// in_progress: off the queue but holding the bench — one job's wait.
	solving, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if solving == nil {
		t.Fatal("Dequeue returned no entry")
	}
	_, entry = getQueueEntry(t, s, solving.ID)
	if entry.Status != ingest.StatusInProgress {
		t.Fatalf("%s status = %q, want in_progress", solving.ID, entry.Status)
	}
	if entry.Position != 0 {
		t.Errorf("in_progress position = %d, want 0 (not waiting in the queue)", entry.Position)
	}
	if want := estimateTime(1, perJob); entry.EstimatedTime != want {
		t.Errorf("in_progress estimated_time = %q, want %q (one job)", entry.EstimatedTime, want)
	}
}

// TestListQueue_PositionAndEstimatedTime guards DF-OFF-BY-ONE-11 on the list
// endpoint: position stays the entry's 1-based place in the returned page
// (offset+i+1, the documented behaviour) while estimated_time is filled per
// entry — the jobs ahead for a pending entry, one job for an in_progress
// entry, and empty for a terminal one.
func TestListQueue_PositionAndEstimatedTime(t *testing.T) {
	s, store, queue := newTestServer(t)
	ctx := context.Background()
	seedObservedSolveTime(t, store, "sub_eta_done")

	submitted := submitForTest(t, s, "cls-list-submitted")
	for i, id := range []string{"sub_list_p2", "sub_list_p3"} {
		if _, err := store.DB().Exec(`INSERT INTO queue_entries
			(id, problem_class, status, stage, created_at)
			VALUES (?, 'cls-list-queued', 'pending', 'queued', ?)`,
			id, []string{"2030-01-01 00:00:01", "2030-01-01 00:00:02"}[i]); err != nil {
			t.Fatalf("insert pending %s: %v", id, err)
		}
	}
	// One pending row starts solving.
	solving, err := queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("Dequeue: %v", err)
	}
	if solving == nil {
		t.Fatal("Dequeue returned no entry")
	}
	// Deterministic: the submitted row's created_at is "now" while the two
	// extra pending rows are pinned to 2030, so FIFO picks the submission.
	if solving.ID != submitted.SubmissionID {
		t.Fatalf("Dequeue returned %s, want the submitted row %s (pending order is created_at ASC)",
			solving.ID, submitted.SubmissionID)
	}

	perJob, err := queue.AvgSolveTime(ctx)
	if err != nil {
		t.Fatalf("AvgSolveTime: %v", err)
	}
	if perJob <= 0 {
		t.Fatalf("fixture observed average = %s, want > 0", perJob)
	}

	// Expected ETA per entry, keyed by ID: pending entries scale with their
	// place in the pending queue, the solving entry is one job.
	pendingRows, err := queue.List(ctx, ingest.StatusPending, 1000, 0)
	if err != nil {
		t.Fatalf("pending list: %v", err)
	}
	wantETA := map[string]string{}
	for i, e := range pendingRows {
		wantETA[e.ID] = estimateTime(i+1, perJob)
	}
	wantETA[solving.ID] = estimateTime(1, perJob)

	rr := do(t, s, "GET", "/api/v1/queue?limit=5", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", rr.Code, rr.Body.String())
	}
	var resp queueListResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	var raw struct {
		Entries []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &raw); err != nil {
		t.Fatalf("decode raw list: %v", err)
	}
	if len(resp.Entries) != 4 {
		t.Fatalf("entries = %d, want 4 (1 complete + 3 pending/solving): %s", len(resp.Entries), rr.Body.String())
	}
	if len(raw.Entries) != len(resp.Entries) {
		t.Fatalf("raw entries = %d, decoded entries = %d", len(raw.Entries), len(resp.Entries))
	}

	seen := map[string]string{}
	for i, e := range resp.Entries {
		seen[e.SubmissionID] = e.Status
		// List position is unchanged: offset (0) + index + 1.
		if e.Position != i+1 {
			t.Errorf("entries[%d] (%s) position = %d, want %d (offset+i+1)", i, e.SubmissionID, e.Position, i+1)
		}
		rawETA, ok := raw.Entries[i]["estimated_time"]
		if !ok {
			t.Fatalf("entries[%d] (%s) has no estimated_time key: %v", i, e.SubmissionID, raw.Entries[i])
		}
		switch e.Status {
		case ingest.StatusPending:
			if e.EstimatedTime == "" {
				t.Errorf("pending %s estimated_time is empty", e.SubmissionID)
			}
			if e.EstimatedTime != wantETA[e.SubmissionID] {
				t.Errorf("pending %s estimated_time = %q, want %q",
					e.SubmissionID, e.EstimatedTime, wantETA[e.SubmissionID])
			}
		case ingest.StatusInProgress:
			if e.EstimatedTime == "" {
				t.Errorf("in_progress %s estimated_time is empty", e.SubmissionID)
			}
			if e.EstimatedTime != wantETA[e.SubmissionID] {
				t.Errorf("in_progress %s estimated_time = %q, want %q",
					e.SubmissionID, e.EstimatedTime, wantETA[e.SubmissionID])
			}
		default:
			if e.EstimatedTime != "" {
				t.Errorf("terminal %s (%s) estimated_time = %q, want empty",
					e.SubmissionID, e.Status, e.EstimatedTime)
			}
		}
		if rawETA != e.EstimatedTime {
			t.Errorf("entries[%d] (%s) raw estimated_time = %v, decoded = %q",
				i, e.SubmissionID, rawETA, e.EstimatedTime)
		}
	}
	for _, id := range []string{submitted.SubmissionID, "sub_eta_done", solving.ID} {
		if _, ok := seen[id]; !ok {
			t.Errorf("entry %s missing from the listing: %v", id, seen)
		}
	}
	if seen[solving.ID] != ingest.StatusInProgress {
		t.Errorf("%s status = %q, want in_progress", solving.ID, seen[solving.ID])
	}

	// Paging: the list position stays the page position, offset included.
	offRR := do(t, s, "GET", "/api/v1/queue?limit=2&offset=1", nil)
	if offRR.Code != http.StatusOK {
		t.Fatalf("paged status = %d, want 200", offRR.Code)
	}
	var offResp queueListResponse
	if err := json.Unmarshal(offRR.Body.Bytes(), &offResp); err != nil {
		t.Fatalf("decode paged list: %v", err)
	}
	if len(offResp.Entries) != 2 {
		t.Fatalf("paged entries = %d, want 2", len(offResp.Entries))
	}
	for i, e := range offResp.Entries {
		if e.Position != 1+i+1 {
			t.Errorf("paged entries[%d] (%s) position = %d, want %d",
				i, e.SubmissionID, e.Position, 1+i+1)
		}
	}
}

// --- Taxonomy + Stats ----------------------------------------------------

func TestTaxonomy_Empty(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "GET", "/api/v1/taxonomy", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	tree, ok := body["tree"].([]any)
	if !ok {
		t.Fatalf("tree not array: %v", body["tree"])
	}
	if len(tree) != 0 {
		t.Errorf("tree len = %d, want 0", len(tree))
	}
}

func TestTaxonomy_WithData(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "docker-perms", "perms", "docker", "go", "1.0", "use --user", graph.AnswerVerified)
	rr := do(t, s, "GET", "/api/v1/taxonomy", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		Tree []taxonomyNode `json:"tree"`
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if len(body.Tree) != 1 {
		t.Errorf("tree len = %d, want 1", len(body.Tree))
	}
	if body.Tree[0].Title != "docker-perms" {
		t.Errorf("tree[0].title = %q", body.Tree[0].Title)
	}
	if len(body.Tree[0].Answers) != 1 {
		t.Errorf("tree[0].answers len = %d, want 1", len(body.Tree[0].Answers))
	}
}

func TestStats_Empty(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "GET", "/api/v1/stats", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var st graph.Stats
	_ = json.Unmarshal(rr.Body.Bytes(), &st)
	if st.TotalProblems != 0 || st.TotalAnswers != 0 {
		t.Errorf("stats: %+v, want zeroed", st)
	}
	if st.AvgSolveTime != "" {
		t.Errorf("avg_solve_time = %q, want empty with no completed solves", st.AvgSolveTime)
	}
}

func TestStats_Populated(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClass(t, store, "a", "descA", "docker", "go", "1.0", "solA", graph.AnswerVerified)
	seedClass(t, store, "b", "descB", "docker", "go", "1.0", "solB", graph.AnswerPending)
	body := submitProblemRequest{ProblemClass: "c", Cadence: ingest.CadencePrePhase}
	do(t, s, "POST", "/api/v1/problems/submit", body)
	rr := do(t, s, "GET", "/api/v1/stats", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var st graph.Stats
	_ = json.Unmarshal(rr.Body.Bytes(), &st)
	if st.TotalProblems != 2 {
		t.Errorf("total_problems = %d, want 2", st.TotalProblems)
	}
	if st.TotalAnswers != 2 {
		t.Errorf("total_answers = %d, want 2", st.TotalAnswers)
	}
	if st.VerifiedAnswers != 1 {
		t.Errorf("verified_answers = %d, want 1", st.VerifiedAnswers)
	}
	if st.QueueDepth != 1 {
		t.Errorf("queue_depth = %d, want 1", st.QueueDepth)
	}
	if st.HitRate < 0.49 || st.HitRate > 0.51 {
		t.Errorf("hit_rate = %f, want ~0.5", st.HitRate)
	}
}

// TestStats_FailedSignatureExcluded guards OB-GAP-060 at the API layer:
// GET /api/v1/stats must report verified_answers that exclude status-
// verified nodes whose signatures JSON carries result='failed', so
// hit_rate stays below 1.0 while such rows exist.
func TestStats_FailedSignatureExcluded(t *testing.T) {
	s, store, _ := newTestServer(t)
	seedClassWithSigs(t, store, "a", "descA", "docker", "go", "1.0", "solA", graph.AnswerVerified, `{"result":"passed"}`)
	seedClassWithSigs(t, store, "a", "descA", "docker", "go", "1.0", "solB", graph.AnswerVerified, `{"result":"passed"}`)
	seedClassWithSigs(t, store, "a", "descA", "docker", "go", "1.0", "solC", graph.AnswerVerified, `{"result":"failed"}`)

	rr := do(t, s, "GET", "/api/v1/stats", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var st graph.Stats
	_ = json.Unmarshal(rr.Body.Bytes(), &st)
	if st.TotalAnswers != 3 {
		t.Errorf("total_answers = %d, want 3", st.TotalAnswers)
	}
	if st.VerifiedAnswers != 2 {
		t.Errorf("verified_answers = %d, want 2 (failed-signature node excluded)", st.VerifiedAnswers)
	}
	if st.HitRate < 0.66 || st.HitRate > 0.67 {
		t.Errorf("hit_rate = %f, want ≈0.667", st.HitRate)
	}
}

// TestStats_AvgSolveTime asserts /api/v1/stats reports a non-empty
// avg_solve_time once the queue holds a completed solve with real
// started_at/completed_at timestamps (OB-GAP-047).
func TestStats_AvgSolveTime(t *testing.T) {
	s, store, _ := newTestServer(t)
	// Insert a completed solve directly: started 10:00:00, completed
	// 10:02:13 -> 133s -> "2m13s".
	if _, err := store.DB().Exec(`INSERT INTO queue_entries
		(id, problem_class, status, stage, started_at, completed_at)
		VALUES ('sub_avg1', 'cls', 'complete', 'done',
			'2026-08-21 10:00:00', '2026-08-21 10:02:13')`); err != nil {
		t.Fatalf("insert completed entry: %v", err)
	}
	// A pending row with no timing must not affect the average.
	if _, err := store.DB().Exec(`INSERT INTO queue_entries
		(id, problem_class, status) VALUES ('sub_avg2', 'cls', 'pending')`); err != nil {
		t.Fatalf("insert pending entry: %v", err)
	}

	rr := do(t, s, "GET", "/api/v1/stats", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var st graph.Stats
	_ = json.Unmarshal(rr.Body.Bytes(), &st)
	if st.AvgSolveTime == "" {
		t.Fatalf("avg_solve_time is empty, want non-empty for a completed solve")
	}
	if st.AvgSolveTime != "2m13s" {
		t.Errorf("avg_solve_time = %q, want %q", st.AvgSolveTime, "2m13s")
	}
}

// TestEstimateTime pins the submit-response ETA derivation.
//
// estimated_time is per-job cost x queued depth, rounded to the nearest
// second: the per-job cost is the lab's observed mean solve time
// (internal/ingest Queue.AvgSolveTime), falling back to
// defaultPerJobEstimate when there is no history, and the total is
// capped at maxEstimateTotal. A depth of 0 (nothing queued) stays "0s".
//
// The result must remain a Go duration string so the OpenAPI
// `estimated_time: type: string` field stays parseable by
// time.ParseDuration.
func TestEstimateTime(t *testing.T) {
	const observed = 2*time.Minute + 13*time.Second // 2m13s, the stats fixture

	cases := []struct {
		name   string
		depth  int
		perJob time.Duration
		want   string
	}{
		{name: "nothing queued", depth: 0, perJob: observed, want: "0s"},
		{name: "nothing queued keeps 0s even with no history", depth: 0, perJob: 0, want: "0s"},
		{name: "negative depth", depth: -3, perJob: observed, want: "0s"},
		{name: "no history falls back to the default per-job cost", depth: 1, perJob: 0, want: "30s"},
		{name: "negative average falls back to the default per-job cost", depth: 2, perJob: -time.Second, want: "1m0s"},
		{name: "observed average scales with depth", depth: 3, perJob: observed, want: "6m39s"},
		{name: "rounds to the nearest second", depth: 1, perJob: 1500 * time.Millisecond, want: "2s"},
		{name: "just under the cap is not capped", depth: 10, perJob: 2*time.Minute + 57*time.Second, want: "29m30s"},
		{name: "caps at 30m", depth: 20, perJob: 2*time.Minute + 57*time.Second, want: "30m0s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := estimateTime(tc.depth, tc.perJob)
			if got != tc.want {
				t.Fatalf("estimateTime(%d, %s) = %q, want %q", tc.depth, tc.perJob, got, tc.want)
			}
			// The field is typed string in the OpenAPI spec; the value
			// must stay a parseable Go duration.
			if _, err := time.ParseDuration(got); err != nil {
				t.Fatalf("estimateTime(%d, %s) = %q is not a parseable duration: %v",
					tc.depth, tc.perJob, got, err)
			}
		})
	}

	// Zero means "nothing ahead of you", not "no estimate": the depth 0
	// short-circuit must win over any per-job cost.
	if got := estimateTime(0, defaultPerJobEstimate); got != "0s" {
		t.Errorf("estimateTime(0, default) = %q, want %q", got, "0s")
	}
	// The cap is a real ceiling, not a constant: a deeper queue at the
	// same observed average must not exceed it.
	if got := estimateTime(1000, observed); got != maxEstimateTotal.String() {
		t.Errorf("estimateTime(1000, %s) = %q, want capped %q", observed, got, maxEstimateTotal.String())
	}
}

// TestSubmit_EstimatedTimeFromObservedAverage asserts the submit
// response's estimated_time is derived from the lab's observed mean
// solve time rather than a fixed 30s per queue position (DF-OFF-BY-ONE-5).
//
// Fixture: one completed solve of 2m13s (same timestamps as
// TestStats_AvgSolveTime) is the observed average, so a single queued
// submission — the only pending row — is promised 2m13s, not "30s".
// Seeding two more pending rows makes the post-enqueue depth 3 and the
// promise 3 x 2m13s = 6m39s, proving the value scales with the queue.
func TestSubmit_EstimatedTimeFromObservedAverage(t *testing.T) {
	s, store, _ := newTestServer(t)
	// started 10:00:00, completed 10:02:13 -> 133s -> observed mean "2m13s".
	if _, err := store.DB().Exec(`INSERT INTO queue_entries
		(id, problem_class, status, stage, started_at, completed_at)
		VALUES ('sub_eta0', 'cls', 'complete', 'done',
			'2026-08-21 10:00:00', '2026-08-21 10:02:13')`); err != nil {
		t.Fatalf("insert completed entry: %v", err)
	}

	submit := func(t *testing.T, class string) submitProblemResponse {
		t.Helper()
		rr := do(t, s, "POST", "/api/v1/problems/submit", submitProblemRequest{
			ProblemClass: class,
			Environment:  "docker",
			Language:     "go",
			Cadence:      ingest.CadencePrePhase,
		})
		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200, body = %s", rr.Code, rr.Body.String())
		}
		var resp submitProblemResponse
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return resp
	}

	// Depth 1: only the submission's own pending row is queued.
	first := submit(t, "deadlock-in-worker-pool")
	if first.Position != 1 {
		t.Fatalf("position = %d, want 1 (own pending row)", first.Position)
	}
	if first.EstimatedTime != "2m13s" {
		t.Errorf("estimated_time = %q, want %q (observed mean solve time, not the fixed 30s default)",
			first.EstimatedTime, "2m13s")
	}
	if _, err := time.ParseDuration(first.EstimatedTime); err != nil {
		t.Errorf("estimated_time = %q is not parseable: %v", first.EstimatedTime, err)
	}

	// Two more jobs queued ahead. The first submission is still pending
	// (nothing solved it), so the queue now holds three pending rows and
	// the next submission becomes depth 4 -> 4 x 2m13s = 8m52s.
	for _, id := range []string{"sub_eta_p1", "sub_eta_p2"} {
		if _, err := store.DB().Exec(`INSERT INTO queue_entries
			(id, problem_class, status) VALUES (?, 'other-class', 'pending')`, id); err != nil {
			t.Fatalf("insert pending entry %s: %v", id, err)
		}
	}
	second := submit(t, "goroutine-leak-in-http-server")
	if second.Position != 4 {
		t.Fatalf("position = %d, want 4 (three queued + own row)", second.Position)
	}
	if second.EstimatedTime != "8m52s" {
		t.Errorf("estimated_time = %q, want %q (4 x observed mean)", second.EstimatedTime, "8m52s")
	}
}

// TestStats_SolverAvailable asserts the stats response always carries the
// solver_available field and that it mirrors Server.SolverAvailable — the
// signal that tells users why their submissions sit queued forever.
func TestStats_SolverAvailable(t *testing.T) {
	s, _, _ := newTestServer(t)
	s.SolverAvailable = false

	rr := do(t, s, "GET", "/api/v1/stats", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	v, ok := body["solver_available"]
	if !ok {
		t.Fatalf("solver_available missing from stats response: %v", body)
	}
	if v != false {
		t.Errorf("solver_available = %v, want false (no solver wired)", v)
	}

	s.SolverAvailable = true
	rr = do(t, s, "GET", "/api/v1/stats", nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body = map[string]any{}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body["solver_available"] != true {
		t.Errorf("solver_available = %v, want true", body["solver_available"])
	}
}

// --- Routing sanity ------------------------------------------------------

// Verify a known-unknown path returns 404, not 500 (a common bug
// when mux patterns are misconfigured).
func TestUnknownPath404(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "GET", "/this/does/not/exist", nil)
	if rr.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rr.Code)
	}
}

// intToString converts an int64 to a string without importing strconv
// at the test scope (kept narrow).
func intToString(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// --- Export/Import handler tests -----------------------------------------

// TestExportNotConfigured verifies the handler returns 501 when
// ExportLocalDir is empty (the default for newTestServer).
func TestExportNotConfigured(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "POST", "/api/v1/export", exportRequest{
		TargetRepo: "https://github.com/example/repo.git",
		AnswerIDs:  []int64{1},
	})
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rr.Code)
	}
}

// TestExportBadRequest verifies the handler returns 400 for missing
// target_repo or empty answer_ids.
func TestExportBadRequest(t *testing.T) {
	s, _, _ := newTestServer(t)
	s.ExportLocalDir = "/tmp/obo-test-export"

	// Missing target_repo.
	rr := do(t, s, "POST", "/api/v1/export", exportRequest{
		AnswerIDs: []int64{1},
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("missing target_repo: status = %d, want 400", rr.Code)
	}

	// Empty answer_ids.
	rr = do(t, s, "POST", "/api/v1/export", exportRequest{
		TargetRepo: "https://github.com/example/repo.git",
	})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("empty answer_ids: status = %d, want 400", rr.Code)
	}

	// Malformed JSON.
	req := httptest.NewRequest("POST", "/api/v1/export", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON: status = %d, want 400", w.Code)
	}
}

// TestExportSuccess verifies the full export path end-to-end: a seeded
// class + verified answer, a real local bare git repo as target_repo,
// and a POST that must return 200 with a commit SHA and files_changed
// (regression: the handler used to build ExportItem with ClassID always
// 0, so every export request died with 500 export_failed).
func TestExportSuccess(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed — skipping export integration test")
	}
	// Engine runs plain `git commit`; pin identity so the commit does
	// not depend on ambient ~/.gitconfig.
	t.Setenv("GIT_AUTHOR_NAME", "Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "test@example.com")
	t.Setenv("GIT_COMMITTER_NAME", "Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "test@example.com")

	s, store, _ := newTestServer(t)
	s.ExportLocalDir = t.TempDir()

	ctx := context.Background()
	pc, created, err := store.UpsertProblemClass(ctx, "export-handler-class", "handler success-path class")
	if err != nil {
		t.Fatalf("UpsertProblemClass: %v", err)
	}
	if !created {
		t.Fatal("expected class to be created")
	}
	answerID, err := store.CreateAnswerNode(ctx, pc.ID, 0,
		"docker", "go", "go-1.26",
		"Use `COPY --chown=appuser:appuser` in Dockerfile.",
		"Verified in Docker 24.0+.",
		"",
	)
	if err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}
	if err := store.UpdateAnswerStatus(ctx, answerID, graph.AnswerVerified); err != nil {
		t.Fatalf("UpdateAnswerStatus: %v", err)
	}

	// Seed a bare repo with one commit so the engine can clone + push.
	remote := initBareRepoForHandler(t, "main")
	seedBareRepoForHandler(t, remote, "main")

	const wantMsg = "export: handler success-path answers"
	rresp := do(t, s, "POST", "/api/v1/export", exportRequest{
		TargetRepo:    remote,
		AnswerIDs:     []int64{answerID},
		CommitMessage: wantMsg,
	})
	if rresp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rresp.Code, rresp.Body.String())
	}
	var body exportResponse
	if err := json.Unmarshal(rresp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.CommitSHA == "" {
		t.Error("commit_sha is empty")
	}
	if body.FilesChanged < 1 {
		t.Errorf("files_changed = %d, want >= 1", body.FilesChanged)
	}

	// DF-OFF-BY-ONE-8: commit_message from the request must drive the
	// commit message. Read it back from the bare remote (not the
	// clone) so the assertion covers the pushed commit.
	cmd := exec.Command("git", "--git-dir", remote, "log", "-1", "--pretty=%s", "main")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("read remote commit subject: %v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != wantMsg {
		t.Errorf("remote commit subject = %q, want %q (commit_message was dropped)", got, wantMsg)
	}

	// Unknown answer_id is a client error (4xx), not a 500 export_failed.
	rresp = do(t, s, "POST", "/api/v1/export", exportRequest{
		TargetRepo: remote,
		AnswerIDs:  []int64{answerID + 9999},
	})
	if rresp.Code < 400 || rresp.Code >= 500 {
		t.Errorf("unknown answer_id: status = %d, want 4xx; body: %s", rresp.Code, rresp.Body.String())
	}
}

// TestExportInvalidAnswerID400 verifies an answer_id that cannot be
// exported is reported as an actionable 400 that names the offending
// request field. Regression (DF-OFF-BY-ONE-8): the export path used to
// answer with 500 export_failed and a message leaking internal
// ExportItem fields ("export item class=0 answer=43: ...").
func TestExportInvalidAnswerID400(t *testing.T) {
	s, _, _ := newTestServer(t)
	s.ExportLocalDir = t.TempDir()

	rr := do(t, s, "POST", "/api/v1/export", exportRequest{
		TargetRepo: "https://github.com/example/repo.git",
		AnswerIDs:  []int64{424242},
	})
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rr.Code, rr.Body.String())
	}
	msg := rr.Body.String()
	if strings.Contains(msg, "class=") {
		t.Errorf("error body leaks internal struct fields: %s", msg)
	}
	if !strings.Contains(msg, "answer_ids") {
		t.Errorf("error body does not name the offending field answer_ids: %s", msg)
	}
}

// initBareRepoForHandler creates a bare git repo with HEAD on branch.
func initBareRepoForHandler(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	barePath := filepath.Join(dir, "remote.git")
	cmd := exec.Command("git", "init", "--bare", "-b", branch, barePath)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	return barePath
}

// seedBareRepoForHandler pushes an initial commit to the bare repo so
// the engine's clone --branch succeeds.
func seedBareRepoForHandler(t *testing.T, barePath, branch string) {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "clone", barePath, dir).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args[0], err, out)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test Repo\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "initial"},
		{"branch", "-M", branch},
		{"push", "origin", branch},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args[0], err, out)
		}
	}
}

// TestImportNotConfigured verifies the handler returns 501 when
// ImportLocalDir is empty.
func TestImportNotConfigured(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "POST", "/api/v1/import", importRequest{
		SourceRepo: "https://github.com/example/repo.git",
	})
	if rr.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want 501", rr.Code)
	}
}

// TestImportBadRequest verifies the handler returns 400 for missing
// source_repo.
func TestImportBadRequest(t *testing.T) {
	s, _, _ := newTestServer(t)
	s.ImportLocalDir = "/tmp/obo-test-import"

	// Missing source_repo.
	rr := do(t, s, "POST", "/api/v1/import", importRequest{})
	if rr.Code != http.StatusBadRequest {
		t.Errorf("missing source_repo: status = %d, want 400", rr.Code)
	}

	// Malformed JSON.
	req := httptest.NewRequest("POST", "/api/v1/import", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("malformed JSON: status = %d, want 400", w.Code)
	}
}

// TestImportSourceRepoMismatch_Conflict verifies that a source_repo which
// does not match the origin of the existing import clone is rejected with
// 409 source_repo_mismatch — never 200 with skipped>=1 from the stale clone
// (DF-OFF-BY-ONE-7).
func TestImportSourceRepoMismatch_Conflict(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed — skipping import integration test")
	}
	s, _, _ := newTestServer(t)

	// Repo A holds a real answer, so pre-fix the stale clone would import
	// it and answer 200.
	repoA := initBareRepoForHandler(t, "main")
	seedBareRepoForHandler(t, repoA, "main")
	pushAnswerFilesForHandler(t, repoA, "main", "stale-clone-class", "docker", "go-1.26")

	// ImportLocalDir already holds a clone of repo A.
	localDir := filepath.Join(t.TempDir(), "clone")
	if out, err := exec.Command("git", "clone", repoA, localDir).CombinedOutput(); err != nil {
		t.Fatalf("manual clone: %v\n%s", err, out)
	}
	s.ImportLocalDir = localDir

	bogusRepo := filepath.Join(t.TempDir(), "does-not-exist-repo")
	rr := do(t, s, "POST", "/api/v1/import", importRequest{SourceRepo: bogusRepo})

	if rr.Code == http.StatusOK {
		t.Fatalf("status = 200 for mismatched source_repo — stale clone was reused; body: %s", rr.Body.String())
	}
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body: %s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rr.Body.String(), err)
	}
	if body["error"] != "source_repo_mismatch" {
		t.Errorf("error = %q, want source_repo_mismatch", body["error"])
	}
	for _, want := range []string{repoA, bogusRepo} {
		if !strings.Contains(body["message"], want) {
			t.Errorf("message %q does not name %q", body["message"], want)
		}
	}
}

// pushAnswerFilesForHandler pushes one minimal answer directory into the bare
// repo, mirroring the export layout, so an import has something to import.
func pushAnswerFilesForHandler(t *testing.T, barePath, branch, classTitle, env, version string) {
	t.Helper()
	dir := t.TempDir()
	if out, err := exec.Command("git", "clone", barePath, dir).CombinedOutput(); err != nil {
		t.Fatalf("git clone: %v\n%s", err, out)
	}
	ansDir := filepath.Join(dir, "pre-solve-answers", classTitle, env, version)
	if err := os.MkdirAll(ansDir, 0o755); err != nil {
		t.Fatalf("mkdir answer dir: %v", err)
	}
	solution := "# Problem: " + classTitle + "\n\n" +
		"**Environment:** " + env + "\n" +
		"**Language:** go " + version + "\n" +
		"**Status:** verified\n\n---\n\n## Solution\n\nUse chmod.\n"
	files := map[string]string{
		"solution.md":     solution,
		"evidence.md":     "# Evidence\n\n**Status:** verified\n\n---\n\nTested.\n",
		"signatures.json": "{}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(ansDir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	for _, args := range [][]string{
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"add", "."},
		{"commit", "-m", "add answers"},
		{"push", "origin", branch},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", args[0], err, out)
		}
	}
}

// TestExportImportRouteRegistered verifies the routes are wired by
// checking that a POST to them does NOT return 404 (it should return
// 501 when not configured, proving the route exists).
func TestExportImportRouteRegistered(t *testing.T) {
	s, _, _ := newTestServer(t)

	rr := do(t, s, "POST", "/api/v1/export", exportRequest{
		TargetRepo: "x",
		AnswerIDs:  []int64{1},
	})
	if rr.Code == http.StatusNotFound {
		t.Error("POST /api/v1/export returned 404 — route not registered")
	}

	rr = do(t, s, "POST", "/api/v1/import", importRequest{
		SourceRepo: "x",
	})
	if rr.Code == http.StatusNotFound {
		t.Error("POST /api/v1/import returned 404 — route not registered")
	}
}

// --- Multipart file upload tests ------------------------------------------

func TestSubmitWithFiles(t *testing.T) {
	s, _, _ := newTestServer(t)

	// Create a temp dir for attachments.
	dir := t.TempDir()
	s.AttachmentsDir = dir

	// Build multipart form body: "data" field + a file.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// Write the JSON data field.
	dataJSON := `{"problem_class":"go-npe","environment":"linux","language":"go","version":"1.26","description":"NPE","cadence":"post-debug"}`
	w, _ := mw.CreateFormField("data")
	_, _ = w.Write([]byte(dataJSON))

	// Write a file attachment.
	fw, _ := mw.CreateFormFile("logfile", "error.log")
	_, _ = fw.Write([]byte("panic: runtime error\n"))

	// Write another file.
	fw2, _ := mw.CreateFormFile("trace", "trace.txt")
	_, _ = fw2.Write([]byte("goroutine 1 [running]:\n"))

	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/problems/submit", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var resp submitProblemResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "queued" {
		t.Fatalf("expected status queued, got %s", resp.Status)
	}

	// Verify files were saved.
	entries, _ := os.ReadDir(dir)
	if len(entries) < 2 {
		t.Fatalf("expected 2 files in attachments dir, got %d", len(entries))
	}
}

func TestSubmitMultipartWithoutDataField(t *testing.T) {
	s, _, _ := newTestServer(t)
	s.AttachmentsDir = t.TempDir()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("file", "test.txt")
	_, _ = fw.Write([]byte("content"))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/problems/submit", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestSubmitJSONStillWorks(t *testing.T) {
	s, _, _ := newTestServer(t)

	body := submitProblemRequest{
		ProblemClass: "go-npe-json",
		Cadence:      "post-debug",
	}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/api/v1/problems/submit", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}

func TestSubmitMultipartNoAttachmentsDir(t *testing.T) {
	s, _, _ := newTestServer(t)
	// AttachmentsDir is empty — files should be silently discarded.

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	w, _ := mw.CreateFormField("data")
	_, _ = w.Write([]byte(`{"problem_class":"no-dir","cadence":"post-debug"}`))
	fw, _ := mw.CreateFormFile("file", "test.txt")
	_, _ = fw.Write([]byte("content"))
	mw.Close()

	req := httptest.NewRequest("POST", "/api/v1/problems/submit", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
}
