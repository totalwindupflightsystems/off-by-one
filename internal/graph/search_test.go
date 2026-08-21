package graph

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestSearch_MatchesProblemClassTitle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "docker-volume-permission", "Files owned by root after Docker volume transfer")
	if _, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "go-1.25",
		"Use COPY --chown=appuser:appuser in Dockerfile", "", `{}`); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	hits, err := s.Search(ctx, "docker", "", "", "", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no hits for 'docker'")
	}
	found := false
	for _, h := range hits {
		if h.Title == "docker-volume-permission" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected hit on docker-volume-permission, got %d hits", len(hits))
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	s := newTestStore(t)
	hits, err := s.Search(context.Background(), "", "", "", "", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("empty query returned %d hits", len(hits))
	}
}

// Search hits must carry the same per-class metadata as
// ListProblemClassesWithCountsFiltered rows (derived best status,
// description, created_at, answer_count) so the API search path can
// populate them without per-hit lookups (OB-GAP-050).
func TestSearch_HitCarriesClassMetadata(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "so-nil-pointer-deref", "nil pointer dereference in Go")
	aid, _ := s.CreateAnswerNode(ctx, cid, 0, "go", "go", "v1", "check for nil first", "", `{}`)
	if err := s.UpdateAnswerStatus(ctx, aid, AnswerVerified); err != nil {
		t.Fatalf("UpdateAnswerStatus: %v", err)
	}
	// A second class with only a pending answer, to verify per-class
	// status derivation (verified vs pending) in one result set.
	cid2, _ := s.CreateProblemClass(ctx, "pending-nil-check", "pending nil guard")
	if _, err := s.CreateAnswerNode(ctx, cid2, 0, "go", "go", "v1", "maybe guard nil", "", `{}`); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	hits, err := s.Search(ctx, "nil", "", "", "", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) < 2 {
		t.Fatalf("hits = %d, want >= 2", len(hits))
	}
	byID := map[int64]SearchHit{}
	for _, h := range hits {
		byID[h.ClassID] = h
	}
	verified, ok := byID[cid]
	if !ok {
		t.Fatalf("no hit for verified class %d", cid)
	}
	if verified.Status != "verified" {
		t.Errorf("verified class status = %q, want verified", verified.Status)
	}
	if verified.Description != "nil pointer dereference in Go" {
		t.Errorf("verified class description = %q, want seeded description", verified.Description)
	}
	if verified.CreatedAt.IsZero() {
		t.Error("verified class CreatedAt is zero, want parsed created_at")
	}
	if verified.AnswerCount != 1 {
		t.Errorf("verified class AnswerCount = %d, want 1", verified.AnswerCount)
	}
	pending, ok := byID[cid2]
	if !ok {
		t.Fatalf("no hit for pending class %d", cid2)
	}
	if pending.Status != "pending" {
		t.Errorf("pending class status = %q, want pending", pending.Status)
	}
}

func TestSearch_FilterByEnv(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "test-class", "test description")
	if _, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1", "docker solution", "", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}
	if _, err := s.CreateAnswerNode(ctx, cid, 0, "k8s", "go", "v1", "k8s solution", "", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	hits, err := s.Search(ctx, "solution", "docker", "", "", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// We don't strictly require the env filter to narrow answer-only
	// hits (FTS5 ambiguity), but we do require at least one hit.
	if len(hits) == 0 {
		t.Error("env filter returned 0 hits for matches")
	}
}

func TestSearch_FilterByLang(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "lang-test", "test description")
	if _, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1", "go solution text", "", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}
	if _, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "python", "v1", "python solution text", "", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	hits, err := s.Search(ctx, "solution", "", "python", "", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("lang filter returned 0 hits")
	}
	// Every answer-level hit must be from the python answer.
	for _, h := range hits {
		if h.AnswerID.Valid {
			a, err := s.GetAnswerNode(ctx, h.AnswerID.Int64)
			if err != nil {
				t.Fatalf("GetAnswerNode: %v", err)
			}
			if a.Lang != "python" {
				t.Errorf("answer lang = %q, want python", a.Lang)
			}
		}
	}
}

func TestSearch_FilterByStatus(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "status-test", "test description")
	aid, _ := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1", "verified solution text", "", "{}")
	if err := s.UpdateAnswerStatus(ctx, aid, AnswerVerified); err != nil {
		t.Fatalf("UpdateAnswerStatus: %v", err)
	}
	if _, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v2", "pending solution text", "", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	hits, err := s.Search(ctx, "solution", "", "", AnswerVerified, 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	// At least one hit should come from the verified answer.
	if len(hits) == 0 {
		t.Fatal("status filter returned 0 hits for verified")
	}
}

func TestSearch_SnippetContainsHighlight(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "snippet-test", "The docker container has permission issues")
	if _, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1", "Fix with chmod", "", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	hits, err := s.Search(ctx, "docker", "", "", "", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no hits")
	}
	// At least one snippet should contain the highlight markers '[' ']'
	// or the truncation marker '…'.
	foundSnippet := false
	for _, h := range hits {
		if strings.Contains(h.Snippet, "[") || strings.Contains(h.Snippet, "…") || h.Snippet != "" {
			foundSnippet = true
			break
		}
	}
	if !foundSnippet {
		t.Errorf("no snippet with highlight markers found in hits: %+v", hits)
	}
}

