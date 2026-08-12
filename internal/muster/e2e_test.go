package muster

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/totalwindupflightsystems/off-by-one/internal/api"
	"github.com/totalwindupflightsystems/off-by-one/internal/graph"
	"github.com/totalwindupflightsystems/off-by-one/internal/ingest"
	schemasql "github.com/totalwindupflightsystems/off-by-one/sql/schema"
)

// mustermock is a stand-in for the real Muster MCP server. It speaks the
// same wire protocol Muster would: a JSON-over-HTTP client that invokes
// the four core MCP tools (submitProblem, discoverSolution, listProblems,
// getQueueStatus) by hitting the Off-by-One REST API. We don't spawn a
// real Muster process because that adds 10+ seconds of startup and a
// binary that may not be installed in CI — the test goal is to verify
// the *integration contract* (Off-by-One accepts Muster-shaped traffic
// and produces the responses Muster expects), not the Muster binary
// itself.
type mustermock struct {
	srv     *httptest.Server
	apiSrv  *api.Server
	store   *graph.Store
	queue   *ingest.Queue
	hc      *http.Client
	calls   int64
	muCalls sync.Mutex
	log     []string
}

func newMustermock(t *testing.T) *mustermock {
	t.Helper()
	store, err := graph.OpenShared("muster_e2e_" + t.Name() + "_" + time.Now().Format("150405.000000"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.ApplyExtra(schemasql.QueueSchema); err != nil {
		t.Fatalf("apply queue schema: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	q, err := ingest.Open(store)
	if err != nil {
		t.Fatalf("open queue: %v", err)
	}
	apiSrv := api.New(store, q, []byte(testOpenAPISpec()))
	apiSrv.SolverAvailable = true
	srv := httptest.NewServer(apiSrv.Handler())
	t.Cleanup(srv.Close)

	hc := &http.Client{Timeout: 5 * time.Second}
	return &mustermock{
		srv:    srv,
		apiSrv: apiSrv,
		store:  store,
		queue:  q,
		hc:     hc,
	}
}

// testOpenAPISpec returns a minimal Muster-compatible OpenAPI 3.0.3
// spec (as JSON — the bridge decodes with json.NewDecoder) with the
// ten Muster operationIds. We embed it as a string so the test doesn't
// depend on the on-disk pkg/api/openapi.yaml.
func testOpenAPISpec() string {
	return `{
  "openapi": "3.0.3",
  "info": {"title": "Off-by-One", "version": "1.0.0"},
  "paths": {
    "/api/v1/problems/submit": {
      "post": {
        "operationId": "submitProblem",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/SubmitProblemRequest"}
            }
          }
        }
      }
    },
    "/api/v1/problems/discover": {
      "post": {
        "operationId": "discoverSolution",
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/DiscoverRequest"}
            }
          }
        }
      }
    },
    "/api/v1/problems": {
      "get": {"operationId": "listProblems"}
    },
    "/api/v1/queue/{submission_id}": {
      "get": {"operationId": "getQueueStatus"}
    },
    "/api/v1/problems/{class}/related": {
      "get": {"operationId": "getRelated"}
    },
    "/api/v1/queue": {
      "get": {"operationId": "listQueue"}
    },
    "/api/v1/export": {
      "post": {
        "operationId": "exportToGit",
        "requestBody": {
          "content": {
            "application/json": {}
          }
        }
      }
    },
    "/api/v1/import": {
      "post": {
        "operationId": "importFromGit",
        "requestBody": {
          "content": {
            "application/json": {}
          }
        }
      }
    },
    "/api/v1/taxonomy": {
      "get": {"operationId": "getTaxonomy"}
    },
    "/api/v1/stats": {
      "get": {"operationId": "getStats"}
    }
  }
}`
}

func (m *mustermock) postJSON(path string, body any) (int, map[string]any) {
	m.logCall("POST", path)
	b, _ := json.Marshal(body)
	resp, err := m.hc.Post(m.srv.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		m.logf("post %s: %v", path, err)
		return 0, nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return resp.StatusCode, out
}

func (m *mustermock) getJSON(path string) (int, map[string]any) {
	m.logCall("GET", path)
	resp, err := m.hc.Get(m.srv.URL + path)
	if err != nil {
		m.logf("get %s: %v", path, err)
		return 0, nil
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(data, &out)
	return resp.StatusCode, out
}

func (m *mustermock) logCall(method, path string) {
	atomic.AddInt64(&m.calls, 1)
	m.muCalls.Lock()
	m.log = append(m.log, method+" "+path)
	m.muCalls.Unlock()
}

func (m *mustermock) logf(format string, args ...any) {
	m.muCalls.Lock()
	m.log = append(m.log, fmt.Sprintf(format, args...))
	m.muCalls.Unlock()
}

// submitProblem invokes the submitProblem MCP tool. Mirrors how Muster
// would auto-generate the tool from the OpenAPI spec.
func (m *mustermock) submitProblem(class, env, lang, version, cadence, desc string) (int, map[string]any) {
	body := map[string]any{
		"problem_class": class,
		"environment":   env,
		"language":      lang,
		"version":       version,
		"description":   desc,
		"cadence":       cadence,
	}
	return m.postJSON("/api/v1/problems/submit", body)
}

// discoverSolution invokes the discoverSolution MCP tool.
func (m *mustermock) discoverSolution(class, env, lang, version string) (int, map[string]any) {
	body := map[string]any{
		"problem_class": class,
		"environment":   env,
		"language":      lang,
		"version":       version,
	}
	return m.postJSON("/api/v1/problems/discover", body)
}

// getQueueStatus invokes the getQueueStatus MCP tool.
func (m *mustermock) getQueueStatus(submissionID string) (int, map[string]any) {
	return m.getJSON("/api/v1/queue/" + submissionID)
}

// --- AC 1: Full E2E — Muster client → submit → queue → solve → graph → discover ---

func TestMusterE2E_FullRoundTrip(t *testing.T) {
	m := newMustermock(t)

	// Step 1: Muster agent submits a problem.
	status, body := m.submitProblem(
		"auth-token-rotation",
		"production",
		"go",
		"1.22.0",
		"post-debug",
		"JWT validation fails after token rotation in staging",
	)
	if status != 200 {
		t.Fatalf("submitProblem: status=%d body=%v", status, body)
	}
	if body["status"] != "queued" {
		t.Fatalf("submitProblem: expected status=queued, got %v", body["status"])
	}
	submissionID, _ := body["submission_id"].(string)
	if submissionID == "" {
		t.Fatal("submitProblem: empty submission_id")
	}

	// Step 2: Simulate the idle cron — dequeue picks the pending entry.
	entry, err := m.queue.Dequeue(context.Background())
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if entry == nil {
		t.Fatal("dequeue: no entry returned")
	}
	if entry.Status != ingest.StatusInProgress {
		t.Fatalf("dequeue: status=%s, want in_progress", entry.Status)
	}

	// Step 3: Mock "sandbox solve" — instead of running Pi Agent inside
	// bwrap (which is what real cron does), we create a verified answer
	// directly in the graph. The point of this test is the wire
	// protocol, not the actual solver.
	pc, _, err := m.store.UpsertProblemClass(context.Background(), entry.ProblemClass, "JWT rotation issue")
	if err != nil {
		t.Fatalf("upsert class: %v", err)
	}
	answerID, err := m.store.CreateAnswerNode(
		context.Background(), pc.ID, 0,
		entry.Environment, entry.Language, entry.Version,
		"Rotate the JWT signing key and clear the local cache before redeploying.",
		"verified-by-sandbox: token validation passes after rotation",
		`{"signatures":["auth.refresh"]}`,
	)
	if err != nil {
		t.Fatalf("create answer: %v", err)
	}
	if err := m.store.UpdateAnswerStatus(context.Background(), answerID, "verified"); err != nil {
		t.Fatalf("mark verified: %v", err)
	}
	if err := m.queue.MarkComplete(context.Background(), entry.ID, answerID); err != nil {
		t.Fatalf("mark complete: %v", err)
	}

	// Step 4: Muster queries getQueueStatus to confirm the entry is
	// done with a linked answer.
	status, gsBody := m.getQueueStatus(submissionID)
	if status != 200 {
		t.Fatalf("getQueueStatus: status=%d body=%v", status, gsBody)
	}
	if gsBody["status"] != "complete" {
		t.Fatalf("getQueueStatus: status=%v, want complete", gsBody["status"])
	}

	// Step 5: Muster queries discoverSolution — the agent looks for
	// the existing answer before doing more work.
	status, discBody := m.discoverSolution("auth-token-rotation", "production", "go", "1.22.0")
	if status != 200 {
		t.Fatalf("discoverSolution: status=%d body=%v", status, discBody)
	}
	if discBody["found"] != true {
		t.Fatalf("discoverSolution: found=%v, want true", discBody["found"])
	}
	ans, _ := discBody["answer"].(map[string]any)
	if ans == nil {
		t.Fatal("discoverSolution: answer missing")
	}
	sol, _ := ans["solution"].(string)
	if !strings.Contains(sol, "Rotate the JWT") {
		t.Fatalf("discoverSolution: solution does not match — got %q", sol)
	}

	if c := atomic.LoadInt64(&m.calls); c < 3 {
		t.Errorf("expected ≥3 mustermock calls, got %d", c)
	}
}

// --- AC 2: Error paths — duplicate, not found, invalid cadence ---

func TestMusterE2E_DuplicateSubmissionDedup(t *testing.T) {
	m := newMustermock(t)

	// First submission: queued.
	status1, body1 := m.submitProblem("slow-query", "staging", "python", "3.12", "pre-phase", "Slow query on user table")
	if status1 != 200 {
		t.Fatalf("first submit: status=%d", status1)
	}
	if body1["status"] != "queued" {
		t.Fatalf("first submit: status=%v, want queued", body1["status"])
	}

	// Same (class, env, lang, version) tuple — must dedup.
	status2, body2 := m.submitProblem("slow-query", "staging", "python", "3.12", "pre-phase", "Same problem again")
	if status2 != http.StatusConflict {
		t.Fatalf("dup submit: status=%d, want 409 (dedup)", status2)
	}
	if body2["status"] != "deduplicated" {
		t.Fatalf("dup submit: status=%v, want deduplicated", body2["status"])
	}
}

func TestMusterE2E_DuplicateAgainstVerifiedAnswer(t *testing.T) {
	m := newMustermock(t)
	ctx := context.Background()

	// Seed a verified answer.
	pc, _, err := m.store.UpsertProblemClass(ctx, "race-condition", "Concurrency issue")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	id, err := m.store.CreateAnswerNode(ctx, pc.ID, 0, "prod", "go", "1.21", "Use sync.Mutex", "verified", "{}")
	if err != nil {
		t.Fatalf("create answer: %v", err)
	}
	_ = m.store.UpdateAnswerStatus(ctx, id, "verified")

	// Submit a problem that matches the verified answer's (class, env, lang, version).
	status, body := m.submitProblem("race-condition", "prod", "go", "1.21", "pre-phase", "Hit a race")
	if status != http.StatusConflict {
		t.Fatalf("submit against verified answer: status=%d, want 409", status)
	}
	if body["status"] != "deduplicated" {
		t.Fatalf("submit against verified answer: status=%v, want deduplicated", body["status"])
	}
}

func TestMusterE2E_DiscoverNonexistentProblem(t *testing.T) {
	m := newMustermock(t)

	status, body := m.discoverSolution("nonexistent-class-xyz", "prod", "rust", "1.75")
	if status != 404 {
		t.Fatalf("discover nonexistent: status=%d, want 404", status)
	}
	if body["error"] != "not_found" {
		t.Fatalf("discover nonexistent: error=%v, want not_found", body["error"])
	}
}

func TestMusterE2E_InvalidCadenceRejected(t *testing.T) {
	m := newMustermock(t)

	// "weekly" is not a valid cadence.
	status, _ := m.submitProblem("some-class", "prod", "go", "1.22", "weekly", "test")
	if status != 400 {
		t.Fatalf("invalid cadence: status=%d, want 400", status)
	}
}

func TestMusterE2E_EmptyClassRejected(t *testing.T) {
	m := newMustermock(t)

	// Empty problem_class — handler-level guard.
	status, _ := m.submitProblem("", "prod", "go", "1.22", "pre_phase", "test")
	if status != 400 {
		t.Fatalf("empty class: status=%d, want 400", status)
	}
}

// --- AC 3: Queue lifecycle pending → in_progress → complete ---

func TestMusterE2E_QueueLifecycleTransitions(t *testing.T) {
	m := newMustermock(t)
	ctx := context.Background()

	// 1. Submit → pending.
	_, body := m.submitProblem("lifecycle-class", "prod", "go", "1.22", "pre-phase", "test")
	id, _ := body["submission_id"].(string)

	entry, err := m.queue.Get(ctx, id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if entry.Status != "pending" {
		t.Fatalf("after submit: status=%s, want pending", entry.Status)
	}

	// 2. Dequeue → in_progress.
	entry, err = m.queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	if entry == nil {
		t.Fatal("dequeue returned nil")
	}
	if entry.Status != "in_progress" {
		t.Fatalf("after dequeue: status=%s, want in_progress", entry.Status)
	}

	// 3. Solve + MarkComplete → complete.
	pc, _, err := m.store.UpsertProblemClass(ctx, entry.ProblemClass, "")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	ansID, err := m.store.CreateAnswerNode(ctx, pc.ID, 0, entry.Environment, entry.Language, entry.Version, "solution", "evidence", "{}")
	if err != nil {
		t.Fatalf("create answer: %v", err)
	}
	_ = m.store.UpdateAnswerStatus(ctx, ansID, "verified")
	if err := m.queue.MarkComplete(ctx, entry.ID, ansID); err != nil {
		t.Fatalf("mark complete: %v", err)
	}

	// 4. Verify final state via Muster's getQueueStatus tool.
	_, gsBody := m.getQueueStatus(id)
	if gsBody["status"] != "complete" {
		t.Fatalf("after complete: status=%v, want complete", gsBody["status"])
	}

	// 5. Verify the entry is no longer dequeueable.
	next, err := m.queue.Dequeue(ctx)
	if err != nil {
		t.Fatalf("dequeue after complete: %v", err)
	}
	if next != nil {
		t.Errorf("expected no pending entry after complete, got %+v", next)
	}
}

// --- AC 4: Muster reconnection — kill Muster → restart → verify tools still work ---

func TestMusterE2E_ReconnectionAfterRestart(t *testing.T) {
	m := newMustermock(t)
	ctx := context.Background()

	// Use the first mustermock to submit a problem, then "kill" it.
	status, body := m.submitProblem("reconnect-class", "prod", "go", "1.22", "pre-phase", "before reconnect")
	if status != 200 {
		t.Fatalf("first submit: %d", status)
	}
	id, _ := body["submission_id"].(string)
	if id == "" {
		t.Fatal("no submission id")
	}

	// Simulate Muster crash by closing the mock's HTTP client. In a
	// real scenario, Muster would lose its connection and the
	// Off-by-One server would still be running.
	m.hc.CloseIdleConnections()

	// The Off-by-One server is still alive. A fresh Muster client
	// (new http.Client, new state) can resume work.
	fresh := &mustermock{
		srv:    m.srv, // Off-by-One is still up
		apiSrv: m.apiSrv,
		store:  m.store,
		queue:  m.queue,
		hc:     &http.Client{Timeout: 5 * time.Second},
	}

	// 1. Status query from fresh Muster should still work.
	status, gsBody := fresh.getQueueStatus(id)
	if status != 200 {
		t.Fatalf("post-reconnect getQueueStatus: status=%d body=%v", status, gsBody)
	}
	if gsBody["submission_id"] != id {
		t.Errorf("post-reconnect: submission_id mismatch %v vs %s", gsBody["submission_id"], id)
	}

	// 2. New submission from fresh Muster must work.
	status, body2 := fresh.submitProblem("after-reconnect", "prod", "go", "1.22", "post-debug", "new problem")
	if status != 200 {
		t.Fatalf("post-reconnect submit: %d", status)
	}
	if body2["status"] != "queued" {
		t.Fatalf("post-reconnect submit: status=%v, want queued", body2["status"])
	}

	// 3. Bridge healthcheck should report server up + spec valid.
	bridge := NewBridge(m.srv.URL)
	health := bridge.HealthCheck(ctx)
	if !health.ServerUp {
		t.Errorf("post-reconnect: ServerUp=false; health=%+v", health)
	}
	if !health.SpecValid {
		t.Errorf("post-reconnect: SpecValid=false; health=%+v", health)
	}
}

// --- AC 5: All E2E tests must pass in -short mode (no long-running solve) ---

// TestMusterE2E_ShortModeOnly documents the constraint that this entire
// test file must run in -short mode (no bwrap, no Pi Agent). If a
// future test in this file requires a long-running solve, add a
// `testing.Short()` guard. Right now the file has no skip needed.
func TestMusterE2E_ShortModeOnly(t *testing.T) {
	if !testing.Short() {
		t.Skip("this test exists to document that all E2E tests in this file MUST pass under -short")
	}
}

// --- Bridge integration: HealthCheck reports full Muster visibility ---

func TestMusterE2E_BridgeReportsToolsAfterConnection(t *testing.T) {
	m := newMustermock(t)
	bridge := NewBridge(m.srv.URL)
	bridge.MarkConnected(true)

	if !bridge.IsConnected() {
		t.Error("after MarkConnected(true): IsConnected=false")
	}

	// Healthcheck against the live Off-by-One.
	health := bridge.HealthCheck(context.Background())
	if !health.ServerUp {
		t.Errorf("ServerUp=false in health check: %+v", health)
	}
	if !health.SpecValid {
		t.Errorf("SpecValid=false: %+v", health)
	}

	// After the bridge sees MCP activity, TotalCalls should reflect it.
	bridge.LogToolCall(ToolCall{Tool: "submitProblem", Method: "POST", Path: "/api/v1/problems/submit", Status: 200, Duration: "10ms"})
	if bridge.TotalCalls() != 1 {
		t.Errorf("TotalCalls=%d, want 1", bridge.TotalCalls())
	}
	calls := bridge.RecentCalls(5)
	if len(calls) != 1 || calls[0].Tool != "submitProblem" {
		t.Errorf("RecentCalls unexpected: %+v", calls)
	}
}
