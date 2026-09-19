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
	cid, _ := s.CreateProblemClass(ctx, "parent-chain", "")
	a1, _ := s.CreateAnswerNode(ctx, cid, 0, "e", "l", "v1", "oldest", "", "{}")
	a2, _ := s.CreateAnswerNode(ctx, cid, a1, "e", "l", "v2", "middle", "", "{}")
	a3, _ := s.CreateAnswerNode(ctx, cid, a2, "e", "l", "v3", "newest", "", "{}")
	// Mark all verified so bestAnswer picks the latest.
	for _, id := range []int64{a1, a2, a3} {
		if err := s.UpdateAnswerStatus(ctx, id, AnswerVerified); err != nil {
			t.Fatalf("UpdateAnswerStatus: %v", err)
		}
	}

	res, err := s.Discovery(ctx, "parent-chain", "e", "l", "v3", false)
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

// TestStore_Stats_ExcludesFailedSignature guards OB-GAP-060: answer_nodes
// whose status is 'verified' but whose signatures JSON carries
// result='failed' must not count toward VerifiedAnswers (and therefore
// must depress HitRate below 1.0). The pre-fix query counted every
// status-verified row, so this test fails against the old aggregation.
func TestStore_Stats_ExcludesFailedSignature(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()

	pc, _, err := s.UpsertProblemClass(ctx, "class-a", "desc")
	if err != nil {
		t.Fatalf("UpsertProblemClass: %v", err)
	}
	seed := func(sigs string) {
		t.Helper()
		id, err := s.CreateAnswerNode(ctx, pc.ID, 0, "linux", "go", "1.0", "sol", "ev", sigs)
		if err != nil {
			t.Fatalf("CreateAnswerNode: %v", err)
		}
		if err := s.UpdateAnswerStatus(ctx, id, AnswerVerified); err != nil {
			t.Fatalf("UpdateAnswerStatus: %v", err)
		}
	}

	// Two verified answers with passing signatures, one verified answer
	// whose signature reports failure (the OB-GAP-060 divergence).
	seed(`{"result":"passed","model":"m1"}`)
	seed(`{"result":"passed","model":"m1"}`)
	seed(`{"result":"failed","model":"m2"}`)

	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.TotalAnswers != 3 {
		t.Errorf("TotalAnswers = %d, want 3", st.TotalAnswers)
	}
	if st.VerifiedAnswers != 2 {
		t.Errorf("VerifiedAnswers = %d, want 2 (failed-signature node excluded)", st.VerifiedAnswers)
	}
	if got, want := st.HitRate, 2.0/3.0; got < want-0.01 || got > want+0.01 {
		t.Errorf("HitRate = %.3f, want ≈%.3f (must be < 1.0 while failed-verified rows exist)", got, want)
	}
}

// TestStore_Discovery_ExcludesFailedSignature guards OB-GAP-057 at the
// query level: a row whose status says verified but whose signature says
// result='failed' must not be served by Discovery, even though status is
// the primary signal — an older binary could have written that pair (the
// live graph holds 28 such rows). A normal verified sibling is still
// returned.
func TestStore_Discovery_ExcludesFailedSignature(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()

	seed := func(classID int64, sigs, solution string) {
		t.Helper()
		id, err := s.CreateAnswerNode(ctx, classID, 0, "docker", "go", "1.0", solution, "ev", sigs)
		if err != nil {
			t.Fatalf("CreateAnswerNode: %v", err)
		}
		if err := s.UpdateAnswerStatus(ctx, id, AnswerVerified); err != nil {
			t.Fatalf("UpdateAnswerStatus: %v", err)
		}
	}

	// Class with one good answer and one failed-signature row that claims
	// to be verified.
	mixed, _, err := s.UpsertProblemClass(ctx, "mixed-class", "desc")
	if err != nil {
		t.Fatalf("UpsertProblemClass (mixed): %v", err)
	}
	seed(mixed.ID, `{"result":"passed"}`, "good answer")
	seed(mixed.ID, `{"result":"failed"}`, "gave up")

	res, err := s.Discovery(ctx, "mixed-class", "", "", "", false)
	if err != nil {
		t.Fatalf("Discovery (mixed): %v", err)
	}
	if res.Exact == nil {
		t.Fatal("Discovery (mixed): no exact answer, want the verified sibling")
	}
	if res.Exact.Solution != "good answer" {
		t.Errorf("Discovery (mixed) served %q, want the passing answer", res.Exact.Solution)
	}

	// Class whose only answer is the failed-signature row: discovery must
	// report no exact answer instead of serving it (the live
	// go-board-audit-idle-tick case).
	onlyFailed, _, err := s.UpsertProblemClass(ctx, "only-failed-class", "desc")
	if err != nil {
		t.Fatalf("UpsertProblemClass (only-failed): %v", err)
	}
	seed(onlyFailed.ID, `{"result":"failed"}`, "gave up")

	res, err = s.Discovery(ctx, "only-failed-class", "", "", "", false)
	if err != nil {
		t.Fatalf("Discovery (only-failed): %v", err)
	}
	if res.Exact != nil {
		t.Errorf("Discovery served a failed-signature answer: %+v", res.Exact)
	}
}

