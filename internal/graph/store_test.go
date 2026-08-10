package graph

import (
	"context"
	"path/filepath"
	"testing"
)

// newTestStore returns a Store backed by a fresh in-memory SQLite DB.
// Each test gets its own DB to avoid state bleed.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustGetClassID(t *testing.T, s *Store, ctx context.Context, title string) int64 {
	t.Helper()
	pc, err := s.GetProblemClassByTitle(ctx, title)
	if err != nil {
		t.Fatalf("GetProblemClassByTitle(%q): %v", title, err)
	}
	return pc.ID
}

func TestStore_OpenInMemory(t *testing.T) {
	s := newTestStore(t)
	if s == nil {
		t.Fatal("Open returned nil")
	}
	// Schema must have created the three tables.
	for _, table := range []string{"problem_classes", "answer_nodes", "problem_edges"} {
		var name string
		err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing: %v", table, err)
		}
	}
	// FTS5 virtual tables must exist.
	for _, table := range []string{"problem_classes_fts", "answer_nodes_fts"} {
		var name string
		err := s.db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name = ?`, table,
		).Scan(&name)
		if err != nil {
			t.Errorf("fts table %s missing: %v", table, err)
		}
	}
}

func TestStore_OpenOnDisk(t *testing.T) {
	// Verify the WAL+busy_timeout pragma path doesn't crash.
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	id, err := s.CreateProblemClass(context.Background(), "smoke", "smoke test")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	if id == 0 {
		t.Fatal("got id 0")
	}
	pc, err := s.GetProblemClass(context.Background(), id)
	if err != nil {
		t.Fatalf("GetProblemClass: %v", err)
	}
	if pc.Title != "smoke" {
		t.Errorf("title = %q, want smoke", pc.Title)
	}
}

func TestStore_CreateProblemClass_AndGetByTitle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	id, err := s.CreateProblemClass(ctx, "file-ownership", "Files owned by root after volume transfer")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == 0 {
		t.Fatal("id 0")
	}
	pc, err := s.GetProblemClassByTitle(ctx, "file-ownership")
	if err != nil {
		t.Fatalf("GetByTitle: %v", err)
	}
	if pc.ID != id {
		t.Errorf("id = %d, want %d", pc.ID, id)
	}
	if pc.Title != "file-ownership" {
		t.Errorf("title = %q", pc.Title)
	}
}

func TestStore_CreateProblemClass_Duplicate(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.CreateProblemClass(ctx, "dup", "first"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := s.CreateProblemClass(ctx, "dup", "second")
	if err != ErrDuplicate {
		t.Errorf("err = %v, want ErrDuplicate", err)
	}
}

func TestStore_UpsertProblemClass(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	_, created, err := s.UpsertProblemClass(ctx, "fresh", "first")
	if err != nil || !created {
		t.Fatalf("first upsert: created=%v err=%v", created, err)
	}
	_, created2, err := s.UpsertProblemClass(ctx, "fresh", "second")
	if err != nil || created2 {
		t.Errorf("second upsert: created=%v err=%v (want false, nil)", created2, err)
	}
}

func TestStore_CreateAnswerNode_WithParent(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	cid, err := s.CreateProblemClass(ctx, "docker-cp", "")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	a1, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "go-1.25", "fix v1", "tested v1", `{"sig":1}`)
	if err != nil {
		t.Fatalf("create v1: %v", err)
	}
	a2, err := s.CreateAnswerNode(ctx, cid, a1, "docker", "go", "go-1.26", "fix v2", "tested v2", `{"sig":2}`)
	if err != nil {
		t.Fatalf("create v2: %v", err)
	}

	got, err := s.GetAnswerNode(ctx, a2)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !got.ParentID.Valid || got.ParentID.Int64 != a1 {
		t.Errorf("parent_id = %v, want %d", got.ParentID, a1)
	}
	if got.Version != "go-1.26" {
		t.Errorf("version = %q", got.Version)
	}
}

func TestStore_ListAnswers_OrderedNewestFirst(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "test", "")
	for _, v := range []string{"v1", "v2", "v3"} {
		if _, err := s.CreateAnswerNode(ctx, cid, 0, "e", "l", v, "a", "b", "{}"); err != nil {
			t.Fatalf("CreateAnswerNode: %v", err)
		}
	}

	answers, err := s.ListAnswers(ctx, cid)
	if err != nil {
		t.Fatalf("ListAnswers: %v", err)
	}
	if len(answers) != 3 {
		t.Fatalf("len = %d, want 3", len(answers))
	}
	if answers[0].Version != "v3" {
		t.Errorf("newest first: got %q", answers[0].Version)
	}
}

func TestStore_UpdateAnswerStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "test", "")
	id, _ := s.CreateAnswerNode(ctx, cid, 0, "e", "l", "v", "a", "b", "{}")

	if err := s.UpdateAnswerStatus(ctx, id, AnswerVerified); err != nil {
		t.Fatalf("update: %v", err)
	}
	a, _ := s.GetAnswerNode(ctx, id)
	if a.Status != AnswerVerified {
		t.Errorf("status = %q", a.Status)
	}
	if err := s.UpdateAnswerStatus(ctx, id, "garbage"); err == nil {
		t.Error("invalid status accepted")
	}
}

func TestStore_CreateEdge_DuplicateAndSelf(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	c1, _ := s.CreateProblemClass(ctx, "a", "")
	c2, _ := s.CreateProblemClass(ctx, "b", "")

	if _, err := s.CreateEdge(ctx, c1, c2, EdgeSameRootCause, 1.0); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.CreateEdge(ctx, c1, c2, EdgeSameRootCause, 1.0); err != ErrDuplicate {
		t.Errorf("duplicate err = %v, want ErrDuplicate", err)
	}
	if _, err := s.CreateEdge(ctx, c1, c1, EdgePrerequisite, 1.0); err == nil {
		t.Error("self-edge accepted")
	}
	if _, err := s.CreateEdge(ctx, c1, c2, "invalid_relationship", 1.0); err == nil {
		t.Error("invalid relationship accepted")
	}
}

func TestStore_Discovery_ExactMatch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	if _, err := s.CreateProblemClass(ctx, "file-ownership", "files"); err != nil {
		t.Fatalf("create: %v", err)
	}
	cid := mustGetClassID(t, s, ctx, "file-ownership")
	aid, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "go-1.25", "use --chown", "verified v1", `{}`)
	if err != nil {
		t.Fatalf("answer: %v", err)
	}
	if err := s.UpdateAnswerStatus(ctx, aid, AnswerVerified); err != nil {
		t.Fatalf("status: %v", err)
	}

	res, err := s.Discovery(ctx, "file-ownership", "docker", "go", "go-1.25", false)
	if err != nil {
		t.Fatalf("Discovery: %v", err)
	}
	if res.Class.Title != "file-ownership" {
		t.Errorf("class = %q", res.Class.Title)
	}
	if res.Exact == nil {
		t.Fatal("no exact match")
	}
	if res.Exact.Solution != "use --chown" {
		t.Errorf("solution = %q", res.Exact.Solution)
	}
}

func TestStore_Discovery_WalksParentChain(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "test", "")
	a1, _ := s.CreateAnswerNode(ctx, cid, 0, "e", "l", "v1", "oldest", "", "{}")
	a2, _ := s.CreateAnswerNode(ctx, cid, a1, "e", "l", "v2", "middle", "", "{}")
	a3, _ := s.CreateAnswerNode(ctx, cid, a2, "e", "l", "v3", "newest", "", "{}")
	// Mark all verified so bestAnswer picks the latest.
	for _, id := range []int64{a1, a2, a3} {
		if err := s.UpdateAnswerStatus(ctx, id, AnswerVerified); err != nil {
			t.Fatalf("UpdateAnswerStatus: %v", err)
		}
	}

	res, err := s.Discovery(ctx, "test", "e", "l", "v3", false)
	if err != nil {
		t.Fatalf("Discovery: %v", err)
	}
	if res.Exact == nil || res.Exact.Solution != "newest" {
		t.Fatalf("exact = %v", res.Exact)
	}
	// Version history must contain the chain (oldest first).
	if len(res.Versions) != 3 {
		t.Fatalf("versions = %d, want 3", len(res.Versions))
	}
	if res.Versions[0].Solution != "oldest" || res.Versions[2].Solution != "newest" {
		t.Errorf("version order wrong: %v", res.Versions)
	}
}

func TestStore_Discovery_RelatedEdges(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	c1, _ := s.CreateProblemClass(ctx, "main", "")
	c2, _ := s.CreateProblemClass(ctx, "related-1", "")
	c3, _ := s.CreateProblemClass(ctx, "related-2", "")
	if _, err := s.CreateEdge(ctx, c1, c2, EdgeSameRootCause, 0.95); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	if _, err := s.CreateEdge(ctx, c1, c3, EdgePrerequisite, 0.7); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	if _, err := s.CreateAnswerNode(ctx, c1, 0, "e", "l", "v", "x", "", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	res, err := s.Discovery(ctx, "main", "e", "l", "v", true)
	if err != nil {
		t.Fatalf("Discovery: %v", err)
	}
	if len(res.Related) != 2 {
		t.Fatalf("related = %d, want 2", len(res.Related))
	}
	// First should be the higher-weight edge.
	if res.Related[0].TargetTitle != "related-1" || res.Related[0].Weight != 0.95 {
		t.Errorf("first related = %+v", res.Related[0])
	}
}

func TestStore_Discovery_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Discovery(context.Background(), "nonexistent", "", "", "", false)
	if err != ErrNotFound {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestNewID(t *testing.T) {
	a := NewID("sub")
	b := NewID("sub")
	if a == b {
		t.Error("NewID returned duplicates")
	}
	if !startsWith(a, "sub_") {
		t.Errorf("missing prefix: %q", a)
	}
}

// GetProblemClassStatus must derive the same best_status as the list
// query: ci_passed > verified > pending > failed, 'pending' when the
// class has no answers (OB-GAP-024).
func TestStore_GetProblemClassStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	pc, _, err := s.UpsertProblemClass(ctx, "status-class", "desc")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	check := func(want string) {
		t.Helper()
		got, err := s.GetProblemClassStatus(ctx, pc.ID)
		if err != nil {
			t.Fatalf("GetProblemClassStatus: %v", err)
		}
		if got != want {
			t.Errorf("status = %q, want %q", got, want)
		}
	}
	add := func(solution, status string) {
		t.Helper()
		id, err := s.CreateAnswerNode(ctx, pc.ID, 0, "docker", "go", "1.0", solution, "evidence: test", "{}")
		if err != nil {
			t.Fatalf("create answer: %v", err)
		}
		if err := s.UpdateAnswerStatus(ctx, id, status); err != nil {
			t.Fatalf("update status: %v", err)
		}
	}

	check(AnswerPending) // no answers → COALESCE to pending

	add("sol-failed", AnswerFailed)
	check(AnswerFailed)

	add("sol-pending", AnswerPending)
	check(AnswerPending) // pending beats failed

	add("sol-verified", AnswerVerified)
	check(AnswerVerified) // verified beats pending

	add("sol-ci", AnswerCIPassed)
	check(AnswerCIPassed) // ci_passed beats verified
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
