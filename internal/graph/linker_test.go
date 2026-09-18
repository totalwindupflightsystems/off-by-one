package graph

import (
	"context"
	"math"
	"path/filepath"
	"testing"
)

// newLinkerTestStore returns a Store backed by its own temp-file database.
// The package's shared in-memory helper is deliberately avoided here: the
// linker scans EVERY problem class, so a database shared with other tests in
// the package would make the ranked candidate list depend on test order.
func newLinkerTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "linker-test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func mustCreateClass(t *testing.T, s *Store, ctx context.Context, title, description string) int64 {
	t.Helper()
	id, err := s.CreateProblemClass(ctx, title, description)
	if err != nil {
		t.Fatalf("CreateProblemClass(%q): %v", title, err)
	}
	if id == 0 {
		t.Fatalf("CreateProblemClass(%q) returned id 0", title)
	}
	return id
}

func countEdges(t *testing.T, s *Store, ctx context.Context) int {
	t.Helper()
	var n int
	if err := s.DB().QueryRowContext(ctx, `SELECT count(*) FROM problem_edges`).Scan(&n); err != nil {
		t.Fatalf("count problem_edges: %v", err)
	}
	return n
}

// TestLinkSimilarClassesCreatesEdges proves the linker is a real edge
// producer (OB-GAP-067): the `similar` relationship is accepted by
// CreateEdge, edges are written in BOTH directions with a weight inside the
// documented (0,1] range, an unrelated class is left alone, and a second run
// creates zero new rows.
func TestLinkSimilarClassesCreatesEdges(t *testing.T) {
	ctx := context.Background()
	s := newLinkerTestStore(t)

	// python-sdk-idle-audit      → {python, sdk, idle, audit}
	// python-audit-idle-maintenance → {python, audit, idle, maintenance}
	//   → 3 shared lexemes out of the smaller set's 4 → weight 0.75
	aID := mustCreateClass(t, s, ctx, "python-sdk-idle-audit", "audit a python SDK for idle waiting")
	bID := mustCreateClass(t, s, ctx, "python-audit-idle-maintenance", "maintenance pass for the python idle audit")
	// Shares no lexeme with either of the above.
	cID := mustCreateClass(t, s, ctx, "rust-ownership-lifetimes", "borrow checker lifetimes")

	created, err := s.LinkSimilarClasses(ctx, aID, 2, 5)
	if err != nil {
		t.Fatalf("LinkSimilarClasses: %v", err)
	}
	if created != 2 {
		t.Errorf("created = %d, want 2 (one edge in each direction between the pair)", created)
	}

	edges, err := s.ListEdgesFrom(ctx, aID)
	if err != nil {
		t.Fatalf("ListEdgesFrom(%d): %v", aID, err)
	}
	if len(edges) != 1 {
		t.Fatalf("ListEdgesFrom(a) returned %d edges, want 1", len(edges))
	}
	e := edges[0]
	if e.Relationship != EdgeSimilar {
		t.Errorf("relationship = %q, want %q", e.Relationship, EdgeSimilar)
	}
	if e.SourceID != aID || e.TargetID != bID {
		t.Errorf("edge = %d->%d, want %d->%d", e.SourceID, e.TargetID, aID, bID)
	}
	if !(e.Weight > 0 && e.Weight <= 1) {
		t.Errorf("weight = %v, want 0 < w <= 1", e.Weight)
	}
	if want := 0.75; math.Abs(e.Weight-want) > 1e-9 {
		t.Errorf("weight = %v, want %v (3 of the smaller title's 4 lexemes)", e.Weight, want)
	}

	// The reverse direction must exist too — every traversal read in the
	// codebase (ListEdgesFrom, relatedClasses, the /related handler) walks
	// one direction only, so a one-sided write would leave
	// /problems/python-audit-idle-maintenance/related empty.
	rev, err := s.ListEdgesFrom(ctx, bID)
	if err != nil {
		t.Fatalf("ListEdgesFrom(%d): %v", bID, err)
	}
	if len(rev) != 1 {
		t.Fatalf("ListEdgesFrom(b) returned %d edges, want 1 (bidirectional link)", len(rev))
	}
	if rev[0].TargetID != aID || rev[0].Relationship != EdgeSimilar {
		t.Errorf("reverse edge = %d (%s), want ->%d (%s)", rev[0].TargetID, rev[0].Relationship, aID, EdgeSimilar)
	}

	// The unrelated class must not have been linked.
	if unrelated, err := s.ListEdgesFrom(ctx, cID); err != nil {
		t.Fatalf("ListEdgesFrom(%d): %v", cID, err)
	} else if len(unrelated) != 0 {
		t.Errorf("unrelated class has %d edges, want 0", len(unrelated))
	}

	// Idempotency: re-running creates nothing and does not error.
	again, err := s.LinkSimilarClasses(ctx, aID, 2, 5)
	if err != nil {
		t.Fatalf("second LinkSimilarClasses: %v", err)
	}
	if again != 0 {
		t.Errorf("second run created = %d, want 0", again)
	}
	if n := countEdges(t, s, ctx); n != 2 {
		t.Errorf("problem_edges rows = %d, want 2 after the idempotent re-run", n)
	}

	// LinkAllSimilar must be re-runnable across the whole catalog without
	// error and without adding rows.
	total, err := s.LinkAllSimilar(ctx, 2, 5)
	if err != nil {
		t.Fatalf("LinkAllSimilar: %v", err)
	}
	if total != 0 {
		t.Errorf("LinkAllSimilar on an already-linked DB created = %d, want 0", total)
	}
	if n := countEdges(t, s, ctx); n != 2 {
		t.Errorf("problem_edges rows = %d after LinkAllSimilar, want 2", n)
	}
}

