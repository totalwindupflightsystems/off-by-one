package graph

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
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

// ---------------------------------------------------------------------------
// OB-GAP-073 — the corpus-linking optimization.
//
// The tests below pin the two things the optimization may not change (the
// selected neighbours, their weights, both directions and the idempotent
// re-run) and the two things it exists to remove (one title tokenization per
// class per pass instead of one per class pair; one write transaction per
// batch instead of one per edge).
//
// The oracle is the PRE-optimization algorithm, copied here from
// internal/graph/linker.go at commit bf25f1a: it scans the whole class table
// per class, tokenizes every candidate title inside that scan, ranks by
// (weight desc, shared desc, title asc), cuts to maxNeighbors and expands
// both directions. It is deliberately slower and dumber than the production
// path — that is the point: a parity test is only worth something if the two
// implementations were written independently.
// ---------------------------------------------------------------------------

// edgeKey is a directed edge as (source, target). Arrays are comparable, so
// they work as map keys and a reverse lookup is just a swapped key.
type edgeKey [2]int64

func (k edgeKey) String() string { return fmt.Sprintf("%d->%d", k[0], k[1]) }

// edgeSet is a set of directed edges with their weights.
type edgeSet map[edgeKey]float64

// linkerParityCorpus builds the corpus the parity tests rank against. It is
// shaped to exercise every branch of the ranking rule:
//
//   - five classes sharing three lexemes ({python, idle, audit}) with equal
//     weights, so the title tie-break decides which of them survive;
//   - more qualifying neighbours than maxNeighbors=3 for those classes, so
//     the cut-off always drops candidates;
//   - a weaker overlap ({python, idle} only) that must rank below them;
//   - two shorter titles ({python, idle, audit} and {python, idle, drift}),
//     i.e. candidates whose token set is SMALLER than the source's — the case
//     where overlap-over-the-smaller-set differs from overlap-over-the-source
//     (weight 3/3=1.0 and 2/3 respectively, not 3/4 and 2/4);
//   - two classes sharing a single lexeme ({nil}), below minShared=2, which
//     must not link at all;
//   - two classes whose titles hold no significant lexeme ("fix-the-bug" is
//     all stopwords, "go-js-py" is all short fragments) — the empty-token
//     path, which must produce no candidates and no error.
func linkerParityCorpus(t *testing.T, s *Store, ctx context.Context) []int64 {
	t.Helper()
	titles := []string{
		"python-idle-audit-alpha",
		"python-idle-audit-beta",
		"python-idle-audit-gamma",
		"python-idle-audit-delta",
		"python-idle-audit-epsilon",
		"python-idle-drift-zeta",
		"python-sdk-idle-audit",
		"python-idle-audit",
		"python-idle-drift",
		"rust-ownership-lifetimes",
		"go-nil-pointer-deref",
		"go-nil-interface-grace",
		"fix-the-bug",
		"go-js-py",
	}
	ids := make([]int64, 0, len(titles))
	for _, title := range titles {
		ids = append(ids, mustCreateClass(t, s, ctx, title, "parity corpus: "+title))
	}
	return ids
}

// referenceCandidates is the pre-optimization candidate scan (verbatim
// algorithm, see the section comment): one table scan per class with every
// candidate title tokenized inside it.
func referenceCandidates(t *testing.T, s *Store, ctx context.Context, classID int64, srcTokens map[string]bool, minShared int) []similarCandidate {
	t.Helper()
	rows, err := s.DB().QueryContext(ctx,
		`SELECT id, title FROM problem_classes WHERE id != ?`, classID)
	if err != nil {
		t.Fatalf("reference candidate scan: %v", err)
	}
	defer rows.Close()

	var out []similarCandidate
	for rows.Next() {
		var id int64
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			t.Fatalf("reference candidate scan: %v", err)
		}
		candTokens := tokenSet(TitleTokens(title))
		shared := overlapCount(srcTokens, candTokens)
		if shared < minShared {
			continue
		}
		smaller := len(srcTokens)
		if len(candTokens) < smaller {
			smaller = len(candTokens)
		}
		if smaller == 0 {
			continue
		}
		out = append(out, similarCandidate{
			id:     id,
			title:  title,
			shared: shared,
			weight: clampWeight(float64(shared) / float64(smaller)),
		})
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reference candidate scan: %v", err)
	}
	return out
}