// --- OB-GAP-064: failed-signature answers in version history + status ---

// seedAnswerWithStatus inserts an answer with the given signature blob and
// forces its status column. It is the fixture for the status/signature
// divergence OB-GAP-064 targets: status says verified/ci_passed while the
// signature JSON says result='failed'.
func seedAnswerWithStatus(t *testing.T, s *Store, ctx context.Context, classID, parentID int64, solution, signatures, status string) int64 {
	t.Helper()
	id, err := s.CreateAnswerNode(ctx, classID, parentID, "docker", "go", "1.0", solution, "evidence", signatures)
	if err != nil {
		t.Fatalf("CreateAnswerNode(%q): %v", solution, err)
	}
	if err := s.UpdateAnswerStatus(ctx, id, status); err != nil {
		t.Fatalf("UpdateAnswerStatus(%d, %q): %v", id, status, err)
	}
	return id
}

// TestStore_Discovery_VersionHistory_ExcludesFailedSignature guards
// OB-GAP-064 at the Discovery surface: a row in the MIDDLE of a parent_id
// chain whose status says verified but whose signature says
// result='failed' is not a version of the answer and must be omitted from
// DiscoveryResult.Versions — while the good ancestors on BOTH sides are
// still returned, i.e. the walk continues past the skipped row instead of
// stopping at it.
func TestStore_Discovery_VersionHistory_ExcludesFailedSignature(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()

	cid, err := s.CreateProblemClass(ctx, "chain-failed-middle", "three-link chain with a failed middle")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	r1 := seedAnswerWithStatus(t, s, ctx, cid, 0, "oldest good root", `{"result":"ok"}`, AnswerVerified)
	r2 := seedAnswerWithStatus(t, s, ctx, cid, r1, "failed middle link", `{"result":"failed"}`, AnswerVerified)
	r3 := seedAnswerWithStatus(t, s, ctx, cid, r2, "newest good tip", `{"result":"ok"}`, AnswerVerified)

	res, err := s.Discovery(ctx, "chain-failed-middle", "", "", "", false)
	if err != nil {
		t.Fatalf("Discovery: %v", err)
	}
	if res.Exact == nil || res.Exact.ID != r3 {
		t.Fatalf("Exact = %+v, want the newest good answer id %d", res.Exact, r3)
	}

	solutions := make([]string, 0, len(res.Versions))
	for _, v := range res.Versions {
		solutions = append(solutions, v.Solution)
	}
	for _, v := range res.Versions {
		if v.ID == r2 || v.Solution == "failed middle link" {
			t.Errorf("Versions contains failed-signature row: %v (id %d, want it omitted)", solutions, v.ID)
		}
	}
	if len(res.Versions) != 2 {
		t.Fatalf("Versions = %v (%d rows), want 2 (failed-signature r2 excluded, surviving count down by exactly one)", solutions, len(res.Versions))
	}
	// The ancestor BEHIND the failed row must still be present: the skip
	// continues the walk, it does not truncate it.
	if res.Versions[0].ID != r1 {
		t.Errorf("Versions[0] id = %d, want %d (good ancestor behind the failed row)", res.Versions[0].ID, r1)
	}
	// Ordering contract unchanged: oldest → newest.
	if res.Versions[0].Solution != "oldest good root" || res.Versions[len(res.Versions)-1].Solution != "newest good tip" {
		t.Errorf("Versions order = %v, want oldest → newest with the failed link removed", solutions)
	}
}

