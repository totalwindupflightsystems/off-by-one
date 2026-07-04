package graph

import (
	"context"
	"testing"
)

// newSharedTestStore returns a Store backed by a uniquely-named in-memory
// SQLite DB. Unlike newTestStore (which uses Open(":memory:") → a single
// shared cache), this gives each test its own isolated DB to avoid state
// bleed between tests that populate the same tables.
func newSharedTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("OpenShared: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// --- OpenShared, DB, ApplyExtra ---

func TestStore_OpenShared(t *testing.T) {
	s, err := OpenShared(t.Name())
	if err != nil {
		t.Fatalf("OpenShared: %v", err)
	}
	defer s.Close()
	// Verify the DB is usable.
	ctx := context.Background()
	if _, err := s.CreateProblemClass(ctx, "shared-test", "desc"); err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
}

func TestStore_DB(t *testing.T) {
	s := newSharedTestStore(t)
	db := s.DB()
	if db == nil {
		t.Fatal("DB() returned nil")
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("db.Ping: %v", err)
	}
}

func TestStore_ApplyExtra(t *testing.T) {
	s := newSharedTestStore(t)
	extra := `CREATE TABLE IF NOT EXISTS extra_test (id INTEGER PRIMARY KEY)`
	if err := s.ApplyExtra(extra); err != nil {
		t.Fatalf("ApplyExtra: %v", err)
	}
	// Verify the table exists by inserting a row.
	if _, err := s.DB().Exec(`INSERT INTO extra_test (id) VALUES (1)`); err != nil {
		t.Fatalf("insert into extra_test: %v", err)
	}
	// Invalid SQL should return an error.
	if err := s.ApplyExtra(`THIS IS NOT SQL`); err == nil {
		t.Error("ApplyExtra with invalid SQL should return error")
	}
}

// --- ListProblemClasses ---

func TestStore_ListProblemClasses(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()
	// Create several problem classes.
	titles := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	for _, title := range titles {
		if _, err := s.CreateProblemClass(ctx, title, "desc-"+title); err != nil {
			t.Fatalf("CreateProblemClass(%q): %v", title, err)
		}
	}

	// List all — should return 5, ordered by id DESC.
	got, err := s.ListProblemClasses(ctx, 0, 0)
	if err != nil {
		t.Fatalf("ListProblemClasses: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	if got[0].Title != "epsilon" {
		t.Errorf("first = %q, want epsilon (id DESC)", got[0].Title)
	}

	// Pagination: limit=2 offset=0 → top 2.
	page, err := s.ListProblemClasses(ctx, 2, 0)
	if err != nil {
		t.Fatalf("ListProblemClasses(2,0): %v", err)
	}
	if len(page) != 2 {
		t.Fatalf("len = %d, want 2", len(page))
	}
	if page[0].Title != "epsilon" || page[1].Title != "delta" {
		t.Errorf("page = [%s, %s], want [epsilon, delta]", page[0].Title, page[1].Title)
	}

	// Pagination: limit=2 offset=2 → next 2.
	page2, err := s.ListProblemClasses(ctx, 2, 2)
	if err != nil {
		t.Fatalf("ListProblemClasses(2,2): %v", err)
	}
	if len(page2) != 2 {
		t.Fatalf("len = %d, want 2", len(page2))
	}
	if page2[0].Title != "gamma" || page2[1].Title != "beta" {
		t.Errorf("page2 = [%s, %s], want [gamma, beta]", page2[0].Title, page2[1].Title)
	}

	// Negative offset defaults to 0.
	page3, err := s.ListProblemClasses(ctx, 1, -1)
	if err != nil {
		t.Fatalf("ListProblemClasses(1,-1): %v", err)
	}
	if len(page3) != 1 || page3[0].Title != "epsilon" {
		t.Errorf("page3 = %v, want [epsilon]", page3)
	}

	// Empty store returns empty slice, not nil.
	empty, err := OpenShared(t.Name() + "-empty")
	if err != nil {
		t.Fatalf("OpenShared empty: %v", err)
	}
	defer empty.Close()
	classes, err := empty.ListProblemClasses(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListProblemClasses on empty: %v", err)
	}
	if len(classes) != 0 {
		t.Errorf("len = %d, want 0", len(classes))
	}
}

// --- ListEdgesFrom ---

func TestStore_ListEdgesFrom(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()
	id1, _ := s.CreateProblemClass(ctx, "source", "desc")
	id2, _ := s.CreateProblemClass(ctx, "target1", "desc")
	id3, _ := s.CreateProblemClass(ctx, "target2", "desc")

	// Create two edges from id1 with different weights.
	if _, err := s.CreateEdge(ctx, id1, id2, EdgeSameRootCause, 0.5); err != nil {
		t.Fatalf("CreateEdge 1: %v", err)
	}
	if _, err := s.CreateEdge(ctx, id1, id3, EdgeSameRootCause, 0.9); err != nil {
		t.Fatalf("CreateEdge 2: %v", err)
	}

	edges, err := s.ListEdgesFrom(ctx, id1)
	if err != nil {
		t.Fatalf("ListEdgesFrom: %v", err)
	}
	if len(edges) != 2 {
		t.Fatalf("len = %d, want 2", len(edges))
	}
	// Ordered by weight DESC — 0.9 first.
	if edges[0].Weight < edges[1].Weight {
		t.Error("edges not ordered by weight DESC")
	}

	// Non-existent source returns empty.
	edges2, err := s.ListEdgesFrom(ctx, 999)
	if err != nil {
		t.Fatalf("ListEdgesFrom(999): %v", err)
	}
	if len(edges2) != 0 {
		t.Errorf("len = %d, want 0", len(edges2))
	}
}

// --- RelatedTitles ---

func TestStore_RelatedTitles(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()
	id1, _ := s.CreateProblemClass(ctx, "main", "desc")
	id2, _ := s.CreateProblemClass(ctx, "related-1", "desc")
	id3, _ := s.CreateProblemClass(ctx, "related-2", "desc")

	if _, err := s.CreateEdge(ctx, id1, id2, EdgeSameRootCause, 0.5); err != nil {
		t.Fatalf("CreateEdge: %v", err)
	}
	if _, err := s.CreateEdge(ctx, id3, id1, EdgeSameRootCause, 0.5); err != nil {
		t.Fatalf("CreateEdge reverse: %v", err)
	}

	titles, err := s.RelatedTitles(ctx, id1)
	if err != nil {
		t.Fatalf("RelatedTitles: %v", err)
	}
	// Should include "related-1" (forward edge) and "related-2" (reverse edge).
	titleSet := make(map[string]bool)
	for _, t := range titles {
		titleSet[t] = true
	}
	if !titleSet["related-1"] {
		t.Error("missing related-1")
	}
	if !titleSet["related-2"] {
		t.Error("missing related-2")
	}

	// Non-existent source returns empty.
	empty, err := s.RelatedTitles(ctx, 999)
	if err != nil {
		t.Fatalf("RelatedTitles(999): %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("len = %d, want 0", len(empty))
	}
}

// --- Stats ---

func TestStore_Stats_Empty(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()
	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.TotalProblems != 0 || st.TotalAnswers != 0 || st.VerifiedAnswers != 0 {
		t.Errorf("empty stats = %+v, want all zeros", st)
	}
	if st.HitRate != 0 || st.Coverage != 0 {
		t.Errorf("empty ratios = hit=%.2f cov=%.2f, want 0", st.HitRate, st.Coverage)
	}
}

func TestStore_Stats_WithAnswers(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()
	id1, _ := s.CreateProblemClass(ctx, "class-a", "desc")
	id2, _ := s.CreateProblemClass(ctx, "class-b", "desc")

	// 3 answers: 1 pending, 1 verified, 1 failed.
	a1, err := s.CreateAnswerNode(ctx, id1, 0, "linux", "go", "1.0", "sol", "ev", "{}")
	if err != nil {
		t.Fatalf("CreateAnswerNode 1: %v", err)
	}
	a2, err := s.CreateAnswerNode(ctx, id1, 0, "linux", "go", "1.1", "sol", "ev", "{}")
	if err != nil {
		t.Fatalf("CreateAnswerNode 2: %v", err)
	}
	if _, err := s.CreateAnswerNode(ctx, id2, 0, "linux", "go", "1.0", "sol", "ev", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode 3: %v", err)
	}
	_ = a1
	if err := s.UpdateAnswerStatus(ctx, a2, "verified"); err != nil {
		t.Fatalf("UpdateAnswerStatus: %v", err)
	}

	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.TotalProblems != 2 {
		t.Errorf("TotalProblems = %d, want 2", st.TotalProblems)
	}
	if st.TotalAnswers != 3 {
		t.Errorf("TotalAnswers = %d, want 3", st.TotalAnswers)
	}
	if st.VerifiedAnswers != 1 {
		t.Errorf("VerifiedAnswers = %d, want 1", st.VerifiedAnswers)
	}
	// HitRate = 1/3 ≈ 0.333
	if st.HitRate < 0.33 || st.HitRate > 0.34 {
		t.Errorf("HitRate = %.4f, want ~0.333", st.HitRate)
	}
	// Coverage = 1/2 = 0.5
	if st.Coverage != 0.5 {
		t.Errorf("Coverage = %.4f, want 0.5", st.Coverage)
	}
}

// --- AnswerCount ---

func TestStore_AnswerCount(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()
	id1, _ := s.CreateProblemClass(ctx, "with-answers", "desc")
	id2, _ := s.CreateProblemClass(ctx, "no-answers", "desc")

	// Create 3 answers for class 1.
	if _, err := s.CreateAnswerNode(ctx, id1, 0, "linux", "go", "1.0", "sol", "ev", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode 1: %v", err)
	}
	if _, err := s.CreateAnswerNode(ctx, id1, 0, "linux", "go", "1.1", "sol", "ev", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode 2: %v", err)
	}
	if _, err := s.CreateAnswerNode(ctx, id1, 0, "linux", "go", "1.2", "sol", "ev", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode 3: %v", err)
	}

	n, err := s.AnswerCount(ctx, id1)
	if err != nil {
		t.Fatalf("AnswerCount: %v", err)
	}
	if n != 3 {
		t.Errorf("AnswerCount = %d, want 3", n)
	}

	// Class with no answers.
	n2, err := s.AnswerCount(ctx, id2)
	if err != nil {
		t.Fatalf("AnswerCount(id2): %v", err)
	}
	if n2 != 0 {
		t.Errorf("AnswerCount = %d, want 0", n2)
	}

	// Non-existent class.
	n3, err := s.AnswerCount(ctx, 999)
	if err != nil {
		t.Fatalf("AnswerCount(999): %v", err)
	}
	if n3 != 0 {
		t.Errorf("AnswerCount = %d, want 0", n3)
	}
}

// --- ListProblemClassesWithCounts ---

func TestStore_ListProblemClassesWithCounts(t *testing.T) {
	s := newSharedTestStore(t)
	ctx := context.Background()
	id1, err := s.CreateProblemClass(ctx, "class-with-answers", "desc")
	if err != nil {
		t.Fatalf("CreateProblemClass 1: %v", err)
	}
	if _, err := s.CreateProblemClass(ctx, "class-no-answers", "desc"); err != nil {
		t.Fatalf("CreateProblemClass 2: %v", err)
	}

	// Create 2 answers for class 1: one pending, one verified.
	a1, _ := s.CreateAnswerNode(ctx, id1, 0, "linux", "go", "1.0", "sol", "ev", "{}")
	a2, _ := s.CreateAnswerNode(ctx, id1, 0, "linux", "go", "1.1", "sol", "ev", "{}")
	if err := s.UpdateAnswerStatus(ctx, a1, "pending"); err != nil {
		t.Fatalf("UpdateAnswerStatus: %v", err)
	}
	if err := s.UpdateAnswerStatus(ctx, a2, "verified"); err != nil {
		t.Fatalf("UpdateAnswerStatus: %v", err)
	}

	result, err := s.ListProblemClassesWithCounts(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListProblemClassesWithCounts: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("len = %d, want 2", len(result))
	}

	// Ordered by id DESC — class-no-answers (id2) first.
	if result[0].Title != "class-no-answers" {
		t.Errorf("first = %q, want class-no-answers", result[0].Title)
	}
	if result[0].AnswerCount != 0 {
		t.Errorf("AnswerCount = %d, want 0", result[0].AnswerCount)
	}
	if result[0].Status != "pending" {
		t.Errorf("Status = %q, want pending (no answers → default)", result[0].Status)
	}

	// class-with-answers (id1) second.
	if result[1].Title != "class-with-answers" {
		t.Errorf("second = %q, want class-with-answers", result[1].Title)
	}
	if result[1].AnswerCount != 2 {
		t.Errorf("AnswerCount = %d, want 2", result[1].AnswerCount)
	}
	if result[1].Status != "verified" {
		t.Errorf("Status = %q, want verified (highest precedence)", result[1].Status)
	}

	// Pagination.
	page, err := s.ListProblemClassesWithCounts(ctx, 1, 0)
	if err != nil {
		t.Fatalf("ListProblemClassesWithCounts(1,0): %v", err)
	}
	if len(page) != 1 {
		t.Fatalf("len = %d, want 1", len(page))
	}
}
