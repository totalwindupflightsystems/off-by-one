package api

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestGetQueueStatus_NotFound(t *testing.T) {
	s, _, _ := newTestServer(t)
	rr := do(t, s, "GET", "/api/v1/queue/sub_nonexistent", nil)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
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

	rresp := do(t, s, "POST", "/api/v1/export", exportRequest{
		TargetRepo: remote,
		AnswerIDs:  []int64{answerID},
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

	// Unknown answer_id is a client error (4xx), not a 500 export_failed.
	rresp = do(t, s, "POST", "/api/v1/export", exportRequest{
		TargetRepo: remote,
		AnswerIDs:  []int64{answerID + 9999},
	})
	if rresp.Code < 400 || rresp.Code >= 500 {
		t.Errorf("unknown answer_id: status = %d, want 4xx; body: %s", rresp.Code, rresp.Body.String())
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
