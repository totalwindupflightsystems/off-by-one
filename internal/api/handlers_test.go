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