// referenceEdgesForClass returns the edges the pre-optimization linker wrote
// for one class. The ranking, the maxNeighbors cut-off and the
// two-direction expansion are the pre-fix code, so the oracle covers
// selection and truncation and not just the candidate scan.
func referenceEdgesForClass(t *testing.T, s *Store, ctx context.Context, classID int64, minShared, maxNeighbors int) edgeSet {
	t.Helper()
	src, err := s.GetProblemClass(ctx, classID)
	if err != nil {
		t.Fatalf("reference: GetProblemClass(%d): %v", classID, err)
	}
	srcTokens := tokenSet(TitleTokens(src.Title))
	if len(srcTokens) == 0 {
		return edgeSet{}
	}
	candidates := referenceCandidates(t, s, ctx, classID, srcTokens, minShared)

	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.weight != b.weight {
			return a.weight > b.weight
		}
		if a.shared != b.shared {
			return a.shared > b.shared
		}
		return a.title < b.title
	})
	if len(candidates) > maxNeighbors {
		candidates = candidates[:maxNeighbors]
	}

	out := make(edgeSet, 2*len(candidates))
	for _, c := range candidates {
		out[edgeKey{classID, c.id}] = c.weight
		out[edgeKey{c.id, classID}] = c.weight
	}
	return out
}