// listStatusesByTitle runs the filtered list query and returns derived
// statuses keyed by class title.
func listStatusesByTitle(t *testing.T, s *Store, ctx context.Context, status string) map[string]string {
	t.Helper()
	rows, err := s.ListProblemClassesWithCountsFiltered(ctx, status, "", "", 100, 0)
	if err != nil {
		t.Fatalf("ListProblemClassesWithCountsFiltered(%q): %v", status, err)
	}
	out := make(map[string]string, len(rows))
	for _, r := range rows {
		out[r.Title] = r.Status
	}
	return out
}

// TestStore_ListProblemClassesWithCounts_ExcludesFailedSignatureStatus
// guards OB-GAP-064 at the list surface: a class whose only status-verified
// answer carries a failed signature must derive 'failed' — never
// 'verified' — so ?status=solved stops listing a failed solve as solved. A
// companion class with a genuine verified answer still derives 'verified'
// (guard against over-blocking), and the ci_passed branch is held to the
// same rule.
func TestStore_ListProblemClassesWithCounts_ExcludesFailedSignatureStatus(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()

	failedVerified, err := s.CreateProblemClass(ctx, "status-failed-signature", "verified row with a failed signature")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	seedAnswerWithStatus(t, s, ctx, failedVerified, 0, "gave up", `{"result":"failed"}`, AnswerVerified)

	failedCIPassed, err := s.CreateProblemClass(ctx, "status-failed-signature-ci", "ci_passed row with a failed signature")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	seedAnswerWithStatus(t, s, ctx, failedCIPassed, 0, "gave up under CI", `{"result":"failed"}`, AnswerCIPassed)

	genuine, err := s.CreateProblemClass(ctx, "status-genuine-verified", "genuine verified row")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	seedAnswerWithStatus(t, s, ctx, genuine, 0, "real fix", `{"result":"passed"}`, AnswerVerified)

	all := listStatusesByTitle(t, s, ctx, "")
	if got := all["status-failed-signature"]; got != "failed" {
		t.Errorf("status = %q, want %q for a class whose only verified row has a failed signature", got, "failed")
	}
	if got := all["status-failed-signature-ci"]; got != "failed" {
		t.Errorf("status = %q, want %q for a class whose only ci_passed row has a failed signature", got, "failed")
	}
	if got := all["status-genuine-verified"]; got != "verified" {
		t.Errorf("companion status = %q, want %q (a genuine verified row must still derive verified)", got, "verified")
	}

	solved := listStatusesByTitle(t, s, ctx, "solved")
	if st, ok := solved["status-failed-signature"]; ok {
		t.Errorf("?status=solved lists status-failed-signature with status %q, want it excluded", st)
	}
	if st, ok := solved["status-failed-signature-ci"]; ok {
		t.Errorf("?status=solved lists status-failed-signature-ci with status %q, want it excluded", st)
	}
	if got := solved["status-genuine-verified"]; got != "verified" {
		t.Errorf("?status=solved companion = %q, want verified", got)
	}

	// answer_count intentionally still counts every row (out of scope for
	// OB-GAP-064): only the DERIVED status changes.
	for _, r := range mustList(t, s, ctx) {
		if r.Title == "status-failed-signature" && r.AnswerCount != 1 {
			t.Errorf("answer_count = %d, want 1 (counts every row by design)", r.AnswerCount)
		}
	}
}

func mustList(t *testing.T, s *Store, ctx context.Context) []ProblemClassWithCounts {
	t.Helper()
	rows, err := s.ListProblemClassesWithCountsFiltered(ctx, "", "", "", 100, 0)
	if err != nil {
		t.Fatalf("ListProblemClassesWithCountsFiltered: %v", err)
	}
	return rows
}