// TestLinkSimilarClassesRespectsMinShared proves minShared is the gate and
// not an accident: the same pair that links at minShared=1 is NOT linked at
// minShared=2, and the class is left with zero edges.
func TestLinkSimilarClassesRespectsMinShared(t *testing.T) {
	ctx := context.Background()
	s := newLinkerTestStore(t)

	// Tokens (tokens shorter than 3 chars are dropped, so "go" is gone):
	//   go-nil-pointer-deref   → {nil, pointer, deref}
	//   go-nil-interface-grace → {nil, interface, grace}
	// Shared lexemes: {nil} = 1, below the default minShared of 2.
	aID := mustCreateClass(t, s, ctx, "go-nil-pointer-deref", "nil pointer dereference")
	mustCreateClass(t, s, ctx, "go-nil-interface-grace", "nil interface handling")

	created, err := s.LinkSimilarClasses(ctx, aID, 2, 5)
	if err != nil {
		t.Fatalf("LinkSimilarClasses(minShared=2): %v", err)
	}
	if created != 0 {
		t.Errorf("created = %d at minShared=2, want 0 (only 1 shared lexeme)", created)
	}
	if edges, err := s.ListEdgesFrom(ctx, aID); err != nil {
		t.Fatalf("ListEdgesFrom: %v", err)
	} else if len(edges) != 0 {
		t.Errorf("edges = %d at minShared=2, want 0", len(edges))
	}

	// Same data, weaker gate: now the pair must link — proving the earlier
	// zero was the minShared threshold and not an empty candidate scan.
	created, err = s.LinkSimilarClasses(ctx, aID, 1, 5)
	if err != nil {
		t.Fatalf("LinkSimilarClasses(minShared=1): %v", err)
	}
	if created != 2 {
		t.Errorf("created = %d at minShared=1, want 2", created)
	}
	if n := countEdges(t, s, ctx); n != 2 {
		t.Errorf("problem_edges rows = %d, want 2", n)
	}
}

// TestLinkSimilarClassesRejectsZeroID pins the documented input contract.
func TestLinkSimilarClassesRejectsZeroID(t *testing.T) {
	s := newLinkerTestStore(t)
	if _, err := s.LinkSimilarClasses(context.Background(), 0, 2, 5); err == nil {
		t.Fatal("LinkSimilarClasses(0) returned nil error, want an error")
	}
}

// TestTitleTokens covers the lexeme rules the linker depends on: lowercase,
// split on any non-alphanumeric run, drop tokens shorter than three
// characters and the documented stopwords, and dedupe.
func TestTitleTokens(t *testing.T) {
	tests := []struct {
		title string
		want  []string
	}{
		{"python-sdk-idle-audit", []string{"python", "sdk", "idle", "audit"}},
		{"Go-Nil_Pointer.Deref", []string{"nil", "pointer", "deref"}},
		{"  so-nil--pointer  ", []string{"nil", "pointer"}},
		{"audit audit idle", []string{"audit", "idle"}},
		{"fix-the-bug", []string{}},
		{"error-nil-idle-audit", []string{"error", "nil", "idle", "audit"}},
		{"", []string{}},
	}
	for _, tc := range tests {
		got := TitleTokens(tc.title)
		if len(got) != len(tc.want) {
			t.Errorf("TitleTokens(%q) = %v, want %v", tc.title, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("TitleTokens(%q) = %v, want %v", tc.title, got, tc.want)
				break
			}
		}
	}
}