// referenceLinkAll is the pre-optimization whole-corpus pass: every class in
// ascending id order, exactly as it used to run.
func referenceLinkAll(t *testing.T, s *Store, ctx context.Context, minShared, maxNeighbors int) edgeSet {
	t.Helper()
	rows, err := s.DB().QueryContext(ctx, `SELECT id FROM problem_classes ORDER BY id`)
	if err != nil {
		t.Fatalf("reference: list ids: %v", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("reference: scan id: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reference: list ids: %v", err)
	}
	rows.Close()

	all := make(edgeSet)
	for _, id := range ids {
		for key, w := range referenceEdgesForClass(t, s, ctx, id, minShared, maxNeighbors) {
			all[key] = w
		}
	}
	return all
}

// dbEdgeSet reads the whole problem_edges table as a set of directed edges.
// Every row must carry the `similar` relationship (these stores are only ever
// written by the linker), and a repeated (source, target) row is reported as
// a failure — the (source, target, relationship) index makes it impossible,
// and the idempotency contract depends on it.
func dbEdgeSet(t *testing.T, s *Store, ctx context.Context) edgeSet {
	t.Helper()
	rows, err := s.DB().QueryContext(ctx,
		`SELECT source_id, target_id, relationship, weight FROM problem_edges`)
	if err != nil {
		t.Fatalf("select problem_edges: %v", err)
	}
	defer rows.Close()

	out := make(edgeSet)
	for rows.Next() {
		var src, dst int64
		var relationship string
		var w float64
		if err := rows.Scan(&src, &dst, &relationship, &w); err != nil {
			t.Fatalf("scan problem_edges: %v", err)
		}
		if relationship != EdgeSimilar {
			t.Errorf("edge %d->%d relationship = %q, want %q", src, dst, relationship, EdgeSimilar)
		}
		key := edgeKey{src, dst}
		if _, dup := out[key]; dup {
			t.Errorf("problem_edges holds more than one row for %s", key)
		}
		out[key] = w
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read problem_edges: %v", err)
	}
	return out
}

// deltaEdgeSet returns the entries of after that before did not hold.
func deltaEdgeSet(before, after edgeSet) edgeSet {
	delta := make(edgeSet)
	for key, weight := range after {
		if _, existed := before[key]; !existed {
			delta[key] = weight
		}
	}
	return delta
}

// diffEdgeSets returns the differences between two edge sets (sorted, so the
// failure message is readable). Empty means the sets are identical.
func diffEdgeSets(got, want edgeSet) []string {
	var problems []string
	for key, wantWeight := range want {
		gotWeight, ok := got[key]
		if !ok {
			problems = append(problems, "missing edge "+key.String())
			continue
		}
		if math.Abs(gotWeight-wantWeight) > 1e-12 {
			problems = append(problems, fmt.Sprintf("edge %s weight = %v, want %v", key, gotWeight, wantWeight))
		}
	}
	for key := range got {
		if _, ok := want[key]; !ok {
			problems = append(problems, "unexpected edge "+key.String())
		}
	}
	sort.Strings(problems)
	return problems
}

// TestLinkAllSimilarMatchesPreOptimizationRanking is the semantic-parity
// gate for the optimized whole-corpus pass: the edge set it writes must be
// EXACTLY the edge set the pre-optimization algorithm writes — same selected
// neighbours (so the same tie-breaks and maxNeighbors cut-offs), same
// weights, both directions — and a second pass must create nothing and change
// nothing.
func TestLinkAllSimilarMatchesPreOptimizationRanking(t *testing.T) {
	ctx := context.Background()
	s := newLinkerTestStore(t)
	linkerParityCorpus(t, s, ctx)

	const minShared, maxNeighbors = 2, 3
	want := referenceLinkAll(t, s, ctx, minShared, maxNeighbors)
	if len(want) == 0 {
		t.Fatal("the parity oracle produced no edges — the corpus or the oracle is broken, so this test would pass vacuously")
	}

	created, err := s.LinkAllSimilar(ctx, minShared, maxNeighbors)
	if err != nil {
		t.Fatalf("LinkAllSimilar: %v", err)
	}
	got := dbEdgeSet(t, s, ctx)
	if problems := diffEdgeSets(got, want); len(problems) > 0 {
		t.Fatalf("optimized pass differs from the pre-optimization algorithm on %d edge(s):\n  %s",
			len(problems), strings.Join(problems, "\n  "))
	}
	// created counts successful INSERTs. Every edge of the oracle is new on
	// a fresh store and the unique index rejects a second attempt, so the
	// count must equal the distinct edge count — no edge written twice, none
	// silently skipped.
	if created != len(want) {
		t.Errorf("LinkAllSimilar created = %d, want %d (the number of distinct edges)", created, len(want))
	}

	// Every edge must have its mirror image with the same weight: the reads
	// in this codebase are single-directional, so a one-sided write would
	// make /problems/<dst>/related look empty.
	for key, weight := range got {
		reverse := edgeKey{key[1], key[0]}
		reverseWeight, ok := got[reverse]
		if !ok {
			t.Errorf("%s has no reverse edge %s", key, reverse)
			continue
		}
		if math.Abs(reverseWeight-weight) > 1e-12 {
			t.Errorf("reverse edge %s weight = %v, want %v (same as %s)", reverse, reverseWeight, weight, key)
		}
	}

	// Idempotency: a re-run creates nothing, leaves the edge set identical
	// and writes no duplicate rows.
	again, err := s.LinkAllSimilar(ctx, minShared, maxNeighbors)
	if err != nil {
		t.Fatalf("second LinkAllSimilar: %v", err)
	}
	if again != 0 {
		t.Errorf("second pass created = %d, want 0", again)
	}
	if problems := diffEdgeSets(dbEdgeSet(t, s, ctx), want); len(problems) > 0 {
		t.Fatalf("second pass changed the edge set:\n  %s", strings.Join(problems, "\n  "))
	}
	var duplicates int
	if err := s.DB().QueryRowContext(ctx, `
		SELECT COUNT(*) FROM (
			SELECT 1 FROM problem_edges
			GROUP BY source_id, target_id, relationship
			HAVING COUNT(*) > 1
		)`).Scan(&duplicates); err != nil {
		t.Fatalf("count duplicate edges: %v", err)
	}
	if duplicates != 0 {
		t.Errorf("problem_edges holds %d duplicated (source, target, relationship) group(s), want 0", duplicates)
	}
}

// TestLinkSimilarClassesMatchesPreOptimizationRanking is the parity gate for
// the single-class entry point the import path uses. It walks the corpus one
// class at a time and asserts the EXACT edges each call writes — the delta
// between the table before and after the call, which is the only way to see
// one class's own selection (a class also receives reverse edges from earlier
// classes, so an accumulated "edges from this class" view would mask a wrong
// selection). The whole table is then compared against the whole-corpus
// oracle.
func TestLinkSimilarClassesMatchesPreOptimizationRanking(t *testing.T) {
	ctx := context.Background()
	s := newLinkerTestStore(t)
	ids := linkerParityCorpus(t, s, ctx)

	const minShared, maxNeighbors = 2, 3

	// The whole-corpus oracle, computed once before any edge is written.
	allWant := referenceLinkAll(t, s, ctx, minShared, maxNeighbors)
	if len(allWant) == 0 {
		t.Fatal("the parity oracle produced no edges — the corpus or the oracle is broken, so this test would pass vacuously")
	}

	createdTotal := 0
	for _, id := range ids {
		// The oracle's edges for this class: both directions of its own
		// selection, including for the two classes whose titles hold no
		// lexeme (they select nothing and must write nothing).
		want := referenceEdgesForClass(t, s, ctx, id, minShared, maxNeighbors)
		before := dbEdgeSet(t, s, ctx)

		created, err := s.LinkSimilarClasses(ctx, id, minShared, maxNeighbors)
		if err != nil {
			t.Fatalf("LinkSimilarClasses(%d): %v", id, err)
		}
		createdTotal += created
		after := dbEdgeSet(t, s, ctx)

		// The call must write exactly the oracle edges that were not already
		// there: no wrong neighbour, no missed neighbour, no second row for
		// an edge a previous call already wrote.
		expected := make(edgeSet)
		for key, weight := range want {
			if _, existed := before[key]; !existed {
				expected[key] = weight
			}
		}
		got := deltaEdgeSet(before, after)
		if problems := diffEdgeSets(got, expected); len(problems) > 0 {
			t.Fatalf("LinkSimilarClasses(%d) wrote a different edge set than the pre-optimization algorithm on %d edge(s):\n  %s",
				id, len(problems), strings.Join(problems, "\n  "))
		}
		if created != len(expected) {
			t.Errorf("LinkSimilarClasses(%d) created = %d, want %d", id, created, len(expected))
		}
	}
	if createdTotal == 0 {
		t.Fatal("LinkSimilarClasses created no edges at all — the corpus or the oracle is broken, so this test would pass vacuously")
	}

	if problems := diffEdgeSets(dbEdgeSet(t, s, ctx), allWant); len(problems) > 0 {
		t.Fatalf("per-class linking differs from the pre-optimization algorithm on %d edge(s):\n  %s",
			len(problems), strings.Join(problems, "\n  "))
	}

	// Re-running every class must create nothing (idempotent per class too).
	for _, id := range ids {
		created, err := s.LinkSimilarClasses(ctx, id, minShared, maxNeighbors)
		if err != nil {
			t.Fatalf("second LinkSimilarClasses(%d): %v", id, err)
		}
		if created != 0 {
			t.Errorf("second LinkSimilarClasses(%d) created = %d, want 0", id, created)
		}
	}
}

// TestLinkAllSimilarTokenizesEachTitleOnce is the instrumentation gate for
// the hot-path waste (OB-GAP-073): a full pass must tokenize each title ONCE
// (into the pass's title index), not once per other class. The pre-fix pass
// tokenized every title inside a per-class scan of the whole table, i.e.
// len(classes)² times; this test fails loudly if that shape ever comes back.
//
// Wall-clock assertions are deliberately avoided — they are load-dependent
// and flaky. The invocation count is exact.
func TestLinkAllSimilarTokenizesEachTitleOnce(t *testing.T) {
	ctx := context.Background()
	s := newLinkerTestStore(t)

	// All titles share the same lexeme set ({python, idle, audit}; the
	// two-digit suffixes are dropped as too short), so every class has a full
	// candidate list and the pass has real work to do.
	const classes = 12
	for i := 0; i < classes; i++ {
		mustCreateClass(t, s, ctx, fmt.Sprintf("python-idle-audit-%02d", i), "tokenization count corpus")
	}

	calls := 0
	titleTokenizer = func(title string) []string {
		calls++
		return TitleTokens(title)
	}
	defer func() { titleTokenizer = TitleTokens }()

	created, err := s.LinkAllSimilar(ctx, 2, 5)
	if err != nil {
		t.Fatalf("LinkAllSimilar: %v", err)
	}
	if created == 0 {
		t.Fatal("the pass created no edges — it would pass this test vacuously")
	}
	if calls != classes {
		t.Errorf("one pass tokenized %d titles for %d classes, want %d (each title once per pass, not once per class pair)",
			calls, classes, classes)
	}

	// A second (idempotent) pass re-tokenizes once per class again — the
	// index is per pass, not a cross-run cache, so a corpus sync is always
	// scored against the current titles.
	calls = 0
	if _, err := s.LinkAllSimilar(ctx, 2, 5); err != nil {
		t.Fatalf("second LinkAllSimilar: %v", err)
	}
	if calls != classes {
		t.Errorf("second pass tokenized %d titles, want %d", calls, classes)
	}
}

// TestWriteSimilarEdgesBatchesAtomically pins the write-batch boundary the
// optimization introduced: the edges of one batch are committed together, so
// a failure part-way through leaves the table exactly as it was instead of
// the partial set the per-edge autocommit path used to leave behind.
func TestWriteSimilarEdgesBatchesAtomically(t *testing.T) {
	ctx := context.Background()
	s := newLinkerTestStore(t)
	aID := mustCreateClass(t, s, ctx, "python-idle-audit-alpha", "batch corpus")
	bID := mustCreateClass(t, s, ctx, "python-idle-audit-beta", "batch corpus")

	// Premise: this store really enforces foreign keys, so the bogus target
	// below is a genuine mid-batch failure rather than a stray row. If the
	// driver ever stopped applying the DSN pragma, the test must fail here
	// instead of passing for the wrong reason.
	if _, err := insertEdge(ctx, s.DB(), aID, 999999, EdgeSimilar, 0.5); err == nil {
		t.Fatal("premise failed: an edge to a non-existent class was accepted, so foreign keys are not enforced")
	}
	if n := countEdges(t, s, ctx); n != 0 {
		t.Fatalf("problem_edges rows = %d after the failed premise insert, want 0", n)
	}

	edges := []similarEdge{
		{source: aID, target: bID, weight: 0.75},
		{source: bID, target: aID, weight: 0.75},
		{source: aID, target: 999999, weight: 0.5}, // fails: no such class
	}
	created, err := s.writeSimilarEdges(ctx, edges)
	if err == nil {
		t.Fatal("writeSimilarEdges with an impossible edge returned nil error")
	}
	if created != 0 {
		t.Errorf("created = %d on a rolled-back batch, want 0", created)
	}
	if n := countEdges(t, s, ctx); n != 0 {
		t.Errorf("problem_edges rows = %d after a failed batch, want 0 — the batch must roll back as a unit", n)
	}

	// Control: the same batch without the impossible edge lands both
	// directions in one commit.
	created, err = s.writeSimilarEdges(ctx, edges[:2])
	if err != nil {
		t.Fatalf("writeSimilarEdges(control): %v", err)
	}
	if created != 2 {
		t.Errorf("control created = %d, want 2", created)
	}
	if n := countEdges(t, s, ctx); n != 2 {
		t.Errorf("problem_edges rows = %d after the control batch, want 2", n)
	}

	// And a re-run of the same batch creates nothing.
	created, err = s.writeSimilarEdges(ctx, edges[:2])
	if err != nil {
		t.Fatalf("writeSimilarEdges(re-run): %v", err)
	}
	if created != 0 {
		t.Errorf("re-run created = %d, want 0 (duplicate edges are skipped, not re-inserted)", created)
	}
}