// TestStore_CountProblemClasses_ExcludesFailedSignatureStatus guards
// OB-GAP-064 at the pagination-total surface: ?status=solved must not count
// a class whose only verified row has a failed signature, the class must
// count as 'failed' instead, and the unfiltered total is unchanged.
func TestStore_CountProblemClasses_ExcludesFailedSignatureStatus(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()

	failedVerified, err := s.CreateProblemClass(ctx, "count-failed-signature", "verified row with a failed signature")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	seedAnswerWithStatus(t, s, ctx, failedVerified, 0, "gave up", `{"result":"failed"}`, AnswerVerified)

	genuine, err := s.CreateProblemClass(ctx, "count-genuine-verified", "genuine verified row")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	seedAnswerWithStatus(t, s, ctx, genuine, 0, "real fix", `{"result":"passed"}`, AnswerVerified)

	for _, tc := range []struct {
		status string
		want   int
		why    string
	}{
		{"solved", 1, "only the genuine verified class is solved"},
		{"verified", 1, "the failed-signature row must not satisfy the verified branch"},
		{"ci_passed", 0, "no class has a legitimate ci_passed answer"},
		{"failed", 1, "the failed-signature class derives failed instead"},
		{"pending", 0, "neither class is pending"},
		{"", 2, "the unfiltered total is unchanged"},
	} {
		got, err := s.CountProblemClasses(ctx, tc.status, "", "")
		if err != nil {
			t.Fatalf("CountProblemClasses(%q): %v", tc.status, err)
		}
		if got != tc.want {
			t.Errorf("CountProblemClasses(%q) = %d, want %d (%s)", tc.status, got, tc.want, tc.why)
		}
	}
}

// TestStore_GetProblemClassStatus_ExcludesFailedSignature pins the detail
// endpoint's derivation to the same rule as the list view (OB-GAP-024
// parity). GetProblemClassStatus is a fourth best_status CASE site that
// the OB-GAP-064 brief did not enumerate; left unfixed it would report
// 'verified' for a class the list view reports as 'failed'. A genuine
// verified row still derives verified.
func TestStore_GetProblemClassStatus_ExcludesFailedSignature(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()

	failedVerified, err := s.CreateProblemClass(ctx, "detail-failed-signature", "verified row with a failed signature")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	seedAnswerWithStatus(t, s, ctx, failedVerified, 0, "gave up", `{"result":"failed"}`, AnswerVerified)

	genuine, err := s.CreateProblemClass(ctx, "detail-genuine-verified", "genuine verified row")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	seedAnswerWithStatus(t, s, ctx, genuine, 0, "real fix", `{"result":"passed"}`, AnswerVerified)

	for _, tc := range []struct {
		class string
		want  string
	}{
		{"detail-failed-signature", "failed"},
		{"detail-genuine-verified", "verified"},
	} {
		got, err := s.GetProblemClassStatus(ctx, mustGetClassID(t, s, ctx, tc.class))
		if err != nil {
			t.Fatalf("GetProblemClassStatus(%q): %v", tc.class, err)
		}
		if got != tc.want {
			t.Errorf("GetProblemClassStatus(%q) = %q, want %q", tc.class, got, tc.want)
		}
		// The detail status must equal what the list view derives for the
		// same class (OB-GAP-024 parity).
		if listed := listStatusesByTitle(t, s, ctx, "")[tc.class]; listed != got {
			t.Errorf("detail status %q != list status %q for %q (OB-GAP-024 parity)", got, listed, tc.class)
		}
	}
}

// seedAnswerEnvLang inserts an answer with an explicit env/lang pair, the
// fixture for the OB-GAP-080 exact-match env/lang list filters.
func seedAnswerEnvLang(t *testing.T, s *Store, ctx context.Context, classID int64, env, lang, status string) int64 {
	t.Helper()
	id, err := s.CreateAnswerNode(ctx, classID, 0, env, lang, "1.0", "solution for "+env+"/"+lang, "evidence", `{"result":"passed"}`)
	if err != nil {
		t.Fatalf("CreateAnswerNode(env=%q, lang=%q): %v", env, lang, err)
	}
	if err := s.UpdateAnswerStatus(ctx, id, status); err != nil {
		t.Fatalf("UpdateAnswerStatus(%d, %q): %v", id, status, err)
	}
	return id
}