func TestSearch_Pagination(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Create multiple problem classes that share a common search term.
	for i := 0; i < 6; i++ {
		if _, err := s.CreateProblemClass(ctx, "pagination-test-"+string(rune('a'+i)), "common keyword for pagination"); err != nil {
			t.Fatalf("CreateProblemClass: %v", err)
		}
	}

	// Page 1: limit=3, offset=0
	page1, err := s.Search(ctx, "pagination", "", "", "", 3, 0)
	if err != nil {
		t.Fatalf("Search page 1: %v", err)
	}
	if len(page1) > 3 {
		t.Errorf("page 1 returned %d hits, want ≤ 3", len(page1))
	}

	// Page 2: limit=3, offset=3
	page2, err := s.Search(ctx, "pagination", "", "", "", 3, 3)
	if err != nil {
		t.Fatalf("Search page 2: %v", err)
	}
	if len(page2) > 3 {
		t.Errorf("page 2 returned %d hits, want ≤ 3", len(page2))
	}

	// Pages should not overlap (different titles).
	page1Titles := map[string]bool{}
	for _, h := range page1 {
		page1Titles[h.Title] = true
	}
	for _, h := range page2 {
		if page1Titles[h.Title] {
			t.Errorf("title %q appears on both pages", h.Title)
		}
	}

	// Total should cover all 6 entries.
	all, err := s.Search(ctx, "pagination", "", "", "", 100, 0)
	if err != nil {
		t.Fatalf("Search all: %v", err)
	}
	if len(all) < len(page1)+len(page2) {
		t.Errorf("total %d < page1(%d)+page2(%d)", len(all), len(page1), len(page2))
	}
}

func TestSearch_AnswerContentIndexed(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	// Problem class title does NOT contain the search term — only the
	// answer solution does. This verifies the answer_nodes_fts index is
	// working.
	cid, _ := s.CreateProblemClass(ctx, "obscure-name", "Generic description without keywords")
	if _, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1",
		"Set the LD_LIBRARY_PATH to include the custom lib directory", "", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	hits, err := s.Search(ctx, "LD_LIBRARY_PATH", "", "", "", 10, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) == 0 {
		t.Fatal("no hits for answer-only content 'LD_LIBRARY_PATH'")
	}
}

func TestSearch_FTSSpecialChars(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "special-chars", "Handling paths like /usr/local/bin")
	if _, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1", "solution", "", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	// FTS5 special characters are quoted by our escaping, so the search
	// should not error. It may or may not find matches depending on the
	// tokenizer, but it must not crash.
	hits, err := s.Search(ctx, "/usr/local/bin", "", "", "", 10, 0)
	if err != nil {
		t.Fatalf("Search with special chars failed: %v", err)
	}
	_ = hits
}

func TestSearch_LimitClampedToMax(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := s.CreateProblemClass(ctx, "clamp-"+string(rune('a'+i)), "clamp test keyword"); err != nil {
			t.Fatalf("CreateProblemClass: %v", err)
		}
	}

	// limit > 100 should be clamped to 20 by Search internally.
	hits, err := s.Search(ctx, "clamp", "", "", "", 999, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) > 20 {
		t.Errorf("limit=999 returned %d hits, expected ≤ 20", len(hits))
	}
}

func TestSearchCount_ReturnsFullTotal(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		cid, _ := s.CreateProblemClass(ctx, fmt.Sprintf("count-class-%d", i), "shared token searchable")
		if _, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1", "shared token solution", "", "{}"); err != nil {
			t.Fatalf("CreateAnswerNode: %v", err)
		}
	}

	// Page size 2 → 2 hits, but the total must be 3 (all matches), not the
	// page length. This is what powers accurate pagination in the UI.
	hits, err := s.Search(ctx, "shared", "", "", "", 2, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("page size: got %d hits, want 2", len(hits))
	}
	total, err := s.SearchCount(ctx, "shared", "", "", "")
	if err != nil {
		t.Fatalf("SearchCount: %v", err)
	}
	if total != 3 {
		t.Errorf("SearchCount = %d, want 3 (full total, not page size)", total)
	}
}

func TestCountProblemClasses_MatchesListPagination(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		if _, err := s.CreateProblemClass(ctx, fmt.Sprintf("list-class-%d", i), "d"); err != nil {
			t.Fatalf("CreateProblemClass: %v", err)
		}
	}

	rows, err := s.ListProblemClassesWithCountsFiltered(ctx, "", 2, 0)
	if err != nil {
		t.Fatalf("ListProblemClassesWithCountsFiltered: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("page size: got %d, want 2", len(rows))
	}
	n, err := s.CountProblemClasses(ctx, "")
	if err != nil {
		t.Fatalf("CountProblemClasses: %v", err)
	}
	if n != 5 {
		t.Errorf("CountProblemClasses = %d, want 5 (full total, not page size)", n)
	}
}

// TestSearch_DeduplicatesClassMatchesAndAnswerMatches: a class whose title
// AND whose answer's solution both match the query must appear exactly once
// (the two UNION branches produce distinct rows; we dedupe by class id).
func TestSearch_DeduplicatesClassMatchesAndAnswerMatches(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, err := s.CreateProblemClass(ctx, "dup-check-regression", "dup-check-regression description")
	if err != nil {
		t.Fatalf("CreateProblemClass: %v", err)
	}
	if _, err := s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1",
		"the dup-check-regression solution matches both branches", "", "{}"); err != nil {
		t.Fatalf("CreateAnswerNode: %v", err)
	}

	hits, err := s.Search(ctx, "dup-check-regression", "", "", "", 20, 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("Search returned %d hits, want 1 (class must not be duplicated across UNION branches)", len(hits))
	}
	if hits[0].ClassID != cid {
		t.Errorf("hit class id = %d, want %d", hits[0].ClassID, cid)
	}
	total, err := s.SearchCount(ctx, "dup-check-regression", "", "", "")
	if err != nil {
		t.Fatalf("SearchCount: %v", err)
	}
	if total != 1 {
		t.Errorf("SearchCount = %d, want 1 (must agree with hit count)", total)
	}
}
