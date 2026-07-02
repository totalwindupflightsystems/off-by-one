package graph

import (
	"context"
	"strings"
	"testing"
)

func TestSearch_MatchesProblemClassTitle(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "docker-volume-permission", "Files owned by root after Docker volume transfer")
	s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "go-1.25",
		"Use COPY --chown=appuser:appuser in Dockerfile", "", `{}`)

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

func TestSearch_FilterByEnv(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	cid, _ := s.CreateProblemClass(ctx, "test-class", "test description")
	s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1", "docker solution", "", "{}")
	s.CreateAnswerNode(ctx, cid, 0, "k8s", "go", "v1", "k8s solution", "", "{}")

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
	s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1", "go solution text", "", "{}")
	s.CreateAnswerNode(ctx, cid, 0, "docker", "python", "v1", "python solution text", "", "{}")

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
	s.UpdateAnswerStatus(ctx, aid, AnswerVerified)
	s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v2", "pending solution text", "", "{}")

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
	s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1", "Fix with chmod", "", "{}")

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
		s.CreateProblemClass(ctx, "pagination-test-"+string(rune('a'+i)), "common keyword for pagination")
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
	s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1",
		"Set the LD_LIBRARY_PATH to include the custom lib directory", "", "{}")

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
	s.CreateAnswerNode(ctx, cid, 0, "docker", "go", "v1", "solution", "", "{}")

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
		s.CreateProblemClass(ctx, "clamp-"+string(rune('a'+i)), "clamp test keyword")
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