// TestStore_ListProblemClassesWithCountsFiltered_EnvLang guards OB-GAP-080
// at the store surface: ListProblemClassesWithCountsFiltered and
// CountProblemClasses must apply env/lang as exact-match filters with the
// same semantics as Search — empty string = no filter, a class matches a
// value when at least one of its answer rows carries it, and both filters
// intersect. A nonsense value must yield zero rows and a zero count while
// env="" returns everything.
func TestStore_ListProblemClassesWithCountsFiltered_EnvLang(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()

	dockerGo, err := s.CreateProblemClass(ctx, "envlang-docker-go", "docker + go class")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	seedAnswerEnvLang(t, s, ctx, dockerGo, "docker", "go", AnswerVerified)

	kubePy, err := s.CreateProblemClass(ctx, "envlang-kube-python", "kubernetes + python class")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	seedAnswerEnvLang(t, s, ctx, kubePy, "kubernetes", "python", AnswerPending)

	// A class with NO answers: it is part of the unfiltered catalog but
	// must not match any env/lang value (EXISTS finds no row).
	if _, err := s.CreateProblemClass(ctx, "envlang-no-answers", "class without answers"); err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}

	titles := func(rows []ProblemClassWithCounts) []string {
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			out = append(out, r.Title)
		}
		return out
	}
	count := func(status, env, lang string) int {
		t.Helper()
		n, err := s.CountProblemClasses(ctx, status, env, lang)
		if err != nil {
			t.Fatalf("CountProblemClasses(%q, %q, %q): %v", status, env, lang, err)
		}
		return n
	}
	list := func(status, env, lang string) []ProblemClassWithCounts {
		t.Helper()
		rows, err := s.ListProblemClassesWithCountsFiltered(ctx, status, env, lang, 100, 0)
		if err != nil {
			t.Fatalf("ListProblemClassesWithCountsFiltered(%q, %q, %q): %v", status, env, lang, err)
		}
		return rows
	}

	// Empty filters return the whole catalog (3 classes).
	if got := titles(list("", "", "")); len(got) != 3 {
		t.Errorf("no filters: titles = %v, want all 3 classes", got)
	}
	if n := count("", "", ""); n != 3 {
		t.Errorf("no filters: count = %d, want 3", n)
	}

	// Nonsense env returns zero rows and a zero total.
	if got := titles(list("", "zzz-no-such-env-xyz", "")); len(got) != 0 {
		t.Errorf("env=<nonsense>: titles = %v, want none", got)
	}
	if n := count("", "zzz-no-such-env-xyz", ""); n != 0 {
		t.Errorf("env=<nonsense>: count = %d, want 0", n)
	}

	// Nonsense lang likewise.
	if got := titles(list("", "", "zzz-no-such-lang-xyz")); len(got) != 0 {
		t.Errorf("lang=<nonsense>: titles = %v, want none", got)
	}
	if n := count("", "", "zzz-no-such-lang-xyz"); n != 0 {
		t.Errorf("lang=<nonsense>: count = %d, want 0", n)
	}

	// Real env returns exactly the matching class.
	if got := titles(list("", "docker", "")); len(got) != 1 || got[0] != "envlang-docker-go" {
		t.Errorf("env=docker: titles = %v, want [envlang-docker-go]", got)
	}
	if n := count("", "docker", ""); n != 1 {
		t.Errorf("env=docker: count = %d, want 1", n)
	}

	// Real lang returns exactly the matching class.
	if got := titles(list("", "", "python")); len(got) != 1 || got[0] != "envlang-kube-python" {
		t.Errorf("lang=python: titles = %v, want [envlang-kube-python]", got)
	}
	if n := count("", "", "python"); n != 1 {
		t.Errorf("lang=python: count = %d, want 1", n)
	}

	// env and lang intersect: no class has docker+python.
	if got := titles(list("", "docker", "python")); len(got) != 0 {
		t.Errorf("env=docker&lang=python: titles = %v, want none (filters intersect)", got)
	}
	if n := count("", "docker", "python"); n != 0 {
		t.Errorf("env=docker&lang=python: count = %d, want 0", n)
	}

	// A matching pair (same class carries both) still matches.
	if got := titles(list("", "kubernetes", "python")); len(got) != 1 || got[0] != "envlang-kube-python" {
		t.Errorf("env=kubernetes&lang=python: titles = %v, want [envlang-kube-python]", got)
	}

	// env/lang compose with the status filter: env=docker&status=pending
	// excludes the verified docker class.
	if got := titles(list("pending", "docker", "")); len(got) != 0 {
		t.Errorf("status=pending&env=docker: titles = %v, want none", got)
	}
	if got := titles(list("pending", "kubernetes", "")); len(got) != 1 || got[0] != "envlang-kube-python" {
		t.Errorf("status=pending&env=kubernetes: titles = %v, want [envlang-kube-python]", got)
	}
}
